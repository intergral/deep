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

package client

import (
	"flag"
	"github.com/grafana/dskit/grpcclient"
	"time"

	ring_client "github.com/grafana/dskit/ring/client"
)

type Config struct {
	PoolConfig       ring_client.PoolConfig `yaml:"pool_config,omitempty"`
	RemoteTimeout    time.Duration          `yaml:"remote_timeout,omitempty"`
	GRPCClientConfig grpcclient.Config      `yaml:"grpc_client_config"`
}

// RegisterFlags registers flags.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	cfg.GRPCClientConfig.RegisterFlagsWithPrefix("tracepoint.client", f)

	f.DurationVar(&cfg.PoolConfig.HealthCheckTimeout, "tracepoint.client.healthcheck-timeout", 1*time.Second, "Timeout for healthcheck rpcs.")
	f.DurationVar(&cfg.PoolConfig.CheckInterval, "tracepoint.client.healthcheck-interval", 15*time.Second, "Interval to healthcheck tracepoints")
	f.BoolVar(&cfg.PoolConfig.HealthCheckEnabled, "tracepoint.client.healthcheck-enabled", true, "Healthcheck tracepoints.")
	f.DurationVar(&cfg.RemoteTimeout, "tracepoint.client.timeout", 5*time.Second, "Timeout for tracepoint client RPCs.")
}
