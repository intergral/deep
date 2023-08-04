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
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/grafana/dskit/runtimeconfig"
	"github.com/grafana/dskit/services"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"

	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/log"
)

const wildcardTenant = "*"

var (
	metricOverridesLimitsDesc = prometheus.NewDesc(
		"deep_limits_overrides",
		"Resource limit overrides applied to tenants",
		[]string{"limit_name", "tenant"},
		nil,
	)
)

// perTenantOverrides represents the overrides config file
type perTenantOverrides struct {
	TenantLimits map[string]*Limits `yaml:"overrides"`
}

// forTenant returns limits for a given tenant, or nil if there are no tenant-specific limits.
func (o *perTenantOverrides) forTenant(tenantID string) *Limits {
	l, ok := o.TenantLimits[tenantID]
	if !ok || l == nil {
		return nil
	}
	return l
}

// loadPerTenantOverrides is of type runtimeconfig.Loader
func loadPerTenantOverrides(r io.Reader) (interface{}, error) {
	var overrides = &perTenantOverrides{}

	decoder := yaml.NewDecoder(r)
	decoder.SetStrict(true)
	if err := decoder.Decode(&overrides); err != nil {
		return nil, err
	}

	return overrides, nil
}

// Config is a struct used to print the complete runtime config (defaults + overrides)
type Config struct {
	Defaults           *Limits            `yaml:"defaults"`
	PerTenantOverrides perTenantOverrides `yaml:",inline"`
}

// Overrides periodically fetch a set of per-user overrides, and provides convenience
// functions for fetching the correct value.
type Overrides struct {
	services.Service

	defaultLimits    *Limits
	runtimeConfigMgr *runtimeconfig.Manager

	// Manager for subservices
	subservices        *services.Manager
	subservicesWatcher *services.FailureWatcher
}

// NewOverrides makes a new Overrides.
// We store the supplied limits in a global variable to ensure per-tenant limits
// are defaulted to those values.  As such, the last call to NewOverrides will
// become the new global defaults.
func NewOverrides(defaults Limits) (*Overrides, error) {
	var manager *runtimeconfig.Manager
	subservices := []services.Service(nil)

	if defaults.PerTenantOverrideConfig != "" {
		runtimeCfg := runtimeconfig.Config{
			LoadPath:     []string{defaults.PerTenantOverrideConfig},
			ReloadPeriod: time.Duration(defaults.PerTenantOverridePeriod),
			Loader:       loadPerTenantOverrides,
		}
		runtimeCfgMgr, err := runtimeconfig.New(runtimeCfg, prometheus.WrapRegistererWithPrefix("deep_", prometheus.DefaultRegisterer), log.Logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create runtime config manager %w", err)
		}
		manager = runtimeCfgMgr
		subservices = append(subservices, runtimeCfgMgr)
	}

	o := &Overrides{
		runtimeConfigMgr: manager,
		defaultLimits:    &defaults,
	}

	if len(subservices) > 0 {
		var err error
		o.subservices, err = services.NewManager(subservices...)
		if err != nil {
			return nil, fmt.Errorf("failed to create subservices %w", err)
		}
		o.subservicesWatcher = services.NewFailureWatcher()
		o.subservicesWatcher.WatchManager(o.subservices)
	}

	o.Service = services.NewBasicService(o.starting, o.running, o.stopping)

	return o, nil
}

func (o *Overrides) starting(ctx context.Context) error {
	if o.subservices != nil {
		err := services.StartManagerAndAwaitHealthy(ctx, o.subservices)
		if err != nil {
			return fmt.Errorf("failed to start subservices %w", err)
		}
	}

	return nil
}

func (o *Overrides) running(ctx context.Context) error {
	if o.subservices != nil {
		select {
		case <-ctx.Done():
			return nil
		case err := <-o.subservicesWatcher.Chan():
			return fmt.Errorf("overrides subservices failed %w", err)
		}
	}
	<-ctx.Done()
	return nil
}

func (o *Overrides) stopping(_ error) error {
	if o.subservices != nil {
		return services.StopManagerAndAwaitStopped(context.Background(), o.subservices)
	}
	return nil
}

func (o *Overrides) tenantOverrides() *perTenantOverrides {
	if o.runtimeConfigMgr == nil {
		return nil
	}
	cfg, ok := o.runtimeConfigMgr.GetConfig().(*perTenantOverrides)
	if !ok || cfg == nil {
		return nil
	}

	return cfg
}

