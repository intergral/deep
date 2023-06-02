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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/modules/generator/processor/spanmetrics"
)

func TestProcessorConfig_copyWithOverrides(t *testing.T) {
	original := &ProcessorConfig{
		SpanMetrics: spanmetrics.Config{
			HistogramBuckets:    []float64{1, 2},
			Dimensions:          []string{"namespace"},
			IntrinsicDimensions: spanmetrics.IntrinsicDimensions{Service: true},
		},
	}

	t.Run("overrides buckets and dimension", func(t *testing.T) {
		o := &mockOverrides{
			serviceGraphsHistogramBuckets:  []float64{1, 2},
			serviceGraphsDimensions:        []string{"namespace"},
			spanMetricsHistogramBuckets:    []float64{1, 2, 3},
			spanMetricsDimensions:          []string{"cluster", "namespace"},
			spanMetricsIntrinsicDimensions: map[string]bool{"line_no": true},
		}

		copied, err := original.copyWithOverrides(o, "tenant")
		require.NoError(t, err)

		assert.NotEqual(t, *original, copied)

		// assert nothing changed
		assert.Equal(t, []float64{1, 2}, original.SpanMetrics.HistogramBuckets)
		assert.Equal(t, []string{"namespace"}, original.SpanMetrics.Dimensions)
		assert.Equal(t, spanmetrics.IntrinsicDimensions{Service: true}, original.SpanMetrics.IntrinsicDimensions)

		// assert overrides were applied
		assert.Equal(t, []float64{1, 2, 3}, copied.SpanMetrics.HistogramBuckets)
		assert.Equal(t, []string{"cluster", "namespace"}, copied.SpanMetrics.Dimensions)
		assert.Equal(t, spanmetrics.IntrinsicDimensions{Service: true, LineNo: true}, copied.SpanMetrics.IntrinsicDimensions)
	})

	t.Run("empty overrides", func(t *testing.T) {
		o := &mockOverrides{}

		copied, err := original.copyWithOverrides(o, "tenant")
		require.NoError(t, err)

		assert.Equal(t, *original, copied)
	})

	t.Run("invalid overrides", func(t *testing.T) {
		o := &mockOverrides{
			spanMetricsIntrinsicDimensions: map[string]bool{"invalid": true},
		}

		_, err := original.copyWithOverrides(o, "tenant")
		require.Error(t, err)
	})
}
