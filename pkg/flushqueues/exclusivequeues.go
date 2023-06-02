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

package flushqueues

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/uber-go/atomic"
)

type ExclusiveQueues struct {
	queues     []*PriorityQueue
	index      *atomic.Int32
	activeKeys sync.Map
	stopped    atomic.Bool
}

// New creates a new set of flush queues with a prom gauge to track current depth
func New(queues int, metric prometheus.Gauge) *ExclusiveQueues {
	f := &ExclusiveQueues{
		queues: make([]*PriorityQueue, queues),
		index:  atomic.NewInt32(0),
	}

	for j := 0; j < queues; j++ {
		f.queues[j] = NewPriorityQueue(metric)
	}

	return f
}

// Enqueue adds the op to the next queue and prevents any other items to be added with this key
func (f *ExclusiveQueues) Enqueue(op Op) error {
	_, ok := f.activeKeys.Load(op.Key())
	if ok {
		return nil
	}

	f.activeKeys.Store(op.Key(), struct{}{})
	return f.Requeue(op)
}

// Dequeue removes the next op from the requested queue.  After dequeueing the calling
// process either needs to call ClearKey or Requeue
func (f *ExclusiveQueues) Dequeue(q int) Op {
	return f.queues[q].Dequeue()
}

// Requeue adds an op that is presumed to already be covered by activeKeys
func (f *ExclusiveQueues) Requeue(op Op) error {
	flushQueueIndex := int(f.index.Inc()) % len(f.queues)
	_, err := f.queues[flushQueueIndex].Enqueue(op)
	return err
}

// Clear unblocks the requested op.  This should be called only after a flush has been successful
func (f *ExclusiveQueues) Clear(op Op) {
	f.activeKeys.Delete(op.Key())
}

func (f *ExclusiveQueues) IsEmpty() bool {
	length := 0

	f.activeKeys.Range(func(_, _ interface{}) bool {
		length++
		return false
	})

	return length <= 0
}

// Stop closes all queues
func (f *ExclusiveQueues) Stop() {
	f.stopped.Store(true)

	for _, q := range f.queues {
		q.Close()
	}
}

func (f *ExclusiveQueues) IsStopped() bool {
	return f.stopped.Load()
}
