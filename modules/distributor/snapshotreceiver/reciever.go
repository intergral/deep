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

package snapshotreceiver

import (
	"context"
	"fmt"
	gkLog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/services"
	"github.com/intergral/deep/pkg/receivers"
	"github.com/intergral/deep/pkg/receivers/types"
	"github.com/intergral/deep/pkg/util/log"
	pb "github.com/intergral/go-deep-proto/poll/v1"
	tp "github.com/intergral/go-deep-proto/tracepoint/v1"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/weaveworks/common/user"
	"go.opencensus.io/stats/view"
	"go.uber.org/multierr"
	"google.golang.org/grpc"
	"time"
)

const (
	logsPerSecond = 10
)

var (
	metricPollDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "poll_duration_seconds",
		Help:      "Records the amount of time to poll a config from tracepoint.",
		Buckets:   prometheus.DefBuckets,
	})
	metricPushDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "push_duration_seconds",
		Help:      "Records the amount of time to push a batch to the ingester.",
		Buckets:   prometheus.DefBuckets,
	})

	metricDistributorAccepted = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "snapshots_requests",
		Help:      "Number of snapshots successfully pushed into the pipeline.",
	}, []string{"tenant", "receiver"})
	metricPollAccepted = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "poll_requests",
		Help:      "Number of completed poll requests.",
	}, []string{"tenant", "receiver"})

	metricDistributorRefused = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "failed_snapshots_requests",
		Help:      "Number of snapshots that could not be pushed into the pipeline.",
	}, []string{"tenant", "receiver"})
	metricPollRefused = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "distributor",
		Name:      "failed_poll_requests",
		Help:      "Number of failed poll requests.",
	}, []string{"tenant", "receiver"})
)

type SnapshotPusher interface {
	PushSnapshot(ctx context.Context, traces *tp.Snapshot) (*tp.SnapshotResponse, error)
	PushPoll(ctx context.Context, pollRequest *pb.PollRequest) (*pb.PollResponse, error)
}

type SnapshotReceiver struct {
	tp.UnimplementedSnapshotServiceServer
}

type snapshotReceiver struct {
	services.Service
	SnapshotReceiver

	pusher      SnapshotPusher
	logger      *log.RateLimitedLogger
	metricViews []*view.View
	fatal       chan error
	receivers   []types.Receiver
	grpc        *grpc.Server
}

func (sr *snapshotReceiver) ReportFatalError(err error) {
	_ = level.Error(log.Logger).Log("msg", "fatal error reported", "err", err)
	sr.fatal <- err
}

// Poll will accept a pp.PollRequest from a receiver and push it on to be distributed.
// Here we also track metrics for received polls
func (sr *snapshotReceiver) Poll(ctx context.Context, pollRequest *pb.PollRequest) (*pb.PollResponse, error) {
	name := receivers.ExtractReceiverName(ctx)
	span, ctx := opentracing.StartSpanFromContext(ctx, "SnapshotReceiver.Poll")
	span.SetTag("receiver", name)
	defer span.Finish()

	start := time.Now()
	pollResponse, err := sr.pusher.PushPoll(ctx, pollRequest)
	metricPollDuration.Observe(time.Since(start).Seconds())

	// we always have a tenant ID by this point
	userID, _ := user.ExtractOrgID(ctx)

	if err != nil {
		sr.logger.Log("msg", "pusher failed to consume trace data", "err", err)
		metricPollRefused.WithLabelValues(userID, name).Inc()
	}
	metricPollAccepted.WithLabelValues(userID, name).Inc()
	return pollResponse, err
}

// Send will accept a tp.Snapshot from a receiver and push it on to be distributed.
// Here we also track metrics for received snapshots
func (sr *snapshotReceiver) Send(ctx context.Context, in *tp.Snapshot) (*tp.SnapshotResponse, error) {
	name := receivers.ExtractReceiverName(ctx)
	span, ctx := opentracing.StartSpanFromContext(ctx, "SnapshotReceiver.Send")
	span.SetTag("receiver", name)
	defer span.Finish()

	start := time.Now()
	snapshotResponse, err := sr.pusher.PushSnapshot(ctx, in)
	metricPushDuration.Observe(time.Since(start).Seconds())

	// we always have a tenant ID by this point
	userID, _ := user.ExtractOrgID(ctx)

	if err != nil {
		sr.logger.Log("msg", "pusher failed to consume trace data", "err", err)
		metricDistributorRefused.WithLabelValues(userID, name).Inc()
	}

	metricDistributorAccepted.WithLabelValues(userID, name).Inc()
	return snapshotResponse, err
}

// New creates a new snapshotReceiver that can accept and process snapshots
func New(receiverCfg map[string]interface{}, pusher SnapshotPusher, middleware Middleware, logger gkLog.Logger) (services.Service, tp.SnapshotServiceServer, error) {
	receiver := &snapshotReceiver{
		pusher: pusher,
		logger: log.NewRateLimitedLogger(logsPerSecond, level.Error(log.Logger)),
		fatal:  make(chan error),
	}

	receiversFor, err := receivers.ForConfig(receiverCfg, middleware.WrapSnapshots(receiver.Send), middleware.WrapPoll(receiver.Poll), logger)
	if err != nil {
		return nil, nil, err
	}
	receiver.receivers = receiversFor

	return services.NewBasicService(receiver.starting, receiver.running, receiver.stopping), receiver, nil
}

func (sr *snapshotReceiver) running(ctx context.Context) error {
	select {
	case err := <-sr.fatal:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (sr *snapshotReceiver) starting(ctx context.Context) error {
	for _, receiver := range sr.receivers {
		err := receiver.Start(ctx, sr)
		if err != nil {
			return fmt.Errorf("error starting receiver %w", err)
		}
	}
	return nil
}

// Called after distributor is asked to stop via StopAsync.
func (sr *snapshotReceiver) stopping(_ error) error {
	// when shutdown is called on the receiver it immediately shuts down its connection
	// which drops requests on the floor. at this point in the shutdown process
	// the readiness handler is already down so we are not receiving any more requests.
	// sleep for 30 seconds to here to all pending requests to finish.
	time.Sleep(30 * time.Second)

	ctx, cancelFn := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelFn()

	errs := make([]error, 0)

	for _, receiver := range sr.receivers {
		err := receiver.Shutdown(ctx)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return multierr.Combine(errs...)
	}

	return nil
}
