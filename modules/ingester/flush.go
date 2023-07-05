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

package ingester

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	gklog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/grafana/dskit/services"
	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/uber/jaeger-client-go"
	"github.com/weaveworks/common/user"

	"github.com/intergral/deep/pkg/util/log"
)

var (
	metricBlocksFlushed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "blocks_flushed_total",
		Help:      "The total number of blocks flushed to storage",
	})
	metricFailedFlushes = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "failed_operations_total",
		Help:      "The total number of failed operations",
	}, []string{"operation"})
	metricFlushRetries = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "operation_retries_total",
		Help:      "The total number of retries after a failed operation",
	}, []string{"operation"})
	metricFlushFailedRetries = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "failed_retried_operations",
		Help:      "The total number of operations that failed, even after a retry.",
	}, []string{"operation"})
	metricFlushDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "operation_duration_seconds",
		Help:      "Records the amount of time to complete an operation.",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10),
	}, []string{"operation"})
	metricFlushSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "flush_size_bytes",
		Help:      "Size in bytes of blocks flushed.",
		Buckets:   prometheus.ExponentialBuckets(1024*1024, 2, 10), // from 1MB up to 1GB
	})
)

const (
	initialBackoff      = 30 * time.Second
	flushJitter         = 10 * time.Second
	maxBackoff          = 120 * time.Second
	maxCompleteAttempts = 3
)

const (
	opKindComplete = iota
	opKindFlush
)

// Flush triggers a flush of all in memory snapshots to disk.  This is called
// by the lifecycler on shutdown and will put our snapshots in the WAL to be
// replayed.
func (i *Ingester) Flush() {
	instances := i.getInstances()

	for _, instance := range instances {
		err := instance.CutSnapshots(0, true)
		if err != nil {
			level.Error(log.WithUserID(instance.tenantID, log.Logger)).Log("msg", "failed to cut complete snapshots on shutdown", "err", err)
		}
	}
}

// ShutdownHandler handles a graceful shutdown for an ingester. It does the following things in order
// * Stop incoming writes by exiting from the ring
// * Flush all blocks to backend
func (i *Ingester) ShutdownHandler(w http.ResponseWriter, _ *http.Request) {
	go func() {
		level.Info(log.Logger).Log("msg", "shutdown handler called")

		// lifecycler should exit the ring on shutdown
		i.lifecycler.SetUnregisterOnShutdown(true)

		// stop accepting new writes
		i.markUnavailable()

		// move all data into flushQueue
		i.sweepAllInstances(true)

		for !i.flushQueues.IsEmpty() {
			time.Sleep(100 * time.Millisecond)
		}

		// stop ingester service
		_ = services.StopAndAwaitTerminated(context.Background(), i)

		level.Info(log.Logger).Log("msg", "shutdown handler complete")
	}()

	_, _ = w.Write([]byte("shutdown job acknowledged"))
}

// FlushHandler calls sweepAllInstances(true) which will force push all snapshots into the WAL and force
// mark all head blocks as ready to flush.
func (i *Ingester) FlushHandler(w http.ResponseWriter, _ *http.Request) {
	i.sweepAllInstances(true)
	w.WriteHeader(http.StatusNoContent)
}

type flushOp struct {
	kind     int
	at       time.Time // When to execute
	attempts uint
	backoff  time.Duration
	tenantID string
	blockID  uuid.UUID
}

func (o *flushOp) Key() string {
	return o.tenantID + "/" + strconv.Itoa(o.kind) + "/" + o.blockID.String()
}

// Priority orders entries in the queue. The larger the number the higher the priority, so inverted here to
// prioritize entries with earliest timestamps.
func (o *flushOp) Priority() int64 {
	return -o.at.Unix()
}

// sweepAllInstances periodically schedules series for flushing and garbage collects instances with no series
func (i *Ingester) sweepAllInstances(immediate bool) {
	instances := i.getInstances()

	for _, instance := range instances {
		i.sweepInstance(instance, immediate)
	}
}

