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
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type simpleItem int64

func (i simpleItem) Priority() int64 {
	return int64(i)
}

func (i simpleItem) Key() string {
	return strconv.FormatInt(int64(i), 10)
}

func TestPriorityQueueBasic(t *testing.T) {
	queue := NewPriorityQueue(nil)
	assert.Equal(t, 0, queue.Length(), "Expected length = 0")

	_, err := queue.Enqueue(simpleItem(1))
	assert.NoError(t, err)
	assert.Equal(t, 1, queue.Length(), "Expected length = 1")

	i, ok := queue.Dequeue().(simpleItem)
	assert.True(t, ok, "Expected cast to succeed")
	assert.Equal(t, simpleItem(1), i, "Expected to dequeue simpleItem(1)")

	queue.Close()
	assert.Nil(t, queue.Dequeue(), "Expect nil dequeue")
}

func TestPriorityQueuePriorities(t *testing.T) {
	queue := NewPriorityQueue(nil)

	_, err := queue.Enqueue(simpleItem(1))
	assert.NoError(t, err)

	_, err = queue.Enqueue(simpleItem(2))
	assert.NoError(t, err)

	assert.Equal(t, simpleItem(2), queue.Dequeue().(simpleItem), "Expected to dequeue simpleItem(2)")
	assert.Equal(t, simpleItem(1), queue.Dequeue().(simpleItem), "Expected to dequeue simpleItem(1)")

	queue.Close()
	assert.Nil(t, queue.Dequeue(), "Expect nil dequeue")
}

func TestPriorityQueuePriorities2(t *testing.T) {
	queue := NewPriorityQueue(nil)

	_, err := queue.Enqueue(simpleItem(2))
	assert.NoError(t, err)

	_, err = queue.Enqueue(simpleItem(1))
	assert.NoError(t, err)

	assert.Equal(t, simpleItem(2), queue.Dequeue().(simpleItem), "Expected to dequeue simpleItem(2)")
	assert.Equal(t, simpleItem(1), queue.Dequeue().(simpleItem), "Expected to dequeue simpleItem(1)")

	queue.Close()
	assert.Nil(t, queue.Dequeue(), "Expect nil dequeue")
}

func TestPriorityQueueWait(t *testing.T) {
	queue := NewPriorityQueue(nil)

	done := make(chan struct{})
	go func() {
		assert.Nil(t, queue.Dequeue(), "Expect nil dequeue")
		close(done)
	}()

	queue.Close()
	runtime.Gosched()
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Close didn't unblock Dequeue.")
	}
}
