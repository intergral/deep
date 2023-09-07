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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/dskit/services"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestOverrides(t *testing.T) {
	tests := []struct {
		name                           string
		limits                         Limits
		overrides                      *perTenantOverrides
		expectedMaxLocalSnapshots      map[string]int
		expectedMaxGlobalSnapshots     map[string]int
		expectedMaxBytesPerSnapshot    map[string]int
		expectedIngestionRateSnapshot  map[string]int
		expectedIngestionBurstSnapshot map[string]int
		expectedMaxSearchDuration      map[string]int
	}{
		{
			name: "limits only",
			limits: Limits{
				MaxGlobalSnapshotsPerTenant: 1,
				MaxLocalSnapshotsPerTenant:  2,
				MaxBytesPerSnapshot:         3,
				IngestionBurstSizeBytes:     4,
				IngestionRateLimitBytes:     5,
			},
			expectedMaxGlobalSnapshots:     map[string]int{"user1": 1, "user2": 1},
			expectedMaxLocalSnapshots:      map[string]int{"user1": 2, "user2": 2},
			expectedMaxBytesPerSnapshot:    map[string]int{"user1": 3, "user2": 3},
			expectedIngestionBurstSnapshot: map[string]int{"user1": 4, "user2": 4},
			expectedIngestionRateSnapshot:  map[string]int{"user1": 5, "user2": 5},
			expectedMaxSearchDuration:      map[string]int{"user1": 0, "user2": 0},
		},
		{
			name: "basic overrides",
			limits: Limits{
				MaxGlobalSnapshotsPerTenant: 1,
				MaxLocalSnapshotsPerTenant:  2,
				MaxBytesPerSnapshot:         3,
				IngestionBurstSizeBytes:     4,
				IngestionRateLimitBytes:     5,
			},
			overrides: &perTenantOverrides{
				TenantLimits: map[string]*Limits{
					"user1": {
						MaxGlobalSnapshotsPerTenant: 6,
						MaxLocalSnapshotsPerTenant:  7,
						MaxBytesPerSnapshot:         8,
						IngestionBurstSizeBytes:     9,
						IngestionRateLimitBytes:     10,
						MaxSearchDuration:           model.Duration(11 * time.Second),
					},
				},
			},
			expectedMaxGlobalSnapshots:     map[string]int{"user1": 6, "user2": 1},
			expectedMaxLocalSnapshots:      map[string]int{"user1": 7, "user2": 2},
			expectedMaxBytesPerSnapshot:    map[string]int{"user1": 8, "user2": 3},
			expectedIngestionBurstSnapshot: map[string]int{"user1": 9, "user2": 4},
			expectedIngestionRateSnapshot:  map[string]int{"user1": 10, "user2": 5},
			expectedMaxSearchDuration:      map[string]int{"user1": int(11 * time.Second), "user2": 0},
		},
		{
			name: "wildcard override",
			limits: Limits{
				MaxGlobalSnapshotsPerTenant: 1,
				MaxLocalSnapshotsPerTenant:  2,
				MaxBytesPerSnapshot:         3,
				IngestionBurstSizeBytes:     4,
				IngestionRateLimitBytes:     5,
			},
			overrides: &perTenantOverrides{
				TenantLimits: map[string]*Limits{
					"user1": {
						MaxGlobalSnapshotsPerTenant: 6,
						MaxLocalSnapshotsPerTenant:  7,
						MaxBytesPerSnapshot:         8,
						IngestionBurstSizeBytes:     9,
						IngestionRateLimitBytes:     10,
					},
					"*": {
						MaxGlobalSnapshotsPerTenant: 11,
						MaxLocalSnapshotsPerTenant:  12,
						MaxBytesPerSnapshot:         13,
						IngestionBurstSizeBytes:     14,
						IngestionRateLimitBytes:     15,
						MaxSearchDuration:           model.Duration(16 * time.Second),
					},
				},
			},
			expectedMaxGlobalSnapshots:     map[string]int{"user1": 6, "user2": 11},
			expectedMaxLocalSnapshots:      map[string]int{"user1": 7, "user2": 12},
			expectedMaxBytesPerSnapshot:    map[string]int{"user1": 8, "user2": 13},
			expectedIngestionBurstSnapshot: map[string]int{"user1": 9, "user2": 14},
			expectedIngestionRateSnapshot:  map[string]int{"user1": 10, "user2": 15},
			expectedMaxSearchDuration:      map[string]int{"user1": 0, "user2": int(16 * time.Second)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.overrides != nil {
				overridesFile := filepath.Join(t.TempDir(), "overrides.yaml")

				buff, err := yaml.Marshal(tt.overrides)
				require.NoError(t, err)

				err = os.WriteFile(overridesFile, buff, os.ModePerm)
				require.NoError(t, err)

				tt.limits.PerTenantOverrideConfig = overridesFile
				tt.limits.PerTenantOverridePeriod = model.Duration(time.Hour)
			}

			prometheus.DefaultRegisterer = prometheus.NewRegistry() // have to overwrite the registry or test panics with multiple metric reg
			overrides, err := NewOverrides(tt.limits)
			require.NoError(t, err)
			err = services.StartAndAwaitRunning(context.TODO(), overrides)
			require.NoError(t, err)

			for user, expectedVal := range tt.expectedMaxLocalSnapshots {
				assert.Equal(t, expectedVal, overrides.MaxLocalSnapshotsPerTenant(user))
			}

			for user, expectedVal := range tt.expectedMaxGlobalSnapshots {
				assert.Equal(t, expectedVal, overrides.MaxGlobalSnapshotsPerTenant(user))
			}

			for user, expectedVal := range tt.expectedIngestionBurstSnapshot {
				assert.Equal(t, expectedVal, overrides.IngestionBurstSizeBytes(user))
			}

			for user, expectedVal := range tt.expectedIngestionRateSnapshot {
				assert.Equal(t, float64(expectedVal), overrides.IngestionRateLimitBytes(user))
			}

			for user, expectedVal := range tt.expectedMaxSearchDuration {
				assert.Equal(t, time.Duration(expectedVal), overrides.MaxSearchDuration(user))
			}

			// if srv != nil {
			err = services.StopAndAwaitTerminated(context.TODO(), overrides)
			require.NoError(t, err)
			//}
		})
	}
}
