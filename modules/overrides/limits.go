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

package overrides

import (
	"flag"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

const (
	// LocalIngestionRateStrategy indicates that this limit can be evaluated in local terms only
	LocalIngestionRateStrategy = "local"
	// GlobalIngestionRateStrategy indicates that an attempt should be made to consider this limit across the entire Deep cluster
	GlobalIngestionRateStrategy = "global"

	// ErrorPrefixLiveSnapshotsExceeded is used to flag batches from the ingester that were rejected b/c they had too many snapshots
	ErrorPrefixLiveSnapshotsExceeded = "LIVE_SNAPSHOTS_EXCEEDED:"
	// ErrorPrefixSnapshotTooLarge is used to flag batches from the ingester that were rejected b/c they exceeded the single snapshot limit
	ErrorPrefixSnapshotTooLarge = "SNAPSHOT_TOO_LARGE:"
	// ErrorPrefixRateLimited is used to flag batches that have exceeded the snapshot/second of the tenant
	ErrorPrefixRateLimited = "RATE_LIMITED:"

	// metrics
	MetricMaxLocalSnapshotsPerTenant      = "max_local_snapshots_per_tenant"
	MetricMaxGlobalSnapshotsPerTenant     = "max_global_snapshots_per_tenant"
	MetricMaxBytesPerSnapshot             = "max_bytes_per_snapshot"
	MetricMaxBytesPerTagValuesQuery       = "max_bytes_per_tag_values_query"
	MetricIngestionRateLimitBytes         = "ingestion_rate_limit_bytes"
	MetricIngestionBurstSizeBytes         = "ingestion_burst_size_bytes"
	MetricBlockRetention                  = "block_retention"
	MetricMetricsGeneratorMaxActiveSeries = "metrics_generator_max_active_series"
)

var metricLimitsDesc = prometheus.NewDesc(
	"deep_limits_defaults",
	"Default resource limits",
	[]string{"limit_name"},
	nil,
)

// Limits describe all the limits for users; can be used to describe global default
// limits via flags, or per-user limits via yaml config.
type Limits struct {
	// Distributor enforced limits.
	IngestionRateStrategy   string `yaml:"ingestion_rate_strategy" json:"ingestion_rate_strategy"`
	IngestionRateLimitBytes int    `yaml:"ingestion_rate_limit_bytes" json:"ingestion_rate_limit_bytes"`
	IngestionBurstSizeBytes int    `yaml:"ingestion_burst_size_bytes" json:"ingestion_burst_size_bytes"`

	// Ingester enforced limits.
	MaxLocalSnapshotsPerTenant  int `yaml:"max_snapshots_per_tenant" json:"max_snapshots_per_tenant"`
	MaxGlobalSnapshotsPerTenant int `yaml:"max_global_snapshots_per_tenant" json:"max_global_snapshots_per_tenant"`

	// Forwarders
	Forwarders []string `yaml:"forwarders" json:"forwarders"`

	// Metrics-generator config
	MetricsGeneratorRingSize                                int             `yaml:"metrics_generator_ring_size" json:"metrics_generator_ring_size"`
	MetricsGeneratorProcessors                              ListToMap       `yaml:"metrics_generator_processors" json:"metrics_generator_processors"`
	MetricsGeneratorMaxActiveSeries                         uint32          `yaml:"metrics_generator_max_active_series" json:"metrics_generator_max_active_series"`
	MetricsGeneratorCollectionInterval                      time.Duration   `yaml:"metrics_generator_collection_interval" json:"metrics_generator_collection_interval"`
	MetricsGeneratorDisableCollection                       bool            `yaml:"metrics_generator_disable_collection" json:"metrics_generator_disable_collection"`
	MetricsGeneratorForwarderQueueSize                      int             `yaml:"metrics_generator_forwarder_queue_size" json:"metrics_generator_forwarder_queue_size"`
	MetricsGeneratorForwarderWorkers                        int             `yaml:"metrics_generator_forwarder_workers" json:"metrics_generator_forwarder_workers"`
	MetricsGeneratorProcessorServiceGraphsHistogramBuckets  []float64       `yaml:"metrics_generator_processor_service_graphs_histogram_buckets" json:"metrics_generator_processor_service_graphs_histogram_buckets"`
	MetricsGeneratorProcessorServiceGraphsDimensions        []string        `yaml:"metrics_generator_processor_service_graphs_dimensions" json:"metrics_generator_processor_service_graphs_dimensions"`
	MetricsGeneratorProcessorSpanMetricsHistogramBuckets    []float64       `yaml:"metrics_generator_processor_span_metrics_histogram_buckets" json:"metrics_generator_processor_span_metrics_histogram_buckets"`
	MetricsGeneratorProcessorSpanMetricsDimensions          []string        `yaml:"metrics_generator_processor_span_metrics_dimensions" json:"metrics_generator_processor_span_metrics_dimensions"`
	MetricsGeneratorProcessorSpanMetricsIntrinsicDimensions map[string]bool `yaml:"metrics_generator_processor_span_metrics_intrinsic_dimensions" json:"metrics_generator_processor_span_metrics_intrinsic_dimensions"`

	// Compactor enforced limits.
	BlockRetention model.Duration `yaml:"block_retention" json:"block_retention"`

	// Querier and Ingester enforced limits.
	MaxBytesPerTagValuesQuery int `yaml:"max_bytes_per_tag_values_query" json:"max_bytes_per_tag_values_query"`

	// QueryFrontend enforced limits
	MaxSearchDuration model.Duration `yaml:"max_search_duration" json:"max_search_duration"`

	// MaxBytesPerSnapshot is enforced in the Ingester, Compactor, Querier (Search) and Serverless (Search). It
	//  is not used when doing a snapshot by id lookup.
	MaxBytesPerSnapshot int `yaml:"max_bytes_per_snapshot" json:"max_bytes_per_snapshot"`

	// PerTenantOverrideConfig is the path to the per-tenant config
	PerTenantOverrideConfig string `yaml:"per_tenant_override_config" json:"per_tenant_override_config"`
	// PerTenantOverridePeriod the time between reloads of the override file
	PerTenantOverridePeriod model.Duration `yaml:"per_tenant_override_period" json:"per_tenant_override_period"`
}

// RegisterFlags adds the flags required to config this to the given FlagSet
func (l *Limits) RegisterFlags(f *flag.FlagSet) {
	// Distributor Limits
	f.StringVar(&l.IngestionRateStrategy, "distributor.rate-limit-strategy", "local", "Whether the various ingestion rate limits should be applied individually to each distributor instance (local), or evenly shared across the cluster (global).")
	f.IntVar(&l.IngestionRateLimitBytes, "distributor.ingestion-rate-limit-bytes", 15e6, "Per-user ingestion rate limit in bytes per second.")
	f.IntVar(&l.IngestionBurstSizeBytes, "distributor.ingestion-burst-size-bytes", 20e6, "Per-user ingestion burst size in bytes. Should be set to the expected size (in bytes) of a single push request.")

	// Ingester limits
	f.IntVar(&l.MaxLocalSnapshotsPerTenant, "ingester.max-snapshots-per-tenant", 10e3, "Maximum number of active snapshots per tenant, per ingester. 0 to disable.")
	f.IntVar(&l.MaxGlobalSnapshotsPerTenant, "ingester.max-global-snapshots-per-tenant", 0, "Maximum number of active snapshots per tenant, across the cluster. 0 to disable.")
	f.IntVar(&l.MaxBytesPerSnapshot, "ingester.max-bytes-per-snapshot", 50e5, "Maximum size of a snapshots in bytes.  0 to disable.")

	// Querier limits
	f.IntVar(&l.MaxBytesPerTagValuesQuery, "querier.max-bytes-per-tag-values-query", 50e5, "Maximum size of response for a tag-values query. Used mainly to limit large the number of values associated with a particular tag")

	f.StringVar(&l.PerTenantOverrideConfig, "limits.per-tenant-override-config", "", "File name of per tenant overrides.")
	_ = l.PerTenantOverridePeriod.Set("10s")
	f.Var(&l.PerTenantOverridePeriod, "limits.per-tenant-override-period", "Period with this to reload the overrides.")
}

func (l *Limits) Describe(ch chan<- *prometheus.Desc) {
	ch <- metricLimitsDesc
}

func (l *Limits) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(metricLimitsDesc, prometheus.GaugeValue, float64(l.MaxLocalSnapshotsPerTenant), MetricMaxLocalSnapshotsPerTenant)
	ch <- prometheus.MustNewConstMetric(metricLimitsDesc, prometheus.GaugeValue, float64(l.MaxGlobalSnapshotsPerTenant), MetricMaxGlobalSnapshotsPerTenant)
	ch <- prometheus.MustNewConstMetric(metricLimitsDesc, prometheus.GaugeValue, float64(l.MaxBytesPerSnapshot), MetricMaxBytesPerSnapshot)
	ch <- prometheus.MustNewConstMetric(metricLimitsDesc, prometheus.GaugeValue, float64(l.MaxBytesPerTagValuesQuery), MetricMaxBytesPerTagValuesQuery)
	ch <- prometheus.MustNewConstMetric(metricLimitsDesc, prometheus.GaugeValue, float64(l.IngestionRateLimitBytes), MetricIngestionRateLimitBytes)
	ch <- prometheus.MustNewConstMetric(metricLimitsDesc, prometheus.GaugeValue, float64(l.IngestionBurstSizeBytes), MetricIngestionBurstSizeBytes)
	ch <- prometheus.MustNewConstMetric(metricLimitsDesc, prometheus.GaugeValue, float64(l.BlockRetention), MetricBlockRetention)
	ch <- prometheus.MustNewConstMetric(metricLimitsDesc, prometheus.GaugeValue, float64(l.MetricsGeneratorMaxActiveSeries), MetricMetricsGeneratorMaxActiveSeries)
}
