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

package querier

import (
	"flag"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/grpcclient"
	"github.com/intergral/deep/modules/querier/worker"
	pkg_wrk "github.com/intergral/deep/pkg/worker"
)

// Config for a querier.
type Config struct {
	Search             SearchConfig       `yaml:"search"`
	SnapshotByIDConfig SnapshotByIDConfig `yaml:"snapshot_by_id"`

	ExtraQueryDelay        time.Duration `yaml:"extra_query_delay,omitempty"`
	MaxConcurrentQueries   int           `yaml:"max_concurrent_queries"`
	Worker                 worker.Config `yaml:"frontend_worker"`
	QueryRelevantIngesters bool          `yaml:"query_relevant_ingesters"`
}

type SearchConfig struct {
	QueryTimeout      time.Duration `yaml:"query_timeout"`
	PreferSelf        int           `yaml:"prefer_self"`
	ExternalEndpoints []string      `yaml:"external_endpoints"`
	HedgeRequestsAt   time.Duration `yaml:"external_hedge_requests_at"`
	HedgeRequestsUpTo int           `yaml:"external_hedge_requests_up_to"`
}

type SnapshotByIDConfig struct {
	QueryTimeout time.Duration `yaml:"query_timeout"`
}

// RegisterFlagsAndApplyDefaults register flags.
func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {
	cfg.SnapshotByIDConfig.QueryTimeout = 10 * time.Second
	cfg.QueryRelevantIngesters = false
	cfg.ExtraQueryDelay = 0
	cfg.MaxConcurrentQueries = 20
	cfg.Search.PreferSelf = 10
	cfg.Search.HedgeRequestsAt = 8 * time.Second
	cfg.Search.HedgeRequestsUpTo = 2
	cfg.Search.QueryTimeout = 30 * time.Second
	cfg.Worker = worker.Config{
		Config: pkg_wrk.Config{
			MatchMaxConcurrency:   true,
			MaxConcurrentRequests: cfg.MaxConcurrentQueries,
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
		}}

	f.StringVar(&cfg.Worker.FrontendAddress, prefix+".frontend-address", "", "Address of query frontend service, in host:port format.")
}
