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

package worker

import (
	"flag"
	"time"

	"github.com/go-kit/log"
	pkg_worker "github.com/intergral/deep/pkg/worker"
	"github.com/pkg/errors"
)

type TPWorkerConfig struct {
	pkg_worker.Config `yaml:",inline"`
}

func (cfg *TPWorkerConfig) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&cfg.FrontendAddress, "tracepoint.frontend-address", "", "Address of query frontend service, in host:port format. If -tracepoint.scheduler-address is set as well, tracepoint will use scheduler instead. Only one of -tracepoint.frontend-address or -tracepoint.scheduler-address can be set. If neither is set, queries are only received via HTTP endpoint.")

	f.DurationVar(&cfg.DNSLookupPeriod, "tracepoint.dns-lookup-period", 10*time.Second, "How often to query DNS for query-frontend or query-scheduler address.")

	f.IntVar(&cfg.Parallelism, "tracepoint.worker-parallelism", 10, "Number of simultaneous queries to process per query-frontend or query-scheduler.")
	f.BoolVar(&cfg.MatchMaxConcurrency, "tracepoint.worker-match-max-concurrent", false, "Force worker concurrency to match the -tracepoint.max-concurrent option. Overrides tracepoint.worker-parallelism.")
	f.StringVar(&cfg.QuerierID, "tracepoint.id", "", "Querier ID, sent to frontend service to identify requests from the same tracepoint. Defaults to hostname.")

	cfg.GRPCClientConfig.RegisterFlagsWithPrefix("tracepoint.frontend-client", f)
}

func (cfg *TPWorkerConfig) Validate(log log.Logger) error {
	if cfg.FrontendAddress != "" {
		return errors.New("starting tracepoint api worker without frontend address is not supported")
	}
	return cfg.GRPCClientConfig.Validate(log)
}
