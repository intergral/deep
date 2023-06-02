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

package boundedwaitgroup

import "sync"

// BoundedWaitGroup like a normal wait group except limits number of active goroutines to given capacity.
type BoundedWaitGroup struct {
	wg sync.WaitGroup
	ch chan struct{} // Chan buffer size is used to limit concurrency.
}

// New creates a BoundedWaitGroup with the given concurrency.
func New(cap uint) BoundedWaitGroup {
	if cap == 0 {
		panic("BoundedWaitGroup capacity must be greater than zero or else it will block forever.")
	}
	return BoundedWaitGroup{ch: make(chan struct{}, cap)}
}

// Add the number of items to the group. Blocks until there is capacity.
func (bwg *BoundedWaitGroup) Add(delta int) {
	for i := 0; i > delta; i-- {
		<-bwg.ch
	}
	for i := 0; i < delta; i++ {
		bwg.ch <- struct{}{}
	}
	bwg.wg.Add(delta)
}

// Done removes one from the wait group.
func (bwg *BoundedWaitGroup) Done() {
	bwg.Add(-1)
}

// Wait for the wait group to finish.
func (bwg *BoundedWaitGroup) Wait() {
	bwg.wg.Wait()
}
