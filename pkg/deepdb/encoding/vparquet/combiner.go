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

package vparquet

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
)

// token is uint64 to reduce hash collision rates.  Experimentally, it was observed
// that fnv32 could approach a collision rate of 1 in 10,000. fnv64 avoids collisions
// when tested against traces with up to 1M spans (see matching test). A collision
// results in a dropped span during combine.
type token uint64

func newHash() hash.Hash64 {
	return fnv.New64()
}

// tokenForID returns a token for use in a hash map given a span id and span kind
// buffer must be a 4 byte slice and is reused for writing the span kind to the hashing function
// kind is used along with the actual id b/c in zipkin traces span id is not guaranteed to be unique
// as it is shared between client and server spans.
func tokenForID(h hash.Hash64, buffer []byte, kind int32, b []byte) token {
	binary.LittleEndian.PutUint32(buffer, uint32(kind))

	h.Reset()
	_, _ = h.Write(b)
	_, _ = h.Write(buffer)
	return token(h.Sum64())
}

func CombineTraces(traces ...*Snapshot) *Snapshot {
	if len(traces) == 1 {
		return traces[0]
	}

	c := NewCombiner()
	for i := 0; i < len(traces); i++ {
		c.ConsumeWithFinal(traces[i], i == len(traces)-1)
	}
	res, _ := c.Result()
	return res
}

// Combiner combines multiple partial traces into one, deduping spans based on
// ID and kind.  Note that it is destructive. There are design decisions for
// efficiency:
// * Only scan/hash the spans for each input once, which is reused across calls.
// * Only sort the final result once and if needed.
// * Don't scan/hash the spans for the last input (final=true).
type Combiner struct {
	result   *Snapshot
	spans    map[token]struct{}
	combined bool
}

func NewCombiner() *Combiner {
	return &Combiner{}
}

// Consume the given trace and destructively combines its contents.
func (c *Combiner) Consume(tr *Snapshot) (spanCount int) {
	return c.ConsumeWithFinal(tr, false)
}

// ConsumeWithFinal consumes the trace, but allows for performance savings when
// it is known that this is the last expected input trace.
func (c *Combiner) ConsumeWithFinal(tr *Snapshot, final bool) (spanCount int) {
	c.result = tr
	c.combined = true
	return
}

// Result returns the final trace and span count.
func (c *Combiner) Result() (*Snapshot, int) {

	return c.result, 1
}
