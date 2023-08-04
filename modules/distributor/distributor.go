/*
 * Copyright (C) 2023  Intergral GmbH
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package distributor

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/intergral/deep/modules/distributor/snapshotreceiver"
	"github.com/intergral/deep/modules/tracepoint/client"
	deeppb_poll "github.com/intergral/deep/pkg/deeppb/poll/v1"
	"github.com/intergral/deep/pkg/util"
	pb "github.com/intergral/go-deep-proto/poll/v1"
	tp "github.com/intergral/go-deep-proto/tracepoint/v1"
	"google.golang.org/grpc/status"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/limiter"
	"github.com/grafana/dskit/ring"
	ring_client "github.com/grafana/dskit/ring/client"
	"github.com/grafana/dskit/services"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/prometheus/util/strutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/intergral/deep/modules/distributor/forwarder"
	generator_client "github.com/intergral/deep/modules/generator/client"
	ingester_client "github.com/intergral/deep/modules/ingester/client"
	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/pkg/deeppb"
	deeppb_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/model"
	deep_util "github.com/intergral/deep/pkg/util"
)

const (
	// reasonRateLimited indicates that the tenants snapshots/second exceeded their limits
	reasonRateLimited = "rate_limited"
	// reasonSnapshotTooLarge indicates that a single snapshots is too large
	reasonSnapshotTooLarge = "snapshot_too_large"
	// reasonLiveSnapshotsExceeded indicates that deep is already tracking too many live snapshots in the ingesters for this user
	reasonLiveSnapshotsExceeded = "live_snapshots_exceeded"
	// reasonInternalError indicates an unexpected error occurred processing this snapshot, analogous to a 500
	reasonInternalError = "internal_error"

	distributorRingKey = "distributor"

	discardReasonLabel = "reason"
)

var (
	metricDiscardedSnapshots = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "discarded_snapshots_total",
		Help:      "The total number of samples that were discarded.",
	}, []string{discardReasonLabel, "tenant"})
	metricSnapshotBytesIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "snapshot_bytes_received_total",
		Help:      "The total number of snapshot proto bytes received per tenant",
	}, []string{"tenant"})
	metricPollBytesIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "poll_bytes_received_total",
		Help:      "The total number of poll proto bytes received per tenant",
	}, []string{"tenant"})
	metricPollBytesResponded = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "poll_bytes_sent_total",
		Help:      "The total number of poll proto bytes sent in response per tenant",
	}, []string{"tenant"})

	metricIngesterAppends = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "ingester_appends_total",
		Help:      "The total number of batch appends sent to ingesters.",
	}, []string{"ingester"})
	metricIngesterAppendFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "ingester_append_failures_total",
		Help:      "The total number of failed batch appends sent to ingesters.",
	}, []string{"ingester"})

	metricGeneratorPushes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "metrics_generator_pushes_total",
		Help:      "The total number of span pushes sent to metrics-generators.",
	}, []string{"metrics_generator"})
	metricGeneratorPushesFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "metrics_generator_pushes_failures_total",
		Help:      "The total number of failed span pushes sent to metrics-generators.",
	}, []string{"metrics_generator"})
	metricIngesterClients = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "ingester_clients",
		Help:      "The current number of ingester clients.",
	})
	metricGeneratorClients = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "metrics_generator_clients",
		Help:      "The current number of metrics-generator clients.",
	})
)

// Distributor coordinates replicates and distribution of log streams.
type Distributor struct {
	services.Service

	cfg             Config
	clientCfg       ingester_client.Config
	ingestersRing   ring.ReadRing
	pool            *ring_client.Pool
	DistributorRing *ring.Ring
	overrides       *overrides.Overrides
	snapshotEncoder model.SegmentDecoder

	// metrics-generator
	generatorClientCfg generator_client.Config
	generatorsRing     ring.ReadRing
	generatorsPool     *ring_client.Pool
	generatorForwarder *generatorForwarder

	// Generic Forwarder
	forwardersManager *forwarder.Manager

	// Per-tenant rate limiter.
	ingestionRateLimiter *limiter.RateLimiter

	// Manager for subservices
	subservices        *services.Manager
	subservicesWatcher *services.FailureWatcher

	logger log.Logger

	SnapshotReceiver tp.SnapshotServiceServer
	tpClient         *client.TPClient
}

// New crates a new distributor services
func New(cfg Config, tpClient *client.TPClient, clientCfg ingester_client.Config, middleware snapshotreceiver.Middleware, ingestersRing ring.ReadRing, generatorClientCfg generator_client.Config, generatorsRing ring.ReadRing, o *overrides.Overrides, logger log.Logger, reg prometheus.Registerer) (*Distributor, error) {
	factory := cfg.factory
	if factory == nil {
		factory = func(addr string) (ring_client.PoolClient, error) {
			return ingester_client.New(addr, clientCfg)
		}
	}

	subServices := []services.Service(nil)

	// Create the configured ingestion rate limit strategy (local or global).
	var ingestionRateStrategy limiter.RateLimiterStrategy
	var distributorRing *ring.Ring

	// using global ingestion rate means we monitor the number of distributor instances with a ring and divide
	// the ingest rate between the instances
	if o.IngestionRateStrategy() == overrides.GlobalIngestionRateStrategy {
		// create ring to monitor number of distributor instances
		lifecyclerCfg := cfg.DistributorRing.ToLifecyclerConfig()
		lifecycler, err := ring.NewLifecycler(lifecyclerCfg, nil, "distributor", cfg.OverrideRingKey, false, logger, prometheus.WrapRegistererWithPrefix("deep_", reg))
		if err != nil {
			return nil, err
		}
		subServices = append(subServices, lifecycler)
		ingestionRateStrategy = newGlobalIngestionRateStrategy(o, lifecycler)

		newRing, err := ring.New(lifecyclerCfg.RingConfig, "distributor", cfg.OverrideRingKey, logger, prometheus.WrapRegistererWithPrefix("deep_", reg))
		if err != nil {
			return nil, errors.Wrap(err, "unable to initialize distributor ring")
		}
		distributorRing = newRing
		subServices = append(subServices, distributorRing)
	} else {
		ingestionRateStrategy = newLocalIngestionRateStrategy(o)
	}

	// this pool lets the distributor connect to the ingesters via the Ingester Client
	pool := ring_client.NewPool("distributor_pool",
		clientCfg.PoolConfig,
		ring_client.NewRingServiceDiscovery(ingestersRing),
		factory,
		metricIngesterClients,
		logger)

	subServices = append(subServices, pool)

	d := &Distributor{
		cfg:                  cfg,
		clientCfg:            clientCfg,
		ingestersRing:        ingestersRing,
		pool:                 pool,
		DistributorRing:      distributorRing,
		ingestionRateLimiter: limiter.NewRateLimiter(ingestionRateStrategy, 10*time.Second),
		generatorClientCfg:   generatorClientCfg,
		generatorsRing:       generatorsRing,
		overrides:            o,
		snapshotEncoder:      model.MustNewSegmentDecoder(model.CurrentEncoding),
		logger:               logger,
		tpClient:             tpClient,
	}

	// this pool lets the distributor generate metrics via the generator client
	d.generatorsPool = ring_client.NewPool(
		"distributor_metrics_generator_pool",
		generatorClientCfg.PoolConfig,
		ring_client.NewRingServiceDiscovery(generatorsRing),
		func(addr string) (ring_client.PoolClient, error) {
			return generator_client.New(addr, generatorClientCfg)
		},
		metricGeneratorClients,
		logger,
	)

	subServices = append(subServices, d.generatorsPool)
	// create forwarder for metrics generator
	d.generatorForwarder = newGeneratorForwarder(logger, d.sendToGenerators, o)
	subServices = append(subServices, d.generatorForwarder)
	// create dynamic (from config) forwarders that will get sent the snapshots we receive
	forwardersManager, err := forwarder.NewManager(d.cfg.Forwarders, logger, o)
	if err != nil {
		return nil, fmt.Errorf("failed to create forwarders manager: %w", err)
	}

	d.forwardersManager = forwardersManager
	subServices = append(subServices, d.forwardersManager)

	// setup receivers
	cfgReceivers := cfg.Receivers
	if len(cfgReceivers) == 0 {
		cfgReceivers = defaultReceivers
	}

	// create snapshot receiver
	receiverServices, snapshotReceiver, err := snapshotreceiver.New(cfgReceivers, d, middleware, logger)
	d.SnapshotReceiver = snapshotReceiver
	if err != nil {
		return nil, err
	}
	subServices = append(subServices, receiverServices)

	// create new service for distributor and related services
	d.subservices, err = services.NewManager(subServices...)
	if err != nil {
		return nil, fmt.Errorf("failed to create subservices %w", err)
	}
	d.subservicesWatcher = services.NewFailureWatcher()
	d.subservicesWatcher.WatchManager(d.subservices)

	d.Service = services.NewBasicService(d.starting, d.running, d.stopping)
	return d, nil
}

func (d *Distributor) starting(ctx context.Context) error {
	level.Info(d.logger).Log("msg", "Distributor started")
	// Only report success if all sub-services start properly
	err := services.StartManagerAndAwaitHealthy(ctx, d.subservices)
	if err != nil {
		return fmt.Errorf("failed to start subservices %w", err)
	}

	return nil
}

func (d *Distributor) running(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	case err := <-d.subservicesWatcher.Chan():
		return fmt.Errorf("distributor subservices failed %w", err)
	}
}

// Called after distributor is asked to stop via StopAsync.
func (d *Distributor) stopping(_ error) error {
	return services.StopManagerAndAwaitStopped(context.Background(), d.subservices)
}

func (d *Distributor) PushPoll(ctx context.Context, pollRequest *pb.PollRequest) (*pb.PollResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "distributor.PushPoll")
	defer span.Finish()

	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		// can't record as we have no tenant
		return nil, err
	}

	// todo is this really needed?
	// Convert to bytes and back. This is unfortunate for efficiency, but it works
	// around to allow deep agent to be installed in deep service
	convert, err := proto.Marshal(pollRequest)
	if err != nil {
		return nil, err
	}

	// deeppb_tp.Snapshot is wire-compatible with go-deep-proto
	req := &deeppb_poll.PollRequest{}
	err = proto.Unmarshal(convert, req)
	if err != nil {
		return nil, err
	}

	request := &deeppb.LoadTracepointRequest{Request: req}

	size := proto.Size(pollRequest)
	metricPollBytesIngested.WithLabelValues(tenantID).Add(float64(size))

	tracepoints, err := d.tpClient.LoadTracepoints(ctx, request)
	if err != nil {
		return nil, err
	}

	response := tracepoints.GetResponse()

	responseSize := proto.Size(response)
	metricPollBytesResponded.WithLabelValues(tenantID).Add(float64(responseSize))

	marshal, err := proto.Marshal(response)
	if err != nil {
		return nil, err
	}

	resp := &pb.PollResponse{}
	err = proto.Unmarshal(marshal, resp)

	return resp, nil
}

func (d *Distributor) PushSnapshot(ctx context.Context, in *tp.Snapshot) (*tp.SnapshotResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "distributor.PushSnapshot")
	defer span.Finish()

	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		// can't record discarded spans here b/c there's no tenant
		return nil, err
	}

	// todo is this really needed?
	// Convert to bytes and back. This is unfortunate for efficiency, but it works
	// around to allow deep agent to be installed in deep service
	convert, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}

	// deeppb_tp.Snapshot is wire-compatible with go-deep-proto
	snapshot := &deeppb_tp.Snapshot{}
	err = proto.Unmarshal(convert, snapshot)
	if err != nil {
		return nil, err
	}

	if d.cfg.LogReceivedSnapshots.Enabled {
		if d.cfg.LogReceivedSnapshots.IncludeAllAttributes {
			logSnapshotWithAllAttributes(snapshot, d.logger)
		} else {
			logSnapshot(snapshot, d.logger)
		}
	}

	size := proto.Size(snapshot)
	metricSnapshotBytesIngested.WithLabelValues(tenantID).Add(float64(size))

	// check limits
	now := time.Now()
	if !d.ingestionRateLimiter.AllowN(now, tenantID, size) {
		metricDiscardedSnapshots.WithLabelValues(reasonRateLimited, tenantID).Add(1)
		return nil, status.Errorf(codes.ResourceExhausted,
			"%s ingestion rate limit (%d bytes) exceeded while adding %d bytes",
			overrides.ErrorPrefixRateLimited,
			int(d.ingestionRateLimiter.Limit(now, tenantID)),
			size)
	}

	keys, err := extractKeys(snapshot, tenantID)
	if err != nil {
		metricDiscardedSnapshots.WithLabelValues(reasonInternalError, tenantID).Add(1)
		return nil, err
	}

	err = d.sendToIngester(ctx, tenantID, keys, snapshot)
	if err != nil {
		recordDiscardedSnapshots(err, tenantID)
		return nil, err
	}

	if len(d.overrides.MetricsGeneratorProcessors(tenantID)) > 0 {
		d.generatorForwarder.SendSnapshot(ctx, tenantID, keys, snapshot)
	}

	if err := d.forwardersManager.ForTenant(tenantID).ForwardSnapshot(ctx, snapshot); err != nil {
		_ = level.Warn(d.logger).Log("msg", "failed to forward batches for tenant=%s: %w", tenantID, err)
	}

	return &tp.SnapshotResponse{}, nil
}

func extractKeys(snapshot *deeppb_tp.Snapshot, userId string) ([]uint32, error) {
	keys := make([]uint32, 1)

	keys[0] = deep_util.TokenFor(userId, snapshot.GetID())

	return keys, nil
}

func logSnapshot(snapshot *deeppb_tp.Snapshot, logger log.Logger) {
	level.Info(logger).Log("msg", "received", "snapshotId", util.SnapshotIDToHexString(snapshot.GetID()))
}

func logSnapshotWithAllAttributes(snapshot *deeppb_tp.Snapshot, logger log.Logger) {
	for _, a := range snapshot.GetResource() {
		logger = log.With(
			logger,
			"span_"+strutil.SanitizeLabelName(a.GetKey()),
			deep_util.StringifyAnyValue(a.GetValue()))
	}

	for _, a := range snapshot.GetAttributes() {
		logger = log.With(
			logger,
			"span_"+strutil.SanitizeLabelName(a.GetKey()),
			deep_util.StringifyAnyValue(a.GetValue()))
	}

	latencySeconds := float64(snapshot.GetDurationNanos()) / float64(time.Second.Nanoseconds())

	logger = log.With(
		logger,
		"path", snapshot.GetTracepoint().GetPath(),
		"line_no", snapshot.GetTracepoint().GetLineNumber(),
		"span_duration_seconds", latencySeconds,
	)

	logSnapshot(snapshot, logger)
}

func (d *Distributor) sendToGenerators(ctx context.Context, userID string, keys []uint32, snapshot *deeppb_tp.Snapshot) error {
	// If an instance is unhealthy write to the next one (i.e. write extend is enabled)
	op := ring.Write

	readRing := d.generatorsRing.ShuffleShard(userID, d.overrides.MetricsGeneratorRingSize(userID))

	err := ring.DoBatch(ctx, op, readRing, keys, func(generator ring.InstanceDesc, indexes []int) error {
		localCtx, cancel := context.WithTimeout(ctx, d.generatorClientCfg.RemoteTimeout)
		defer cancel()
		localCtx = util.InjectTenantID(localCtx, userID)

		req := deeppb.PushSnapshotRequest{
			Snapshot: snapshot,
		}

		c, err := d.generatorsPool.GetClientFor(generator.Addr)
		if err != nil {
			return errors.Wrap(err, "failed to get client for generator")
		}

		_, err = c.(deeppb.MetricsGeneratorClient).PushSnapshot(localCtx, &req)
		metricGeneratorPushes.WithLabelValues(generator.Addr).Inc()
		if err != nil {
			metricGeneratorPushesFailures.WithLabelValues(generator.Addr).Inc()
		}
		return errors.Wrap(err, "failed to push spans to generator")
	}, func() {})

	return err
}

// Check implements the grpc healthcheck
func (*Distributor) Check(_ context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func (d *Distributor) sendToIngester(ctx context.Context, userID string, keys []uint32, snapshot *deeppb_tp.Snapshot) error {
	// Marshal to bytes once
	bytes, err := d.snapshotEncoder.PrepareForWrite(snapshot, uint32(snapshot.TsNanos/1e9))
	if err != nil {
		return errors.Wrap(err, "failed to marshal PushRequest")
	}

	op := ring.WriteNoExtend
	if d.cfg.ExtendWrites {
		op = ring.Write
	}

	err = ring.DoBatch(ctx, op, d.ingestersRing, keys, func(ingester ring.InstanceDesc, indexes []int) error {
		localCtx, cancel := context.WithTimeout(ctx, d.clientCfg.RemoteTimeout)
		defer cancel()
		localCtx = util.InjectTenantID(localCtx, userID)

		req := deeppb.PushBytesRequest{
			Snapshot: bytes,
			ID:       snapshot.ID,
		}

		c, err := d.pool.GetClientFor(ingester.Addr)
		if err != nil {
			return err
		}

		_, err = c.(deeppb.IngesterServiceClient).PushBytes(localCtx, &req)
		metricIngesterAppends.WithLabelValues(ingester.Addr).Inc()
		if err != nil {
			metricIngesterAppendFailures.WithLabelValues(ingester.Addr).Inc()
		}
		return err
	}, func() {})

	return err
}

func recordDiscardedSnapshots(err error, userID string) {
	s := status.Convert(err)
	if s == nil {
		return
	}
	desc := s.Message()

	var reason = reasonInternalError
	if strings.HasPrefix(desc, overrides.ErrorPrefixLiveSnapshotsExceeded) {
		reason = reasonLiveSnapshotsExceeded
	} else if strings.HasPrefix(desc, overrides.ErrorPrefixSnapshotTooLarge) {
		reason = reasonSnapshotTooLarge
	}

	metricDiscardedSnapshots.WithLabelValues(reason, userID).Add(1)
}