func (o *Overrides) WriteStatusRuntimeConfig(w io.Writer, r *http.Request) error {
	var tenantOverrides perTenantOverrides
	if o.tenantOverrides() != nil {
		tenantOverrides = *o.tenantOverrides()
	}
	var output interface{}
	cfg := Config{
		Defaults:           o.defaultLimits,
		PerTenantOverrides: tenantOverrides,
	}

	mode := r.URL.Query().Get("mode")
	switch mode {
	case "diff":
		// Default runtime config is just empty struct, but to make diff work,
		// we set defaultLimits for every tenant that exists in runtime config.
		defaultCfg := perTenantOverrides{TenantLimits: map[string]*Limits{}}
		defaultCfg.TenantLimits = map[string]*Limits{}
		for k, v := range tenantOverrides.TenantLimits {
			if v != nil {
				defaultCfg.TenantLimits[k] = o.defaultLimits
			}
		}

		cfgYaml, err := util.YAMLMarshalUnmarshal(cfg.PerTenantOverrides)
		if err != nil {
			return err
		}

		defaultCfgYaml, err := util.YAMLMarshalUnmarshal(defaultCfg)
		if err != nil {
			return err
		}

		output, err = util.DiffConfig(defaultCfgYaml, cfgYaml)
		if err != nil {
			return err
		}
	default:
		output = cfg
	}

	out, err := yaml.Marshal(output)
	if err != nil {
		return err
	}

	_, err = w.Write(out)
	if err != nil {
		return err
	}

	return nil
}

func (o *Overrides) TenantIDs() []string {
	tenantOverrides := o.tenantOverrides()
	if tenantOverrides == nil {
		return nil
	}

	result := make([]string, 0, len(tenantOverrides.TenantLimits))
	for userID := range tenantOverrides.TenantLimits {
		result = append(result, userID)
	}

	return result
}

// IngestionRateStrategy returns whether the ingestion rate limit should be individually applied
// to each distributor instance (local) or evenly shared across the cluster (global).
func (o *Overrides) IngestionRateStrategy() string {
	// The ingestion rate strategy can't be overridden on a per-tenant basis,
	// so here we just pick the value for a not-existing user ID (empty string).
	return o.getOverridesForTenant("").IngestionRateStrategy
}

// MaxLocalSnapshotsPerTenant returns the maximum number of snapshots a user is allowed to store
// in a single ingester.
func (o *Overrides) MaxLocalSnapshotsPerTenant(tenantID string) int {
	return o.getOverridesForTenant(tenantID).MaxLocalSnapshotsPerTenant
}

// MaxGlobalSnapshotsPerTenant returns the maximum number of snapshots a user is allowed to store
// across the cluster.
func (o *Overrides) MaxGlobalSnapshotsPerTenant(tenantID string) int {
	return o.getOverridesForTenant(tenantID).MaxGlobalSnapshotsPerTenant
}

// MaxBytesPerSnapshot returns the maximum size of a single snapshot in bytes allowed for a user.
func (o *Overrides) MaxBytesPerSnapshot(tenantID string) int {
	return o.getOverridesForTenant(tenantID).MaxBytesPerSnapshot
}

// Forwarders returns the list of forwarder IDs for a user.
func (o *Overrides) Forwarders(tenantID string) []string {
	return o.getOverridesForTenant(tenantID).Forwarders
}

// MaxBytesPerTagValuesQuery returns the maximum size of a response to a tag-values query allowed for a user.
func (o *Overrides) MaxBytesPerTagValuesQuery(tenantID string) int {
	return o.getOverridesForTenant(tenantID).MaxBytesPerTagValuesQuery
}

// IngestionRateLimitBytes is the number of spans per second allowed for this tenant.
func (o *Overrides) IngestionRateLimitBytes(tenantID string) float64 {
	return float64(o.getOverridesForTenant(tenantID).IngestionRateLimitBytes)
}

// IngestionBurstSizeBytes is the burst size in spans allowed for this tenant.
func (o *Overrides) IngestionBurstSizeBytes(tenantID string) int {
	return o.getOverridesForTenant(tenantID).IngestionBurstSizeBytes
}

// MetricsGeneratorRingSize is the desired size of the metrics-generator ring for this tenant.
// Using shuffle sharding, a tenant can use a smaller ring than the entire ring.
func (o *Overrides) MetricsGeneratorRingSize(tenantID string) int {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorRingSize
}

// MetricsGeneratorProcessors returns the metrics-generator processors enabled for this tenant.
func (o *Overrides) MetricsGeneratorProcessors(tenantID string) map[string]struct{} {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorProcessors.GetMap()
}

// MetricsGeneratorMaxActiveSeries is the maximum amount of active series in the metrics-generator
// registry for this tenant. Note this is a local limit enforced in every instance separately.
func (o *Overrides) MetricsGeneratorMaxActiveSeries(tenantID string) uint32 {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorMaxActiveSeries
}

// MetricsGeneratorCollectionInterval is the collection interval of the metrics-generator registry
// for this tenant.
func (o *Overrides) MetricsGeneratorCollectionInterval(tenantID string) time.Duration {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorCollectionInterval
}

// MetricsGeneratorDisableCollection controls whether metrics are remote written for this tenant.
func (o *Overrides) MetricsGeneratorDisableCollection(tenantID string) bool {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorDisableCollection
}

