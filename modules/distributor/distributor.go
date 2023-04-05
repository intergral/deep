package distributor

import (
	"context"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/intergral/deep/modules/distributor/snapshotreciever"
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
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
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
	// reasonRateLimited indicates that the tenants spans/second exceeded their limits
	reasonRateLimited = "rate_limited"
	// reasonTraceTooLarge indicates that a single trace has too many spans
	reasonTraceTooLarge = "trace_too_large"
	// reasonLiveTracesExceeded indicates that deep is already tracking too many live traces in the ingesters for this user
	reasonLiveTracesExceeded = "live_traces_exceeded"
	// reasonInternalError indicates an unexpected error occurred processing these spans. analogous to a 500
	reasonInternalError = "internal_error"

	distributorRingKey = "distributor"

	discardReasonLabel = "reason"
)

var (
	metricIngesterAppends = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_ingester_appends_total",
		Help:      "The total number of batch appends sent to ingesters.",
	}, []string{"ingester"})
	metricIngesterAppendFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_ingester_append_failures_total",
		Help:      "The total number of failed batch appends sent to ingesters.",
	}, []string{"ingester"})
	metricDiscardedSnapshots = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "discarded_snapshots_total",
		Help:      "The total number of samples that were discarded.",
	}, []string{discardReasonLabel, "tenant"})
	metricSpansIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_snapshots_received_total",
		Help:      "The total number of snapshots received per tenant",
	}, []string{"tenant"})
	metricBytesIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_bytes_received_total",
		Help:      "The total number of proto bytes received per tenant",
	}, []string{"tenant"})

	metricGeneratorPushes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_metrics_generator_pushes_total",
		Help:      "The total number of span pushes sent to metrics-generators.",
	}, []string{"metrics_generator"})
	metricGeneratorPushesFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_metrics_generator_pushes_failures_total",
		Help:      "The total number of failed span pushes sent to metrics-generators.",
	}, []string{"metrics_generator"})
	metricIngesterClients = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "deep",
		Name:      "distributor_ingester_clients",
		Help:      "The current number of ingester clients.",
	})
	metricGeneratorClients = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "deep",
		Name:      "distributor_metrics_generator_clients",
		Help:      "The current number of metrics-generator clients.",
	})
)

// Distributor coordinates replicates and distribution of log streams.
type Distributor struct {
	services.Service
	pb.UnimplementedPollConfigServer

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

	// Per-user rate limiter.
	ingestionRateLimiter *limiter.RateLimiter

	// Manager for subservices
	subservices        *services.Manager
	subservicesWatcher *services.FailureWatcher

	logger log.Logger

	SnapshotReceiver tp.SnapshotServiceServer
}

func (d *Distributor) Poll(ctx context.Context, pollRequest *pb.PollRequest) (*pb.PollResponse, error) {
	print("Called long poll")
	var responseType = pb.ResponseType_UPDATE
	if pollRequest.CurrentHash == "123" {
		responseType = pb.ResponseType_NO_CHANGE
	}

	return &pb.PollResponse{
		Ts:          time.Now().UnixMilli(),
		CurrentHash: "123",
		Response: []*tp.TracePointConfig{{
			Id: "17", Path: "/simple-app/simple_test.py", LineNo: 31,
			Args:    map[string]string{"some": "thing", "fire_count": "-1", "fire_period": "10000"},
			Watches: []string{"len(uuid)", "uuid", "self.char_counter"},
		}},
		ResponseType: responseType,
	}, nil
}

