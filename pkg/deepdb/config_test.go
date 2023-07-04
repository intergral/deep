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

package deepdb

import (
	"errors"
	"testing"

	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/wal"
	"github.com/stretchr/testify/require"
)

func TestApplyToOptions(t *testing.T) {
	opts := common.DefaultSearchOptions()
	cfg := SearchConfig{}

	// test defaults
	cfg.ApplyToOptions(&opts)
	require.Equal(t, opts.PrefetchTraceCount, DefaultPrefetchTraceCount)
	require.Equal(t, opts.ChunkSizeBytes, uint32(DefaultSearchChunkSizeBytes))
	require.Equal(t, opts.ReadBufferCount, DefaultReadBufferCount)
	require.Equal(t, opts.ReadBufferSize, DefaultReadBufferSize)

	// test parameter fields are left alone
	opts.StartPage = 1
	opts.TotalPages = 2
	opts.MaxBytes = 3
	cfg.ApplyToOptions(&opts)
	require.Equal(t, opts.StartPage, 1)
	require.Equal(t, opts.TotalPages, 2)
	require.Equal(t, opts.MaxBytes, 3)

	// test non defaults
	cfg.ChunkSizeBytes = 4
	cfg.PrefetchTraceCount = 5
	cfg.ReadBufferCount = 6
	cfg.ReadBufferSizeBytes = 7
	cfg.ApplyToOptions(&opts)
	require.Equal(t, cfg.ChunkSizeBytes, uint32(4))
	require.Equal(t, cfg.PrefetchTraceCount, 5)
	require.Equal(t, cfg.ReadBufferCount, 6)
	require.Equal(t, cfg.ReadBufferSizeBytes, 7)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		cfg            *Config
		expectedConfig *Config
		err            error
	}{
		// nil config fails
		{
			err: errors.New("config should be non-nil"),
		},
		// nil wal fails
		{
			cfg: &Config{},
			err: errors.New("wal config should be non-nil"),
		},
		// nil block fails
		{
			cfg: &Config{
				WAL: &wal.Config{},
			},
			err: errors.New("block config should be non-nil"),
		},
		// block version copied to wal if empty
		{
			cfg: &Config{
				WAL: &wal.Config{},
				Block: &common.BlockConfig{
					BloomFP:             0.01,
					BloomShardSizeBytes: 1,
					Version:             "vParquet",
				},
			},
			expectedConfig: &Config{
				WAL: &wal.Config{
					Version: "vParquet",
				},
				Block: &common.BlockConfig{
					BloomFP:             0.01,
					BloomShardSizeBytes: 1,
					Version:             "vParquet",
				},
			},
		},
		// block version not copied to wal if populated
		{
			cfg: &Config{
				WAL: &wal.Config{
					Version: "vParquet",
				},
				Block: &common.BlockConfig{
					BloomFP:             0.01,
					BloomShardSizeBytes: 1,
					Version:             "vParquet",
				},
			},
			expectedConfig: &Config{
				WAL: &wal.Config{
					Version: "vParquet",
				},
				Block: &common.BlockConfig{
					BloomFP:             0.01,
					BloomShardSizeBytes: 1,
					Version:             "vParquet",
				},
			},
		},
	}

	for _, test := range tests {
		err := validateConfig(test.cfg)
		require.Equal(t, test.err, err)

		if test.expectedConfig != nil {
			require.Equal(t, test.expectedConfig, test.cfg)
		}
	}
}
