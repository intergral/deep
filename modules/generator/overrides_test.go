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

import "time"

type mockOverrides struct {
	processors                     map[string]struct{}
	serviceGraphsHistogramBuckets  []float64
	serviceGraphsDimensions        []string
	spanMetricsHistogramBuckets    []float64
	spanMetricsDimensions          []string
	spanMetricsIntrinsicDimensions map[string]bool
}

var _ metricsGeneratorOverrides = (*mockOverrides)(nil)

func (m *mockOverrides) MetricsGeneratorMaxActiveSeries(userID string) uint32 {
	return 0
}

func (m *mockOverrides) MetricsGeneratorCollectionInterval(userID string) time.Duration {
	return 15 * time.Second
}

func (m *mockOverrides) MetricsGeneratorProcessors(userID string) map[string]struct{} {
	return m.processors
}

func (m *mockOverrides) MetricsGeneratorDisableCollection(userID string) bool {
	return false
}

func (m *mockOverrides) MetricsGeneratorProcessorServiceGraphsHistogramBuckets(userID string) []float64 {
	return m.serviceGraphsHistogramBuckets
}

func (m *mockOverrides) MetricsGeneratorProcessorServiceGraphsDimensions(userID string) []string {
	return m.serviceGraphsDimensions
}

func (m *mockOverrides) MetricsGeneratorProcessorSpanMetricsHistogramBuckets(userID string) []float64 {
	return m.spanMetricsHistogramBuckets
}

func (m *mockOverrides) MetricsGeneratorProcessorSpanMetricsDimensions(userID string) []string {
	return m.spanMetricsDimensions
}

func (m *mockOverrides) MetricsGeneratorProcessorSpanMetricsIntrinsicDimensions(userID string) map[string]bool {
	return m.spanMetricsIntrinsicDimensions
}
