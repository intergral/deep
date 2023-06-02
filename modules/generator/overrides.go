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
	"github.com/intergral/deep/modules/generator/registry"
	"github.com/intergral/deep/modules/overrides"
)

type metricsGeneratorOverrides interface {
	registry.Overrides

	MetricsGeneratorProcessors(userID string) map[string]struct{}
	MetricsGeneratorProcessorServiceGraphsHistogramBuckets(userID string) []float64
	MetricsGeneratorProcessorServiceGraphsDimensions(userID string) []string
	MetricsGeneratorProcessorSpanMetricsHistogramBuckets(userID string) []float64
	MetricsGeneratorProcessorSpanMetricsDimensions(userID string) []string
	MetricsGeneratorProcessorSpanMetricsIntrinsicDimensions(userID string) map[string]bool
}

var _ metricsGeneratorOverrides = (*overrides.Overrides)(nil)
