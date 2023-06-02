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
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/services"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/multierr"

	"github.com/intergral/deep/modules/distributor/queue"
	"github.com/intergral/deep/modules/overrides"
)

const (
	defaultWorkerCount = 2
	defaultQueueSize   = 100
)

var (
	metricForwarderPushes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_forwarder_pushes_total",
		Help:      "Total number of successful requests queued up for a tenant to the generatorForwarder. This metric is now deprecated in favor of deep_distributor_queue_pushes_total.",
	}, []string{"tenant"})
	metricForwarderPushesFailures = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Name:      "distributor_forwarder_pushes_failures_total",
		Help:      "Total number of failed pushes to the queue for a tenant to the generatorForwarder. This metric is now deprecated in favor of deep_distributor_queue_pushes_failures_total.",
	}, []string{"tenant"})
)

type forwardFunc func(ctx context.Context, tenantID string, keys []uint32, snapshot *tp.Snapshot) error

type request struct {
	tenantID string
	keys     []uint32
	snapshot *tp.Snapshot
}

// generatorForwarder queues up traces to be sent to the metrics-generators
type generatorForwarder struct {
	services.Service

	logger log.Logger

	// per-tenant queues
	queues map[string]*queue.Queue[*request]
	mutex  sync.RWMutex

	forwardFunc forwardFunc

	o                 *overrides.Overrides
	overridesInterval time.Duration
	shutdown          chan interface{}
}

func newGeneratorForwarder(logger log.Logger, fn forwardFunc, o *overrides.Overrides) *generatorForwarder {
	rf := &generatorForwarder{
		logger:            logger,
		queues:            make(map[string]*queue.Queue[*request]),
		mutex:             sync.RWMutex{},
		forwardFunc:       fn,
		o:                 o,
		overridesInterval: time.Minute,
		shutdown:          make(chan interface{}),
	}

	rf.Service = services.NewIdleService(rf.start, rf.stop)

	return rf
}

// getQueueConfig returns queueSize and workerCount for the given tenant
func (f *generatorForwarder) getQueueConfig(tenantID string) (queueSize, workerCount int) {
	queueSize = f.o.MetricsGeneratorForwarderQueueSize(tenantID)
	if queueSize == 0 {
		queueSize = defaultQueueSize
	}

	workerCount = f.o.MetricsGeneratorForwarderWorkers(tenantID)
	if workerCount == 0 {
		workerCount = defaultWorkerCount
	}
	return queueSize, workerCount
}

func (f *generatorForwarder) getOrCreateQueue(tenantID string) *queue.Queue[*request] {
	q, ok := f.getQueue(tenantID)
	if ok {
		return q
	}

	f.mutex.Lock()
	defer f.mutex.Unlock()

	queueSize, workerCount := f.getQueueConfig(tenantID)
	f.queues[tenantID] = f.createQueueAndStartWorkers(tenantID, queueSize, workerCount)

	return f.queues[tenantID]
}

func (f *generatorForwarder) getQueue(tenantID string) (*queue.Queue[*request], bool) {
	f.mutex.RLock()
	defer f.mutex.RUnlock()

	q, ok := f.queues[tenantID]
	return q, ok
}

// watchOverrides watches the overrides for changes
// and updates the queues accordingly
func (f *generatorForwarder) watchOverrides() {
	ticker := time.NewTicker(f.overridesInterval)

	for {
		select {
		case <-ticker.C:
			f.mutex.Lock()

			var (
				queuesToDelete []*queue.Queue[*request]
				queuesToAdd    []struct {
					tenantID               string
					queueSize, workerCount int
				}
			)

			for tenantID, q := range f.queues {
				queueSize, workerCount := f.getQueueConfig(tenantID)
				// if the queue size or worker count has changed, shutdown the queue manager and create a new one
				if q.ShouldUpdate(queueSize, workerCount) {
					_ = level.Info(f.logger).Log(
						"msg", "Marking queue manager for update",
						"tenant", tenantID,
						"old_queue_size", q.Size(),
						"new_queue_size", queueSize,
						"old_worker_count", q.WorkerCount(),
						"new_worker_count", workerCount,
					)
					queuesToDelete = append(queuesToDelete, q)
					queuesToAdd = append(queuesToAdd, struct {
						tenantID               string
						queueSize, workerCount int
					}{tenantID: tenantID, queueSize: queueSize, workerCount: workerCount})
				}
			}

			// Spawn a goroutine to asynchronously shut down queue managers
			go func() {
				for _, q := range queuesToDelete {
					// shutdown the queue manager
					// this will block until all workers have finished and the queue is drained
					_ = level.Info(f.logger).Log("msg", "Shutting down queue manager", "tenant", q.TenantID())
					if err := q.Shutdown(context.Background()); err != nil {
						_ = level.Error(f.logger).Log("msg", "error shutting down queue manager", "tenant", q.TenantID(), "err", err)
					}
				}
			}()

			// Synchronously update queue managers
			for _, q := range queuesToAdd {
				_ = level.Info(f.logger).Log("msg", "Updating queue manager", "tenant", q.tenantID)
				f.queues[q.tenantID] = f.createQueueAndStartWorkers(q.tenantID, q.queueSize, q.workerCount)
			}

			f.mutex.Unlock()
		case <-f.shutdown:
			ticker.Stop()
			return
		}
	}
}

func (f *generatorForwarder) start(_ context.Context) error {
	go f.watchOverrides()

	return nil

}

func (f *generatorForwarder) stop(_ error) error {
	close(f.shutdown)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var errs []error
	for _, q := range f.queues {
		if err := q.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return multierr.Combine(errs...)
}

func (f *generatorForwarder) processFunc(ctx context.Context, data *request) {
	if err := f.forwardFunc(ctx, data.tenantID, data.keys, data.snapshot); err != nil {
		_ = level.Warn(f.logger).Log("msg", "failed to forward request to metrics generator", "err", err)
	}
}

func (f *generatorForwarder) createQueueAndStartWorkers(tenantID string, size, workerCount int) *queue.Queue[*request] {
	q := queue.New(
		queue.Config{
			Name:        "metrics-generator",
			TenantID:    tenantID,
			Size:        size,
			WorkerCount: workerCount,
		},
		f.logger,
		f.processFunc,
	)
	q.StartWorkers()

	return q
}

// SendSnapshot queues up snapshots to be sent to the metrics-generators
func (f *generatorForwarder) SendSnapshot(ctx context.Context, tenantID string, keys []uint32, snapshot *tp.Snapshot) {
	select {
	case <-f.shutdown:
		return
	default:
	}

	q := f.getOrCreateQueue(tenantID)
	err := q.Push(ctx, &request{tenantID: tenantID, keys: keys, snapshot: snapshot})
	if err != nil {
		_ = level.Error(f.logger).Log("msg", "failed to push snapshot to queue", "tenant", tenantID, "err", err)
		metricForwarderPushesFailures.WithLabelValues(tenantID).Inc()
	}

	metricForwarderPushes.WithLabelValues(tenantID).Inc()
}