// MetricsGeneratorForwarderQueueSize is the size of the buffer of requests to send to the metrics-generator
// from the distributor for this tenant.
func (o *Overrides) MetricsGeneratorForwarderQueueSize(tenantID string) int {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorForwarderQueueSize
}

// MetricsGeneratorForwarderWorkers is the number of workers to send metrics to the metrics-generator
func (o *Overrides) MetricsGeneratorForwarderWorkers(tenantID string) int {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorForwarderWorkers
}

// MetricsGeneratorProcessorServiceGraphsHistogramBuckets controls the histogram buckets to be used
// by the service graphs processor.
func (o *Overrides) MetricsGeneratorProcessorServiceGraphsHistogramBuckets(tenantID string) []float64 {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorProcessorServiceGraphsHistogramBuckets
}

// MetricsGeneratorProcessorServiceGraphsDimensions controls the dimensions that are added to the
// service graphs processor.
func (o *Overrides) MetricsGeneratorProcessorServiceGraphsDimensions(tenantID string) []string {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorProcessorServiceGraphsDimensions
}

// MetricsGeneratorProcessorSpanMetricsHistogramBuckets controls the histogram buckets to be used
// by the span metrics processor.
func (o *Overrides) MetricsGeneratorProcessorSpanMetricsHistogramBuckets(tenantID string) []float64 {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorProcessorSpanMetricsHistogramBuckets
}

// MetricsGeneratorProcessorSpanMetricsDimensions controls the dimensions that are added to the
// span metrics processor.
func (o *Overrides) MetricsGeneratorProcessorSpanMetricsDimensions(tenantID string) []string {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorProcessorSpanMetricsDimensions
}

// MetricsGeneratorProcessorSpanMetricsIntrinsicDimensions controls the intrinsic dimensions such as service, span_kind, or
// span_name that are activated or deactivated on the span metrics processor.
func (o *Overrides) MetricsGeneratorProcessorSpanMetricsIntrinsicDimensions(tenantID string) map[string]bool {
	return o.getOverridesForTenant(tenantID).MetricsGeneratorProcessorSpanMetricsIntrinsicDimensions
}

// BlockRetention is the duration of the block retention for this tenant.
func (o *Overrides) BlockRetention(tenantID string) time.Duration {
	return time.Duration(o.getOverridesForTenant(tenantID).BlockRetention)
}

// MaxSearchDuration is the duration of the max search duration for this tenant.
func (o *Overrides) MaxSearchDuration(tenantID string) time.Duration {
	return time.Duration(o.getOverridesForTenant(tenantID).MaxSearchDuration)
}

func (o *Overrides) getOverridesForTenant(tenantID string) *Limits {
	if tenantOverrides := o.tenantOverrides(); tenantOverrides != nil {
		l := tenantOverrides.forTenant(tenantID)
		if l != nil {
			return l
		}

		l = tenantOverrides.forTenant(wildcardTenant)
		if l != nil {
			return l
		}
	}

	return o.defaultLimits
}

func (o *Overrides) Describe(ch chan<- *prometheus.Desc) {
	ch <- metricOverridesLimitsDesc
}

func (o *Overrides) Collect(ch chan<- prometheus.Metric) {
	overrides := o.tenantOverrides()
	if overrides == nil {
		return
	}

	for tenant, limits := range overrides.TenantLimits {
		ch <- prometheus.MustNewConstMetric(metricOverridesLimitsDesc, prometheus.GaugeValue, float64(limits.MaxLocalSnapshotsPerTenant), MetricMaxLocalSnapshotsPerTenant, tenant)
		ch <- prometheus.MustNewConstMetric(metricOverridesLimitsDesc, prometheus.GaugeValue, float64(limits.MaxGlobalSnapshotsPerTenant), MetricMaxGlobalSnapshotsPerTenant, tenant)
		ch <- prometheus.MustNewConstMetric(metricOverridesLimitsDesc, prometheus.GaugeValue, float64(limits.MaxBytesPerSnapshot), MetricMaxBytesPerSnapshot, tenant)
		ch <- prometheus.MustNewConstMetric(metricOverridesLimitsDesc, prometheus.GaugeValue, float64(limits.IngestionRateLimitBytes), MetricIngestionRateLimitBytes, tenant)
		ch <- prometheus.MustNewConstMetric(metricOverridesLimitsDesc, prometheus.GaugeValue, float64(limits.IngestionBurstSizeBytes), MetricIngestionBurstSizeBytes, tenant)
		ch <- prometheus.MustNewConstMetric(metricOverridesLimitsDesc, prometheus.GaugeValue, float64(limits.BlockRetention), MetricBlockRetention, tenant)
		ch <- prometheus.MustNewConstMetric(metricOverridesLimitsDesc, prometheus.GaugeValue, float64(limits.MetricsGeneratorMaxActiveSeries), MetricMetricsGeneratorMaxActiveSeries, tenant)
	}
}
