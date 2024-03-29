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
	"fmt"
	"time"

	"github.com/intergral/deep/pkg/cache"
	"github.com/intergral/deep/pkg/deepdb/backend/azure"
	"github.com/intergral/deep/pkg/deepdb/backend/cache/memcached"
	"github.com/intergral/deep/pkg/deepdb/backend/cache/redis"
	"github.com/intergral/deep/pkg/deepdb/backend/gcs"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/backend/s3"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/pool"
	"github.com/intergral/deep/pkg/deepdb/wal"
)

const (
	DefaultBlocklistPoll            = 5 * time.Minute
	DefaultMaxTimePerTenant         = 5 * time.Minute
	DefaultBlocklistPollConcurrency = uint(50)
	DefaultRetentionConcurrency     = uint(10)
	DefaultTenantIndexBuilders      = 2

	DefaultPrefetchSnapshotCount = 1000
	DefaultSearchChunkSizeBytes  = 1_000_000
	DefaultReadBufferCount       = 32
	DefaultReadBufferSize        = 1 * 1024 * 1024
)

// Config holds the entirety of deepdb configuration
// Defaults are in modules/storage/config.go
type Config struct {
	Pool   *pool.Config        `yaml:"pool,omitempty"`
	WAL    *wal.Config         `yaml:"wal"`
	Block  *common.BlockConfig `yaml:"block"`
	Search *SearchConfig       `yaml:"search"`

	BlocklistPoll                    time.Duration `yaml:"blocklist_poll"`
	BlocklistPollConcurrency         uint          `yaml:"blocklist_poll_concurrency"`
	BlocklistPollFallback            bool          `yaml:"blocklist_poll_fallback"`
	BlocklistPollTenantIndexBuilders int           `yaml:"blocklist_poll_tenant_index_builders"`
	BlocklistPollStaleTenantIndex    time.Duration `yaml:"blocklist_poll_stale_tenant_index"`
	BlocklistPollJitterMs            int           `yaml:"blocklist_poll_jitter_ms"`

	// backends
	Backend string        `yaml:"backend"`
	Local   *local.Config `yaml:"local"`
	GCS     *gcs.Config   `yaml:"gcs"`
	S3      *s3.Config    `yaml:"s3"`
	Azure   *azure.Config `yaml:"azure"`

	// caches
	Cache                   string                  `yaml:"cache"`
	CacheMinCompactionLevel uint8                   `yaml:"cache_min_compaction_level"`
	CacheMaxBlockAge        time.Duration           `yaml:"cache_max_block_age"`
	BackgroundCache         *cache.BackgroundConfig `yaml:"background_cache"`
	Memcached               *memcached.Config       `yaml:"memcached"`
	Redis                   *redis.Config           `yaml:"redis"`
}

type SearchConfig struct {
	// v2 blocks
	ChunkSizeBytes        uint32 `yaml:"chunk_size_bytes"`
	PrefetchSnapshotCount int    `yaml:"prefetch_snapshot_count"`

	// vParquet blocks
	ReadBufferCount     int `yaml:"read_buffer_count"`
	ReadBufferSizeBytes int `yaml:"read_buffer_size_bytes"`
	CacheControl        struct {
		Footer      bool `yaml:"footer"`
		ColumnIndex bool `yaml:"column_index"`
		OffsetIndex bool `yaml:"offset_index"`
	} `yaml:"cache_control"`
}

func (c SearchConfig) ApplyToOptions(o *common.SearchOptions) {
	o.ChunkSizeBytes = c.ChunkSizeBytes
	o.PrefetchSnapshotCount = c.PrefetchSnapshotCount
	o.ReadBufferCount = c.ReadBufferCount
	o.ReadBufferSize = c.ReadBufferSizeBytes

	if o.ChunkSizeBytes == 0 {
		o.ChunkSizeBytes = DefaultSearchChunkSizeBytes
	}
	if o.PrefetchSnapshotCount <= 0 {
		o.PrefetchSnapshotCount = DefaultPrefetchSnapshotCount
	}
	if o.ReadBufferSize <= 0 {
		o.ReadBufferSize = DefaultReadBufferSize
	}
	if o.ReadBufferCount <= 0 {
		o.ReadBufferCount = DefaultReadBufferCount
	}

	o.CacheControl.Footer = c.CacheControl.Footer
	o.CacheControl.ColumnIndex = c.CacheControl.ColumnIndex
	o.CacheControl.OffsetIndex = c.CacheControl.OffsetIndex
}

// CompactorConfig contains compaction configuration options
type CompactorConfig struct {
	ChunkSizeBytes          uint32        `yaml:"v2_in_buffer_bytes"`
	FlushSizeBytes          uint32        `yaml:"v2_out_buffer_bytes"`
	IteratorBufferSize      int           `yaml:"v2_prefetch_snapshot_count"`
	MaxCompactionRange      time.Duration `yaml:"compaction_window"`
	MaxCompactionObjects    int           `yaml:"max_compaction_objects"`
	MaxBlockBytes           uint64        `yaml:"max_block_bytes"`
	BlockRetention          time.Duration `yaml:"block_retention"`
	CompactedBlockRetention time.Duration `yaml:"compacted_block_retention"`
	RetentionConcurrency    uint          `yaml:"retention_concurrency"`
	MaxTimePerTenant        time.Duration `yaml:"max_time_per_tenant"`
	CompactionCycle         time.Duration `yaml:"compaction_cycle"`
}

func validateConfig(cfg *Config) error {
	if cfg == nil {
		return errors.New("config should be non-nil")
	}

	if cfg.WAL == nil {
		return errors.New("wal config should be non-nil")
	}

	if cfg.Block == nil {
		return errors.New("block config should be non-nil")
	}

	// if the wal version is unspecified default to the block version
	if cfg.WAL.Version == "" {
		cfg.WAL.Version = cfg.Block.Version
	}

	err := wal.ValidateConfig(cfg.WAL)
	if err != nil {
		return fmt.Errorf("wal config validation failed: %w", err)
	}

	err = common.ValidateConfig(cfg.Block)
	if err != nil {
		return fmt.Errorf("block config validation failed: %w", err)
	}

	_, err = encoding.FromVersion(cfg.Block.Version)
	if err != nil {
		return fmt.Errorf("block version validation failed: %w", err)
	}

	return nil
}
