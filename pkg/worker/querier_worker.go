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
	"context"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/services"
	"github.com/intergral/deep/pkg/util"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"os"
	"sync"
)

type querierWorker struct {
	*services.BasicService

	cfg Config
	log log.Logger

	processor Processor

	subservices *services.Manager

	mu sync.Mutex
	// Set to nil when stop is called... no more managers are created afterwards.
	managers map[string]*ProcessorManager
}

func NewQuerierWorker(cfg Config, handler RequestHandler, log log.Logger, reg prometheus.Registerer, processFunction ProcessorFunction) (services.Service, error) {
	if cfg.QuerierID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get hostname for configuring querier ID")
		}
		cfg.QuerierID = hostname
	}

	var processor Processor
	var servs []services.Service
	var address string

	level.Info(log).Log("msg", "Starting querier worker connected to query-frontend", "frontend", cfg.FrontendAddress)

	address = cfg.FrontendAddress
	processor = NewFrontendProcessor(cfg, handler, log, processFunction)

	return newQuerierWorkerWithProcessor(cfg, log, processor, address, servs)
}

func newQuerierWorkerWithProcessor(cfg Config, log log.Logger, processor Processor, address string, servs []services.Service) (*querierWorker, error) {
	f := &querierWorker{
		cfg:       cfg,
		log:       log,
		managers:  map[string]*ProcessorManager{},
		processor: processor,
	}

	// Empty address is only used in tests, where individual targets are added manually.
	if address != "" {
		w, err := util.NewDNSWatcher(address, cfg.DNSLookupPeriod, f)
		if err != nil {
			return nil, err
		}

		servs = append(servs, w)
	}

	if len(servs) > 0 {
		subservices, err := services.NewManager(servs...)
		if err != nil {
			return nil, errors.Wrap(err, "querier worker subservices")
		}

		f.subservices = subservices
	}

	f.BasicService = services.NewIdleService(f.starting, f.stopping)
	return f, nil
}

func (w *querierWorker) starting(ctx context.Context) error {
	if w.subservices == nil {
		return nil
	}
	return services.StartManagerAndAwaitHealthy(ctx, w.subservices)
}

func (w *querierWorker) stopping(_ error) error {
	// Stop all goroutines fetching queries. Note that in Stopping state,
	// worker no longer creates new managers in AddressAdded method.
	w.mu.Lock()
	for _, m := range w.managers {
		m.Stop()
	}
	w.mu.Unlock()

	if w.subservices == nil {
		return nil
	}

	// Stop DNS watcher and services used by processor.
	return services.StopManagerAndAwaitStopped(context.Background(), w.subservices)
}

func (w *querierWorker) AddressAdded(address string) {
	ctx := w.ServiceContext()
	if ctx == nil || ctx.Err() != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if m := w.managers[address]; m != nil {
		return
	}

	level.Info(w.log).Log("msg", "adding connection", "addr", address)
	conn, err := w.connect(context.Background(), address)
	if err != nil {
		level.Error(w.log).Log("msg", "error connecting", "addr", address, "err", err)
		return
	}

	w.managers[address] = NewProcessorManager(ctx, w.processor, conn, address)
	// Called with lock.
	w.resetConcurrency()
}

func (w *querierWorker) AddressRemoved(address string) {
	level.Info(w.log).Log("msg", "removing connection", "addr", address)

	w.mu.Lock()
	p := w.managers[address]
	delete(w.managers, address)
	// Called with lock.
	w.resetConcurrency()
	w.mu.Unlock()

	if p != nil {
		p.Stop()
	}
}

// Must be called with lock.
func (w *querierWorker) resetConcurrency() {
	totalConcurrency := 0
	index := 0

	for _, m := range w.managers {
		concurrency := 0

		if w.cfg.MatchMaxConcurrency {
			concurrency = w.cfg.MaxConcurrentRequests / len(w.managers)

			// If max concurrency does not evenly divide into our frontends a subset will be chosen
			// to receive an extra connection.  Frontend addresses were shuffled above so this will be a
			// random selection of frontends.
			if index < w.cfg.MaxConcurrentRequests%len(w.managers) {
				level.Warn(w.log).Log("msg", "max concurrency is not evenly divisible across targets, adding an extra connection", "addr", m.address)
				concurrency++
			}
		} else {
			concurrency = w.cfg.Parallelism
		}

		// If concurrency is 0 then MaxConcurrentRequests is less than the total number of
		// frontends/schedulers. In order to prevent accidentally starving a frontend or scheduler we are just going to
		// always connect once to every target.  This is dangerous b/c we may start exceeding PromQL
		// max concurrency.
		if concurrency == 0 {
			concurrency = 1
		}

		totalConcurrency += concurrency
		m.Concurrency(concurrency)
		index++
	}

	if totalConcurrency > w.cfg.MaxConcurrentRequests {
		level.Warn(w.log).Log("msg", "total worker concurrency is greater than promql max concurrency. Queries may be queued in the querier which reduces QOS")
	}
}

func (w *querierWorker) connect(ctx context.Context, address string) (*grpc.ClientConn, error) {
	// Because we only use single long-running method, it doesn't make sense to inject user ID, send over tracing or add metrics.
	opts, err := w.cfg.GRPCClientConfig.DialOption(nil, nil)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.DialContext(ctx, address, opts...)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
