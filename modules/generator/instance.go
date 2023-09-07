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

package generator

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/intergral/deep/modules/generator/processor"
	"github.com/intergral/deep/modules/generator/processor/spanmetrics"
	"github.com/intergral/deep/modules/generator/registry"
	"github.com/intergral/deep/modules/generator/storage"
	"github.com/intergral/deep/pkg/deeppb"
)

var (
	allSupportedProcessors = []string{spanmetrics.Name}

	metricActiveProcessors = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "deep",
		Subsystem: "metrics_generator",
		Name:      "active_processors",
		Help:      "The active processors per tenant",
	}, []string{"tenant", "processor"})
	metricActiveProcessorsUpdateFailed = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "metrics_generator",
		Name:      "active_processors_update_failed_total",
		Help:      "The total number of times updating the active processors failed",
	}, []string{"tenant"})
	metricSnapshotsIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "metrics_generator",
		Name:      "snapshots_received_total",
		Help:      "The total number of snapshots received per tenant",
	}, []string{"tenant"})
	metricBytesIngested = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "metrics_generator",
		Name:      "bytes_received_total",
		Help:      "The total number of proto bytes received per tenant",
	}, []string{"tenant"})
	metricSnapshotsDiscarded = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "metrics_generator",
		Name:      "snapshots_discarded_total",
		Help:      "The total number of discarded snapshots received per tenant",
	}, []string{"tenant", "reason"})
)

const reasonOutsideTimeRangeSlack = "outside_metrics_ingestion_slack"

type instance struct {
	cfg *Config

	tenantID  string
	overrides metricsGeneratorOverrides

	registry *registry.ManagedRegistry
	wal      storage.Storage

	// processorsMtx protects the processors map, not the processors itself
	processorsMtx sync.RWMutex
	// processors is a map of processor name -> processor, only one instance of a processor can be
	// active at any time
	processors map[string]processor.Processor

	shutdownCh chan struct{}

	reg    prometheus.Registerer
	logger log.Logger
}

func newInstance(cfg *Config, tenantID string, overrides metricsGeneratorOverrides, wal storage.Storage, reg prometheus.Registerer, logger log.Logger) (*instance, error) {
	logger = log.With(logger, "tenant", tenantID)

	i := &instance{
		cfg:       cfg,
		tenantID:  tenantID,
		overrides: overrides,

		registry: registry.New(&cfg.Registry, overrides, tenantID, wal, logger),
		wal:      wal,

		processors: make(map[string]processor.Processor),

		shutdownCh: make(chan struct{}, 1),

		reg:    reg,
		logger: logger,
	}

	err := i.updateProcessors()
	if err != nil {
		return nil, fmt.Errorf("could not initialize processors: %w", err)
	}
	go i.watchOverrides()

	return i, nil
}

func (i *instance) watchOverrides() {
	reloadPeriod := 10 * time.Second

	ticker := time.NewTicker(reloadPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := i.updateProcessors()
			if err != nil {
				metricActiveProcessorsUpdateFailed.WithLabelValues(i.tenantID).Inc()
				level.Error(i.logger).Log("msg", "updating the processors failed", "err", err)
			}

		case <-i.shutdownCh:
			return
		}
	}
}

func (i *instance) updateProcessors() error {
	desiredProcessors := i.overrides.MetricsGeneratorProcessors(i.tenantID)
	desiredCfg, err := i.cfg.Processor.copyWithOverrides(i.overrides, i.tenantID)
	if err != nil {
		return err
	}

	i.processorsMtx.RLock()
	toAdd, toRemove, toReplace, err := i.diffProcessors(desiredProcessors, desiredCfg)
	i.processorsMtx.RUnlock()

	if err != nil {
		return err
	}
	if len(toAdd) == 0 && len(toRemove) == 0 && len(toReplace) == 0 {
		return nil
	}

	i.processorsMtx.Lock()
	defer i.processorsMtx.Unlock()

	for _, processorName := range toAdd {
		err := i.addProcessor(processorName, desiredCfg)
		if err != nil {
			return err
		}
	}
	for _, processorName := range toRemove {
		i.removeProcessor(processorName)
	}
	for _, processorName := range toReplace {
		i.removeProcessor(processorName)

		err := i.addProcessor(processorName, desiredCfg)
		if err != nil {
			return err
		}
	}

	i.updateProcessorMetrics()

	return nil
}

