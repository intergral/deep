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

package compactor

import (
	"flag"
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/ring"
	"github.com/intergral/deep/pkg/deepdb"
	"github.com/intergral/deep/pkg/util"
)

type Config struct {
	Disabled        bool                   `yaml:"disabled,omitempty"`
	ShardingRing    RingConfig             `yaml:"ring,omitempty"`
	Compactor       deepdb.CompactorConfig `yaml:"compaction"`
	OverrideRingKey string                 `yaml:"override_ring_key"`
}

// RegisterFlagsAndApplyDefaults registers the flags.
func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {
	cfg.Compactor = deepdb.CompactorConfig{
		ChunkSizeBytes:          deepdb.DefaultChunkSizeBytes, // 5 MiB
		FlushSizeBytes:          deepdb.DefaultFlushSizeBytes,
		CompactedBlockRetention: time.Hour,
		RetentionConcurrency:    deepdb.DefaultRetentionConcurrency,
		IteratorBufferSize:      deepdb.DefaultIteratorBufferSize,
		MaxTimePerTenant:        deepdb.DefaultMaxTimePerTenant,
		CompactionCycle:         deepdb.DefaultCompactionCycle,
	}

	flagext.DefaultValues(&cfg.ShardingRing)
	cfg.ShardingRing.KVStore.Store = "" // by default compactor is not sharded

	f.DurationVar(&cfg.Compactor.BlockRetention, util.PrefixConfig(prefix, "compaction.block-retention"), 14*24*time.Hour, "Duration to keep blocks/traces.")
	f.IntVar(&cfg.Compactor.MaxCompactionObjects, util.PrefixConfig(prefix, "compaction.max-objects-per-block"), 6000000, "Maximum number of traces in a compacted block.")
	f.Uint64Var(&cfg.Compactor.MaxBlockBytes, util.PrefixConfig(prefix, "compaction.max-block-bytes"), 100*1024*1024*1024 /* 100GB */, "Maximum size of a compacted block.")
	f.DurationVar(&cfg.Compactor.MaxCompactionRange, util.PrefixConfig(prefix, "compaction.compaction-window"), time.Hour, "Maximum time window across which to compact blocks.")
	f.BoolVar(&cfg.Disabled, util.PrefixConfig(prefix, "disabled"), false, "Disable compaction.")
	cfg.OverrideRingKey = compactorRingKey
}

func toBasicLifecyclerConfig(cfg RingConfig, logger log.Logger) (ring.BasicLifecyclerConfig, error) {
	instanceAddr, err := ring.GetInstanceAddr(cfg.InstanceAddr, cfg.InstanceInterfaceNames, logger)
	if err != nil {
		return ring.BasicLifecyclerConfig{}, err
	}

	instancePort := ring.GetInstancePort(cfg.InstancePort, cfg.ListenPort)

	return ring.BasicLifecyclerConfig{
		ID:              cfg.InstanceID,
		Addr:            fmt.Sprintf("%s:%d", instanceAddr, instancePort),
		HeartbeatPeriod: cfg.HeartbeatPeriod,
		NumTokens:       ringNumTokens,
	}, nil
}
