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
	"io"
	"time"

	"github.com/grafana/dskit/grpcclient"
	ring_client "github.com/grafana/dskit/ring/client"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/opentracing/opentracing-go"
	"github.com/weaveworks/common/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/intergral/deep/pkg/deeppb"
)

// Config for a generator client.
type Config struct {
	PoolConfig       ring_client.PoolConfig `yaml:"pool_config,omitempty"`
	RemoteTimeout    time.Duration          `yaml:"remote_timeout,omitempty"`
	GRPCClientConfig grpcclient.Config      `yaml:"grpc_client_config"`
}

type Client struct {
	deeppb.MetricsGeneratorClient
	grpc_health_v1.HealthClient
	io.Closer
}

// RegisterFlags registers flags.
func (cfg *Config) RegisterFlags(f *flag.FlagSet) {
	cfg.GRPCClientConfig.RegisterFlagsWithPrefix("generator.client", f)

	f.DurationVar(&cfg.PoolConfig.HealthCheckTimeout, "generator.client.healthcheck-timeout", 1*time.Second, "Timeout for healthcheck rpcs.")
	f.DurationVar(&cfg.PoolConfig.CheckInterval, "generator.client.healthcheck-interval", 15*time.Second, "Interval to healthcheck generators")
	f.BoolVar(&cfg.PoolConfig.HealthCheckEnabled, "generator.client.healthcheck-enabled", true, "Healthcheck generators.")
	f.DurationVar(&cfg.RemoteTimeout, "generator.client.timeout", 5*time.Second, "Timeout for generator client RPCs.")
}

// New returns a new generator client.
func New(addr string, cfg Config) (*Client, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	instrumentationOpts, err := cfg.GRPCClientConfig.DialOption(instrumentation())
	if err != nil {
		return nil, err
	}

	opts = append(opts, instrumentationOpts...)
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		MetricsGeneratorClient: deeppb.NewMetricsGeneratorClient(conn),
		HealthClient:           grpc_health_v1.NewHealthClient(conn),
		Closer:                 conn,
	}, nil
}

func instrumentation() ([]grpc.UnaryClientInterceptor, []grpc.StreamClientInterceptor) {
	return []grpc.UnaryClientInterceptor{
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
			middleware.ClientUserHeaderInterceptor,
		}, []grpc.StreamClientInterceptor{
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer()),
			middleware.StreamClientUserHeaderInterceptor,
		}
}