// diffProcessors compares the existing processors with the desired processors and config.
// Must be called under a read lock.
func (i *instance) diffProcessors(desiredProcessors map[string]struct{}, desiredCfg ProcessorConfig) (toAdd, toRemove, toReplace []string, err error) {
	for processorName := range desiredProcessors {
		if _, ok := i.processors[processorName]; !ok {
			toAdd = append(toAdd, processorName)
		}
	}
	for processorName, proc := range i.processors {
		if _, ok := desiredProcessors[processorName]; !ok {
			toRemove = append(toRemove, processorName)
			continue
		}

		switch p := proc.(type) {
		case *spanmetrics.Processor:
			if !reflect.DeepEqual(p.Cfg, desiredCfg.SpanMetrics) {
				toReplace = append(toReplace, processorName)
			}
		default:
			level.Error(i.logger).Log(
				"msg", fmt.Sprintf("processor does not exist, supported processors: [%s]", strings.Join(allSupportedProcessors, ", ")),
				"processorName", processorName,
			)
			err = fmt.Errorf("unknown processor %s", processorName)
			return
		}
	}
	return
}

// addProcessor registers the processor and adds it to the processors map. Must be called under a
// write lock.
func (i *instance) addProcessor(processorName string, cfg ProcessorConfig) error {
	level.Debug(i.logger).Log("msg", "adding processor", "processorName", processorName)

	var newProcessor processor.Processor
	switch processorName {
	case spanmetrics.Name:
		newProcessor = spanmetrics.New(cfg.SpanMetrics, i.registry)
	default:
		level.Error(i.logger).Log(
			"msg", fmt.Sprintf("processor does not exist, supported processors: [%s]", strings.Join(allSupportedProcessors, ", ")),
			"processorName", processorName,
		)
		return fmt.Errorf("unknown processor %s", processorName)
	}

	// check the processor wasn't added in the meantime
	if _, ok := i.processors[processorName]; ok {
		return nil
	}

	i.processors[processorName] = newProcessor

	return nil
}

// removeProcessor removes the processor from the processors map and shuts it down. Must be called
// under a write lock.
func (i *instance) removeProcessor(processorName string) {
	level.Debug(i.logger).Log("msg", "removing processor", "processorName", processorName)

	deletedProcessor, ok := i.processors[processorName]
	if !ok {
		return
	}

	delete(i.processors, processorName)

	deletedProcessor.Shutdown(context.Background())
}

// updateProcessorMetrics updates the active processor metrics. Must be called under a read lock.
func (i *instance) updateProcessorMetrics() {
	for _, processorName := range allSupportedProcessors {
		isPresent := 0.0
		if _, ok := i.processors[processorName]; ok {
			isPresent = 1.0
		}
		metricActiveProcessors.WithLabelValues(i.tenantID, processorName).Set(isPresent)
	}
}

func (i *instance) PushSnapshot(ctx context.Context, req *deeppb.PushSnapshotRequest) {
	i.preprocessSnapshots(req)
	i.processorsMtx.RLock()
	defer i.processorsMtx.RUnlock()

	for _, metricProcessor := range i.processors {
		metricProcessor.PushSnapshot(ctx, req)
	}
}

func (i *instance) preprocessSnapshots(req *deeppb.PushSnapshotRequest) {
	size := proto.Size(req)
	snapshotCount := 0
	expiredSnapshotCount := 0
	timeNow := time.Now()
	if int64(req.Snapshot.TsNanos) >= timeNow.Add(-i.cfg.MetricsIngestionSlack).UnixNano() && int64(req.Snapshot.TsNanos) <= timeNow.Add(i.cfg.MetricsIngestionSlack).UnixNano() {
		snapshotCount++
	} else {
		expiredSnapshotCount++
	}
	i.updatePushMetrics(size, snapshotCount, expiredSnapshotCount)
}

func (i *instance) updatePushMetrics(bytesIngested int, snapshotCount int, expiredSnapshotCount int) {
	metricBytesIngested.WithLabelValues(i.tenantID).Add(float64(bytesIngested))
	metricSnapshotsIngested.WithLabelValues(i.tenantID).Add(float64(snapshotCount))
	metricSnapshotsDiscarded.WithLabelValues(i.tenantID, reasonOutsideTimeRangeSlack).Add(float64(expiredSnapshotCount))
}

// shutdown stops the instance and flushes any remaining data. After shutdown
// is called pushSpans should not be called anymore.
func (i *instance) shutdown() {
	close(i.shutdownCh)

	i.processorsMtx.Lock()
	defer i.processorsMtx.Unlock()

	for processorName := range i.processors {
		i.removeProcessor(processorName)
	}

	i.registry.Close()

	err := i.wal.Close()
	if err != nil {
		level.Error(i.logger).Log("msg", "closing wal failed", "tenant", i.tenantID, "err", err)
	}
}
