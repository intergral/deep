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

package generator

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/dskit/kv"
	"github.com/grafana/dskit/ring"

	"github.com/intergral/deep/pkg/util/log"
)

type RingConfig struct {
	KVStore          kv.Config     `yaml:"kvstore"`
	HeartbeatPeriod  time.Duration `yaml:"heartbeat_period"`
	HeartbeatTimeout time.Duration `yaml:"heartbeat_timeout"`

	InstanceID             string   `yaml:"instance_id"`
	InstanceInterfaceNames []string `yaml:"instance_interface_names"`
	InstanceAddr           string   `yaml:"instance_addr"`

	// Injected internally
	ListenPort int `yaml:"-"`
}

func (cfg *RingConfig) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {
	cfg.KVStore.RegisterFlagsWithPrefix(prefix, "collectors/", f)
	cfg.KVStore.Store = "memberlist"

	cfg.HeartbeatPeriod = 5 * time.Second
	cfg.HeartbeatTimeout = 1 * time.Minute

	hostname, err := os.Hostname()
	if err != nil {
		level.Error(log.Logger).Log("msg", "failed to get hostname", "err", err)
		os.Exit(1)
	}
	cfg.InstanceID = hostname
	cfg.InstanceInterfaceNames = []string{"eth0", "en0", "enp0s3"}
}

func (cfg *RingConfig) ToRingConfig() ring.Config {
	rc := ring.Config{}
	flagext.DefaultValues(&rc)

	rc.KVStore = cfg.KVStore
	rc.HeartbeatTimeout = cfg.HeartbeatTimeout
	rc.ReplicationFactor = 1
	rc.SubringCacheDisabled = true

	return rc
}

func (cfg *RingConfig) toLifecyclerConfig() (ring.BasicLifecyclerConfig, error) {
	instanceAddr, err := ring.GetInstanceAddr(cfg.InstanceAddr, cfg.InstanceInterfaceNames, log.Logger)
	if err != nil {
		level.Error(log.Logger).Log("msg", "failed to get instance address", "err", err)
		return ring.BasicLifecyclerConfig{}, err
	}

	instancePort := cfg.ListenPort

	return ring.BasicLifecyclerConfig{
		ID:              cfg.InstanceID,
		Addr:            fmt.Sprintf("%s:%d", instanceAddr, instancePort),
		HeartbeatPeriod: cfg.HeartbeatPeriod,
		NumTokens:       ringNumTokens,
	}, nil
}
