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

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBoundedWaitGroupExecutesCorrectNumberOfTimes(t *testing.T) {
	run := func(capacity uint, runs int) (executed int32) {
		executed = 0
		bg := New(capacity)
		for i := 0; i < runs; i++ {
			bg.Add(1)
			go func() {
				defer bg.Done()
				atomic.AddInt32(&executed, 1)
			}()
		}
		bg.Wait()
		return executed
	}

	assert.Equal(t, int32(100), run(5, 100)) // Capacity < runs
	assert.Equal(t, int32(5), run(100, 5))   // Capacity > runs
}

func TestBoundedWaitGroupDoesntExceedCapacity(t *testing.T) {
	m := sync.Mutex{}
	currExecuting := int32(0)
	maxExecuting := uint(0)

	capacity := uint(10)

	bg := New(capacity)

	for i := 0; i < 100; i++ {
		bg.Add(1)
		go func() {
			defer bg.Done()

			atomic.AddInt32(&currExecuting, 1)
			time.Sleep(10 * time.Millisecond) // Delay to ensure we get some executing in parallel

			m.Lock()
			curr := uint(atomic.LoadInt32(&currExecuting))
			if curr > maxExecuting {
				maxExecuting = curr
			}
			m.Unlock()

			atomic.AddInt32(&currExecuting, -1)
		}()
	}

	bg.Wait()

	assert.Equal(t, capacity, maxExecuting)
}

func TestBoundedWaitGroupPanics(t *testing.T) {
	assert.Panics(t, func() {
		New(0)
	})
}