func (i *Ingester) sweepInstance(instance *tenantBlockManager, immediate bool) {
	// cut snapshots internally
	err := instance.CutSnapshots(i.cfg.MaxSnapshotIdle, immediate)
	if err != nil {
		level.Error(log.WithUserID(instance.tenantID, log.Logger)).Log("msg", "failed to cut snapshots", "err", err)
		return
	}

	// see if it's ready to cut a block
	blockID, err := instance.CutBlockIfReady(i.cfg.MaxBlockDuration, i.cfg.MaxBlockBytes, immediate)
	if err != nil {
		level.Error(log.WithUserID(instance.tenantID, log.Logger)).Log("msg", "failed to cut block", "err", err)
		return
	}

	if blockID != uuid.Nil {
		level.Info(log.Logger).Log("msg", "head block cut. enqueueing flush op", "userid", instance.tenantID, "block", blockID)
		// jitter to help when flushing many instances at the same time
		// no jitter if immediate (initiated via /flush handler for example)
		i.enqueue(&flushOp{
			kind:     opKindComplete,
			tenantID: instance.tenantID,
			blockID:  blockID,
		}, !immediate)
	}

	// dump any blocks that have been flushed for a while
	err = instance.ClearFlushedBlocks(i.cfg.CompleteBlockTimeout)
	if err != nil {
		level.Error(log.WithUserID(instance.tenantID, log.Logger)).Log("msg", "failed to complete block", "err", err)
	}
}

func (i *Ingester) flushLoop(j int) {
	defer func() {
		level.Debug(log.Logger).Log("msg", "Ingester.flushLoop() exited")
		i.flushQueuesDone.Done()
	}()

	for {
		o := i.flushQueues.Dequeue(j)
		if o == nil {
			return
		}

		op := o.(*flushOp)
		op.attempts++

		var retry bool
		var err error
		start := time.Now()
		if op.kind == opKindComplete {
			retry, err = i.handleComplete(op)
		} else {
			retry, err = i.handleFlush(context.Background(), op.tenantID, op.blockID)
		}
		metricFlushDuration.WithLabelValues(opName(op)).Observe(time.Since(start).Seconds())

		if err != nil {
			handleFailedOp(op, err)
		}

		if retry {
			i.requeue(op)
		} else {
			i.flushQueues.Clear(op)
		}
	}
}

func handleFailedOp(op *flushOp, err error) {
	level.Error(log.WithUserID(op.tenantID, log.Logger)).Log("msg", "error performing op in flushQueue",
		"op", op.kind, "block", op.blockID.String(), "attempts", op.attempts, "err", err)
	opName := opName(op)
	metricFailedFlushes.WithLabelValues(opName).Inc()

	if op.attempts > 1 {
		metricFlushFailedRetries.WithLabelValues(opName).Inc()
	}
}

func opName(op *flushOp) string {
	opName := "opKindComplete"
	if op.kind != opKindComplete {
		opName = "opKindFlush"
	}
	return opName
}

func handleAbandonedOp(op *flushOp) {
	level.Info(log.WithUserID(op.tenantID, log.Logger)).Log("msg", "Abandoning op in flush queue because ingester is shutting down",
		"op", op.kind, "block", op.blockID.String(), "attempts", op.attempts)
}

func (i *Ingester) handleComplete(op *flushOp) (retry bool, err error) {
	// No point in proceeding if shutdown has been initiated since
	// we won't be able to queue up the next flush op
	if i.flushQueues.IsStopped() {
		handleAbandonedOp(op)
		return false, nil
	}

	start := time.Now()
	level.Info(log.Logger).Log("msg", "completing block", "userid", op.tenantID, "blockID", op.blockID)
	instance, err := i.getOrCreateInstance(op.tenantID)
	if err != nil {
		return false, err
	}

	err = instance.CompleteBlock(op.blockID)
	level.Info(log.Logger).Log("msg", "block completed", "userid", op.tenantID, "blockID", op.blockID, "duration", time.Since(start))
	if err != nil {
		handleFailedOp(op, err)

		if op.attempts >= maxCompleteAttempts {
			// todo alerts?
			level.Error(log.WithUserID(op.tenantID, log.Logger)).Log("msg", "Block exceeded max completion errors. Deleting. POSSIBLE DATA LOSS",
				"userID", op.tenantID, "attempts", op.attempts, "block", op.blockID.String())

			// Delete WAL and move on
			err = instance.ClearCompletingBlock(op.blockID)
			return false, err
		}

		return true, nil
	}

	err = instance.ClearCompletingBlock(op.blockID)
	if err != nil {
		return false, errors.Wrap(err, "error clearing completing block")
	}

	// add a flushOp for the block we just completed
	// No delay
	i.enqueue(&flushOp{
		kind:     opKindFlush,
		tenantID: instance.tenantID,
		blockID:  op.blockID,
	}, false)

	return false, nil
}