// New a distributor creates.
func New(cfg Config, clientCfg ingester_client.Config, ingestersRing ring.ReadRing, generatorClientCfg generator_client.Config, generatorsRing ring.ReadRing, o *overrides.Overrides, logger log.Logger, loggingLevel logging.Level, reg prometheus.Registerer) (*Distributor, error) {
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

	if o.IngestionRateStrategy() == overrides.GlobalIngestionRateStrategy {
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
	}

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

	d.generatorForwarder = newGeneratorForwarder(logger, d.sendToGenerators, o)
	subServices = append(subServices, d.generatorForwarder)

	forwardersManager, err := forwarder.NewManager(d.cfg.Forwarders, logger, o)
	if err != nil {
		return nil, fmt.Errorf("failed to create forwarders manager: %w", err)
	}

	d.forwardersManager = forwardersManager
	subServices = append(subServices, d.forwardersManager)

	cfgReceivers := cfg.Receivers
	if len(cfgReceivers) == 0 {
		cfgReceivers = defaultReceivers
	}

	receiverServices, snapshotReceiver, err := snapshotreciever.New(cfgReceivers, d, loggingLevel)
	d.SnapshotReceiver = snapshotReceiver
	if err != nil {
		return nil, err
	}
	subServices = append(subServices, receiverServices)

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

func (d *Distributor) PushSnapshot(ctx context.Context, in *tp.Snapshot) (*tp.SnapshotResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "distributor.PushSnapshot")
	defer span.Finish()

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

	userID, err := user.ExtractOrgID(ctx)
	if err != nil {
		// can't record discarded spans here b/c there's no tenant
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
	metricBytesIngested.WithLabelValues(userID).Add(float64(size))
	metricSpansIngested.WithLabelValues(userID).Add(float64(1))

	// check limits
	now := time.Now()
	if !d.ingestionRateLimiter.AllowN(now, userID, size) {
		metricDiscardedSnapshots.WithLabelValues(reasonRateLimited, userID).Add(1)
		return nil, status.Errorf(codes.ResourceExhausted,
			"%s ingestion rate limit (%d bytes) exceeded while adding %d bytes",
			overrides.ErrorPrefixRateLimited,
			int(d.ingestionRateLimiter.Limit(now, userID)),
			size)
	}

	keys, err := extractKeys(snapshot, userID)
	if err != nil {
		metricDiscardedSnapshots.WithLabelValues(reasonInternalError, userID).Add(1)
		return nil, err
	}

	err = d.sendToIngester(ctx, userID, keys, snapshot)
	if err != nil {
		recordDiscardedSnapshots(err, userID, 1)
		return nil, err
	}

	if len(d.overrides.MetricsGeneratorProcessors(userID)) > 0 {
		d.generatorForwarder.SendSnapshot(ctx, userID, keys, snapshot)
	}

	if err := d.forwardersManager.ForTenant(userID).ForwardSnapshot(ctx, snapshot); err != nil {
		_ = level.Warn(d.logger).Log("msg", "failed to forward batches for tenant=%s: %w", userID, err)
	}

	return &tp.SnapshotResponse{}, nil
}

func extractKeys(snapshot *deeppb_tp.Snapshot, userId string) ([]uint32, error) {
	keys := make([]uint32, 1)

	keys[0] = deep_util.TokenFor(userId, []byte(snapshot.GetId()))

	return keys, nil
}

func logSnapshot(snapshot *deeppb_tp.Snapshot, logger log.Logger) {
	level.Info(logger).Log("msg", "received", "snapshotId", snapshot.GetId())
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

	latencySeconds := float64(snapshot.GetNanosDuration()) / float64(time.Second.Nanoseconds())

	logger = log.With(
		logger,
		"path", snapshot.GetTracepoint().GetPath(),
		"line_no", snapshot.GetTracepoint().GetLineNo(),
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
		localCtx = user.InjectOrgID(localCtx, userID)

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
	bytes, err := d.snapshotEncoder.PrepareForWrite(snapshot, uint32(snapshot.Ts))
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
		localCtx = user.InjectOrgID(localCtx, userID)

		req := deeppb.PushBytesRequest{
			Snapshot: bytes,
			Id:       snapshot.Id,
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

func recordDiscardedSnapshots(err error, userID string, spanCount int) {
	s := status.Convert(err)
	if s == nil {
		return
	}
	desc := s.Message()

	var reason = reasonInternalError
	if strings.HasPrefix(desc, overrides.ErrorPrefixLiveTracesExceeded) {
		reason = reasonLiveTracesExceeded
	} else if strings.HasPrefix(desc, overrides.ErrorPrefixTraceTooLarge) {
		reason = reasonTraceTooLarge
	}

	metricDiscardedSnapshots.WithLabelValues(reason, userID).Add(1)
}
