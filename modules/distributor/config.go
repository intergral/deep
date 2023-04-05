package distributor

import (
	"flag"
	"time"

	"github.com/grafana/dskit/flagext"
	ring_client "github.com/grafana/dskit/ring/client"

	"github.com/intergral/deep/modules/distributor/forwarder"
	"github.com/intergral/deep/pkg/util"
)

var defaultReceivers = map[string]interface{}{
	"deep": map[string]interface{}{
		"protocols": map[string]interface{}{
			"grpc": nil,
		},
	},
}

// Config for a Distributor.
type Config struct {
	// Distributors ring
	DistributorRing      RingConfig                 `yaml:"ring,omitempty"`
	Receivers            map[string]interface{}     `yaml:"receivers"`
	OverrideRingKey      string                     `yaml:"override_ring_key"`
	LogReceivedSnapshots LogReceivedSnapshotsConfig `yaml:"log_received_snapshots"`

	Forwarders forwarder.ConfigList `yaml:"forwarders"`

	// disables write extension with inactive ingesters. Use this along with ingester.lifecycler.unregister_on_shutdown = true
	//  note that setting these two config values reduces tolerance to failures on rollout b/c there is always one guaranteed to be failing replica
	ExtendWrites bool `yaml:"extend_writes"`

	// For testing.
	factory func(addr string) (ring_client.PoolClient, error) `yaml:"-"`
}

type LogReceivedSnapshotsConfig struct {
	Enabled              bool `yaml:"enabled"`
	IncludeAllAttributes bool `yaml:"include_all_attributes"`
}

// RegisterFlagsAndApplyDefaults registers flags and applies defaults
func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {
	flagext.DefaultValues(&cfg.DistributorRing)
	cfg.DistributorRing.KVStore.Store = "memberlist"
	cfg.DistributorRing.HeartbeatTimeout = 5 * time.Minute

	cfg.OverrideRingKey = distributorRingKey
	cfg.ExtendWrites = true

	f.BoolVar(&cfg.LogReceivedSnapshots.Enabled, util.PrefixConfig(prefix, "log-received-snapshots.enabled"), false, "Enable to log every received snapshot to help debug ingestion using the logs.")
	f.BoolVar(&cfg.LogReceivedSnapshots.IncludeAllAttributes, util.PrefixConfig(prefix, "log-received-snapshots.include-attributes"), false, "Enable to include snapshot attributes in the logs.")
}