// withSpan adds traceID to a logger, if span is sampled
// TODO: move into some central trace/log package
func withSpan(logger gklog.Logger, sp ot.Span) gklog.Logger {
	if sp == nil {
		return logger
	}
	sctx, ok := sp.Context().(jaeger.SpanContext)
	if !ok || !sctx.IsSampled() {
		return logger
	}

	return gklog.With(logger, "traceId", sctx.TraceID().String())
}

func (i *Ingester) handleFlush(ctx context.Context, tenantID string, blockID uuid.UUID) (retry bool, err error) {
	sp, ctx := ot.StartSpanFromContext(ctx, "flush", ot.Tag{Key: "organization", Value: tenantID}, ot.Tag{Key: "blockID", Value: blockID.String()})
	defer sp.Finish()
	withSpan(level.Info(log.Logger), sp).Log("msg", "flushing block", "tenantID", tenantID, "block", blockID.String())

	instance, err := i.getOrCreateInstance(tenantID)
	if err != nil {
		return true, err
	}

	if instance == nil {
		return false, fmt.Errorf("tenantBlockManager id %s not found", tenantID)
	}

	if block := instance.GetBlockToBeFlushed(blockID); block != nil {
		ctx := user.InjectOrgID(ctx, tenantID)
		ctx, cancel := context.WithTimeout(ctx, i.cfg.FlushOpTimeout)
		defer cancel()

		err = i.store.WriteBlock(ctx, block)

		metricFlushSize.Observe(float64(block.BlockMeta().Size))
		if err != nil {
			ext.Error.Set(sp, true)
			sp.LogFields(otlog.Error(err))
			return true, err
		}

		metricBlocksFlushed.Inc()
	} else {
		return false, fmt.Errorf("error getting block to flush")
	}

	return false, nil
}

func (i *Ingester) enqueue(op *flushOp, jitter bool) {
	delay := time.Duration(0)

	if jitter {
		delay = time.Duration(rand.Float32() * float32(flushJitter))
	}

	op.at = time.Now().Add(delay)

	go func() {
		time.Sleep(delay)

		// Check if shutdown initiated
		if i.flushQueues.IsStopped() {
			handleAbandonedOp(op)
			return
		}

		err := i.flushQueues.Enqueue(op)
		if err != nil {
			handleFailedOp(op, err)
		}
	}()
}

func (i *Ingester) requeue(op *flushOp) {
	op.backoff *= 2
	if op.backoff < initialBackoff {
		op.backoff = initialBackoff
	}
	if op.backoff > maxBackoff {
		op.backoff = maxBackoff
	}

	op.at = time.Now().Add(op.backoff)

	level.Info(log.WithUserID(op.tenantID, log.Logger)).Log("msg", "retrying op in flushQueue",
		"op", op.kind, "block", op.blockID.String(), "backoff", op.backoff)

	go func() {
		time.Sleep(op.backoff)

		// Check if shutdown initiated
		if i.flushQueues.IsStopped() {
			handleAbandonedOp(op)
			return
		}

		metricFlushRetries.WithLabelValues(opName(op)).Inc()

		err := i.flushQueues.Requeue(op)
		if err != nil {
			handleFailedOp(op, err)
		}
	}()
}
