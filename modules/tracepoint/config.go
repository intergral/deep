package tracepoint

import (
	"flag"
	"github.com/intergral/deep/modules/tracepoint/api"
	"github.com/intergral/deep/modules/tracepoint/client"
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

	MaxTraceIdle         time.Duration `yaml:"trace_idle_period"`
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

	f.DurationVar(&cfg.MaxTraceIdle, prefix+".trace-idle-period", 10*time.Second, "Duration after which to consider a trace complete if no spans have been received")
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
}
