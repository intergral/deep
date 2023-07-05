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

package ingester

import (
	"flag"
	"os"
	"time"

	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/ring"

	"github.com/intergral/deep/pkg/deepdb"
	"github.com/intergral/deep/pkg/util/log"
)

// Config for an ingester.
type Config struct {
	LifecyclerConfig ring.LifecyclerConfig `yaml:"lifecycler,omitempty"`

	ConcurrentFlushes    int           `yaml:"concurrent_flushes"`
	FlushCheckPeriod     time.Duration `yaml:"flush_check_period"`
	FlushOpTimeout       time.Duration `yaml:"flush_op_timeout"`
	MaxSnapshotIdle      time.Duration `yaml:"snapshot_idle_period"`
	MaxBlockDuration     time.Duration `yaml:"max_block_duration"`
	MaxBlockBytes        uint64        `yaml:"max_block_bytes"`
	CompleteBlockTimeout time.Duration `yaml:"complete_block_timeout"`
	OverrideRingKey      string        `yaml:"override_ring_key"`
}

// RegisterFlagsAndApplyDefaults registers the flags.
func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {
	// apply generic defaults and then overlay deep default
	flagext.DefaultValues(&cfg.LifecyclerConfig)
	cfg.LifecyclerConfig.RingConfig.KVStore.Store = "memberlist"
	cfg.LifecyclerConfig.RingConfig.ReplicationFactor = 1
	cfg.LifecyclerConfig.RingConfig.HeartbeatTimeout = 5 * time.Minute

	cfg.ConcurrentFlushes = 4
	cfg.FlushCheckPeriod = 10 * time.Second
	cfg.FlushOpTimeout = 5 * time.Minute

	f.DurationVar(&cfg.MaxSnapshotIdle, prefix+".snapshot-idle-period", 10*time.Second, "Duration after which to consider a snapshot complete.")
	f.DurationVar(&cfg.MaxBlockDuration, prefix+".max-block-duration", 30*time.Minute, "Maximum duration which the head block can be appended to before cutting it.")
	f.Uint64Var(&cfg.MaxBlockBytes, prefix+".max-block-bytes", 500*1024*1024, "Maximum size of the head block before cutting it.")
	f.DurationVar(&cfg.CompleteBlockTimeout, prefix+".complete-block-timeout", 3*deepdb.DefaultBlocklistPoll, "Duration to keep blocks in the ingester after they have been flushed.")

	hostname, err := os.Hostname()
	if err != nil {
		level.Error(log.Logger).Log("msg", "failed to get hostname", "err", err)
		os.Exit(1)
	}
	f.StringVar(&cfg.LifecyclerConfig.ID, prefix+".lifecycler.ID", hostname, "ID to register in the ring.")

	cfg.OverrideRingKey = ingesterRingKey
}
