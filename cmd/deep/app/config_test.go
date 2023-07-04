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

package app

import (
	"testing"
	"time"

	"github.com/intergral/deep/modules/distributor"
	"github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/pkg/deepdb"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
	"github.com/stretchr/testify/assert"
)

func TestConfig_CheckConfig(t *testing.T) {
	tt := []struct {
		name   string
		config *Config
		expect []ConfigWarning
	}{
		{
			name:   "check default cfg and expect no warnings",
			config: newDefaultConfig(),
			expect: nil,
		},
		{
			name: "hit all except local backend warnings",
			config: &Config{
				Target: MetricsGenerator,
				StorageConfig: storage.Config{
					TracePoint: deepdb.Config{
						Backend:       "s3",
						BlocklistPoll: time.Minute,
						Block: &common.BlockConfig{
							Version: "v2",
						},
					},
				},
				Distributor: distributor.Config{
					LogReceivedSnapshots: distributor.LogReceivedSnapshotsConfig{
						Enabled: true,
					},
				},
			},
			expect: []ConfigWarning{
				warnCompleteBlockTimeout,
				warnBlockRetention,
				warnRetentionConcurrency,
				warnStorageTraceBackendS3,
				warnBlocklistPollConcurrency,
			},
		},
		{
			name: "hit local backend warnings",
			config: func() *Config {
				cfg := newDefaultConfig()
				cfg.StorageConfig.TracePoint = deepdb.Config{
					Backend:                  "local",
					BlocklistPollConcurrency: 1,
					Block: &common.BlockConfig{
						Version: "v2",
					},
				}
				cfg.Target = "something"
				return cfg
			}(),
			expect: []ConfigWarning{warnStorageTraceBackendLocal},
		},
		{
			name: "warnings for v2 settings when they drift from default",
			config: func() *Config {
				cfg := newDefaultConfig()
				cfg.StorageConfig.TracePoint.Block.Version = vparquet.VersionString
				cfg.Compactor.Compactor.ChunkSizeBytes = 1
				cfg.Compactor.Compactor.FlushSizeBytes = 1
				cfg.Compactor.Compactor.IteratorBufferSize = 1
				return cfg
			}(),
			expect: []ConfigWarning{
				newV2Warning("v2_in_buffer_bytes"),
				newV2Warning("v2_out_buffer_bytes"),
				newV2Warning("v2_prefetch_traces_count"),
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			warnings := tc.config.CheckConfig()
			assert.Equal(t, tc.expect, warnings)
		})
	}
}
