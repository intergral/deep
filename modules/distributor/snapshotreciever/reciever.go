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

package snapshotreciever

import (
	"context"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/services"
	"github.com/intergral/deep/pkg/util/log"
	tp "github.com/intergral/go-deep-proto/tracepoint/v1"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/weaveworks/common/logging"
	"github.com/weaveworks/common/user"
	"go.opencensus.io/stats/view"
	"time"
)

const (
	logsPerSecond = 10
)

var (
	metricPushDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "deep",
		Name:      "distributor_push_duration_seconds",
		Help:      "Records the amount of time to push a batch to the ingester.",
		Buckets:   prometheus.DefBuckets,
	})
	metricDistributorAccepted = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_accepted_snapshots",
		Help:      "Number of snapshots successfully pushed into the pipeline.",
	}, []string{"tenant"},
	)
	metricDistributorRefused = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_refused_snapshots",
		Help:      "Number of snapshots that could not be pushed into the pipeline.",
	}, []string{"tenant"})
)

type SnapshotPusher interface {
	PushSnapshot(ctx context.Context, traces *tp.Snapshot) (*tp.SnapshotResponse, error)
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
}

func (sr *snapshotReceiver) Send(ctx context.Context, in *tp.Snapshot) (*tp.SnapshotResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "SnapshotReceiver.Send")
	defer span.Finish()

	start := time.Now()
	snapshotResponse, err := sr.pusher.PushSnapshot(ctx, in)
	metricPushDuration.Observe(time.Since(start).Seconds())

	userID, err := user.ExtractOrgID(ctx)
	if err != nil {
		// can't record discarded spans here b/c there's no tenant
		return nil, err
	}
	if err != nil {
		sr.logger.Log("msg", "pusher failed to consume trace data", "err", err)
		metricDistributorRefused.WithLabelValues(userID).Inc()
	}

	metricDistributorAccepted.WithLabelValues(userID).Inc()
	return snapshotResponse, err
}

func New(receiverCfg map[string]interface{}, pusher SnapshotPusher, logLevel logging.Level) (services.Service, tp.SnapshotServiceServer, error) {
	receiver := &snapshotReceiver{
		pusher: pusher,
		logger: log.NewRateLimitedLogger(logsPerSecond, level.Error(log.Logger)),
		fatal:  make(chan error),
	}

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

	return nil
}

// Called after distributor is asked to stop via StopAsync.
func (sr *snapshotReceiver) stopping(_ error) error {
	return nil
}
