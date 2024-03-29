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

package tracepoint

import (
	"flag"
	"os"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/grpcclient"
	"github.com/intergral/deep/modules/tracepoint/api"
	"github.com/intergral/deep/modules/tracepoint/client"
	"github.com/intergral/deep/modules/tracepoint/worker"
	pkg_worker "github.com/intergral/deep/pkg/worker"

	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/ring"

	"github.com/intergral/deep/pkg/deepdb"
	"github.com/intergral/deep/pkg/util/log"
)

// Config for an ingester.
type Config struct {
	LifecyclerConfig ring.LifecyclerConfig `yaml:"lifecycler,omitempty"`

	MaxBlockDuration     time.Duration `yaml:"max_block_duration"`
	MaxBlockBytes        uint64        `yaml:"max_block_bytes"`
	CompleteBlockTimeout time.Duration `yaml:"complete_block_timeout"`
	OverrideRingKey      string        `yaml:"override_ring_key"`

	Client client.Config `yaml:"client"`

	API api.Config `yaml:"api"`
}

// RegisterFlagsAndApplyDefaults registers the flags.
func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {
	// apply generic defaults and then overlay deep default
	flagext.DefaultValues(&cfg.LifecyclerConfig)
	cfg.LifecyclerConfig.RingConfig.KVStore.Store = "memberlist"
	cfg.LifecyclerConfig.RingConfig.ReplicationFactor = 1
	cfg.LifecyclerConfig.RingConfig.HeartbeatTimeout = 5 * time.Minute

	f.DurationVar(&cfg.MaxBlockDuration, prefix+".max-block-duration", 30*time.Minute, "Maximum duration which the head block can be appended to before cutting it.")
	f.Uint64Var(&cfg.MaxBlockBytes, prefix+".max-block-bytes", 500*1024*1024, "Maximum size of the head block before cutting it.")
	f.DurationVar(&cfg.CompleteBlockTimeout, prefix+".complete-block-timeout", 3*deepdb.DefaultBlocklistPoll, "Duration to keep blocks in the ingester after they have been flushed.")

	hostname, err := os.Hostname()
	if err != nil {
		level.Error(log.Logger).Log("msg", "failed to get hostname", "err", err)
		os.Exit(1)
	}
	f.StringVar(&cfg.LifecyclerConfig.ID, prefix+".lifecycler.ID", hostname, "ID to register in the ring.")

	cfg.OverrideRingKey = tracepointRingKey

	flagext.DefaultValues(&cfg.Client)
	cfg.Client.GRPCClientConfig.GRPCCompression = "snappy"

	cfg.API.LoadTracepoint.Timeout = 1 * time.Minute
	cfg.API.Worker = worker.TPWorkerConfig{
		Config: pkg_worker.Config{
			MatchMaxConcurrency:   true,
			MaxConcurrentRequests: 20,
			Parallelism:           2,
			GRPCClientConfig: grpcclient.Config{
				MaxRecvMsgSize:  100 << 20,
				MaxSendMsgSize:  16 << 20,
				GRPCCompression: "gzip",
				BackoffConfig: backoff.Config{ // the max possible backoff should be lesser than QueryTimeout, with room for actual query response time
					MinBackoff: 100 * time.Millisecond,
					MaxBackoff: 1 * time.Second,
					MaxRetries: 5,
				},
			},
			DNSLookupPeriod: 10 * time.Second,
		},
	}
}
