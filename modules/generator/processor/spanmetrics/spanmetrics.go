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

package spanmetrics

import (
	"context"
	"github.com/golang/protobuf/proto"
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	deep_util "github.com/intergral/deep/pkg/util"
	"strconv"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/prometheus/util/strutil"

	gen "github.com/intergral/deep/modules/generator/processor"
	processor_util "github.com/intergral/deep/modules/generator/processor/util"
	"github.com/intergral/deep/modules/generator/registry"
	"github.com/intergral/deep/pkg/deeppb"
)

const (
	metricCallsTotal      = "snapshot_metrics_calls_total"
	metricDurationSeconds = "snapshot_metrics_latency"
	metricSizeTotal       = "snapshot_metrics_size_total"
)

type Processor struct {
	Cfg Config

	registry registry.Registry

	spanMetricsCallsTotal      registry.Counter
	spanMetricsDurationSeconds registry.Histogram
	spanMetricsSizeTotal       registry.Counter

	// for testing
	now func() time.Time
}

func New(cfg Config, registry registry.Registry) gen.Processor {
	labels := make([]string, 0, 4+len(cfg.Dimensions))

	if cfg.IntrinsicDimensions.Service {
		labels = append(labels, dimService)
	}
	if cfg.IntrinsicDimensions.FilePath {
		labels = append(labels, dimFilePath)
	}
	if cfg.IntrinsicDimensions.LineNo {
		labels = append(labels, dimLineNo)
	}

	for _, d := range cfg.Dimensions {
		labels = append(labels, sanitizeLabelNameWithCollisions(d))
	}

	return &Processor{
		Cfg:                        cfg,
		registry:                   registry,
		spanMetricsCallsTotal:      registry.NewCounter(metricCallsTotal, labels),
		spanMetricsDurationSeconds: registry.NewHistogram(metricDurationSeconds, labels, cfg.HistogramBuckets),
		spanMetricsSizeTotal:       registry.NewCounter(metricSizeTotal, labels),
		now:                        time.Now,
	}
}

func (p *Processor) Name() string {
	return Name
}

func (p *Processor) PushSnapshot(ctx context.Context, req *deeppb.PushSnapshotRequest) {
	span, _ := opentracing.StartSpanFromContext(ctx, "spanmetrics.PushSpans")
	defer span.Finish()

	p.aggregateMetrics(req.Snapshot)
}

func (p *Processor) Shutdown(_ context.Context) {
}

func (p *Processor) aggregateMetrics(req *tp.Snapshot) {
	svcName, _ := processor_util.FindServiceName(req.Resource)
	p.aggregateMetricsForSpan(svcName, req)
}

func (p *Processor) aggregateMetricsForSpan(svcName string, snapshot *tp.Snapshot) {
	latencySeconds := float64(snapshot.GetDurationNanos()) / float64(time.Second.Nanoseconds())

	labelValues := make([]string, 0, 4+len(p.Cfg.Dimensions))
	// important: the order of labelValues must correspond to the order of labels / intrinsic dimensions
	if p.Cfg.IntrinsicDimensions.Service {
		labelValues = append(labelValues, svcName)
	}
	if p.Cfg.IntrinsicDimensions.FilePath {
		labelValues = append(labelValues, snapshot.GetTracepoint().GetPath())
	}
	if p.Cfg.IntrinsicDimensions.LineNo {
		labelValues = append(labelValues, strconv.Itoa(int(snapshot.GetTracepoint().LineNumber)))
	}

	for _, d := range p.Cfg.Dimensions {
		value, _ := processor_util.FindAttributeValue(d, snapshot.Resource, snapshot.Attributes)
		labelValues = append(labelValues, value)
	}
	spanMultiplier := processor_util.GetSpanMultiplier(p.Cfg.SpanMultiplierKey, snapshot.Attributes)

	registryLabelValues := p.registry.NewLabelValues(labelValues)

	p.spanMetricsCallsTotal.Inc(registryLabelValues, 1*spanMultiplier)
	p.spanMetricsSizeTotal.Inc(registryLabelValues, float64(proto.Size(snapshot))*spanMultiplier)
	p.spanMetricsDurationSeconds.ObserveWithExemplar(registryLabelValues, latencySeconds, deep_util.TraceIDToHexString(snapshot.ID), spanMultiplier)
}

func sanitizeLabelNameWithCollisions(name string) string {
	sanitized := strutil.SanitizeLabelName(name)

	if isIntrinsicDimension(sanitized) {
		return "__" + sanitized
	}

	return sanitized
}

func isIntrinsicDimension(name string) bool {
	return name == dimService ||
		name == dimFilePath ||
		name == dimLineNo
}
