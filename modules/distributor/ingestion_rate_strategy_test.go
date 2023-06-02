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
	"testing"

	"github.com/grafana/dskit/limiter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/modules/overrides"
)

func TestIngestionRateStrategy(t *testing.T) {
	tests := map[string]struct {
		limits        overrides.Limits
		ring          ReadLifecycler
		expectedLimit float64
		expectedBurst int
	}{
		"local rate limiter should just return configured limits": {
			limits: overrides.Limits{
				IngestionRateStrategy:   overrides.LocalIngestionRateStrategy,
				IngestionRateLimitBytes: 5,
				IngestionBurstSizeBytes: 2,
			},
			ring:          nil,
			expectedLimit: 5,
			expectedBurst: 2,
		},
		"global rate limiter should share the limit across the number of distributors": {
			limits: overrides.Limits{
				IngestionRateStrategy:   overrides.GlobalIngestionRateStrategy,
				IngestionRateLimitBytes: 5,
				IngestionBurstSizeBytes: 2,
			},
			ring: func() ReadLifecycler {
				ring := newReadLifecyclerMock()
				ring.On("HealthyInstancesCount").Return(2)
				return ring
			}(),
			expectedLimit: 2.5,
			expectedBurst: 2,
		},
	}

	for testName, testData := range tests {
		testData := testData

		t.Run(testName, func(t *testing.T) {
			var strategy limiter.RateLimiterStrategy

			// Init limits overrides
			o, err := overrides.NewOverrides(testData.limits)
			require.NoError(t, err)

			// Instance the strategy
			switch testData.limits.IngestionRateStrategy {
			case overrides.LocalIngestionRateStrategy:
				strategy = newLocalIngestionRateStrategy(o)
			case overrides.GlobalIngestionRateStrategy:
				strategy = newGlobalIngestionRateStrategy(o, testData.ring)
			default:
				require.Fail(t, "Unknown strategy")
			}

			assert.Equal(t, testData.expectedLimit, strategy.Limit("test"))
			assert.Equal(t, testData.expectedBurst, strategy.Burst("test"))
		})
	}
}

type readLifecyclerMock struct {
	mock.Mock
}

func newReadLifecyclerMock() *readLifecyclerMock {
	return &readLifecyclerMock{}
}

func (m *readLifecyclerMock) HealthyInstancesCount() int {
	args := m.Called()
	return args.Int(0)
}
