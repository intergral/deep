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
	"sync"
	"time"

	"go.uber.org/atomic"
	"google.golang.org/grpc"
)

const (
	notifyShutdownTimeout = 5 * time.Second
)

// ProcessorManager Manages processor goroutines for single grpc connection.
type ProcessorManager struct {
	p       Processor
	conn    *grpc.ClientConn
	address string

	// Main context to control all goroutines.
	ctx context.Context
	wg  sync.WaitGroup

	// Cancel functions for individual goroutines.
	cancelsMu sync.Mutex
	cancels   []context.CancelFunc

	currentProcessors *atomic.Int32
}

func NewProcessorManager(ctx context.Context, p Processor, conn *grpc.ClientConn, address string) *ProcessorManager {
	return &ProcessorManager{
		p:                 p,
		ctx:               ctx,
		conn:              conn,
		address:           address,
		currentProcessors: atomic.NewInt32(0),
	}
}

func (pm *ProcessorManager) Stop() {
	// Notify the remote query-frontend or query-scheduler we're shutting down.
	// We use a new context to make sure it's not cancelled.
	notifyCtx, cancel := context.WithTimeout(context.Background(), notifyShutdownTimeout)
	defer cancel()
	pm.p.NotifyShutdown(notifyCtx, pm.conn, pm.address)

	// Stop all goroutines.
	pm.Concurrency(0)

	// Wait until they finish.
	pm.wg.Wait()

	_ = pm.conn.Close()
}

func (pm *ProcessorManager) Concurrency(n int) {
	pm.cancelsMu.Lock()
	defer pm.cancelsMu.Unlock()

	if n < 0 {
		n = 0
	}

	for len(pm.cancels) < n {
		ctx, cancel := context.WithCancel(pm.ctx)
		pm.cancels = append(pm.cancels, cancel)

		pm.wg.Add(1)
		go func() {
			defer pm.wg.Done()

			pm.currentProcessors.Inc()
			defer pm.currentProcessors.Dec()

			pm.p.ProcessQueriesOnSingleStream(ctx, pm.conn, pm.address)
		}()
	}

	for len(pm.cancels) > n {
		pm.cancels[0]()
		pm.cancels = pm.cancels[1:]
	}
}
