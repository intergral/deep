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

package common

import (
	"bytes"
	"hash"
	"hash/fnv"
	"sort"
)

// This file contains types that need to be referenced by both the ./encoding and ./encoding/vX packages.
// It primarily exists here to break dependency loops.

// ID in deepdb
type ID []byte

type IDMapEntry[T any] struct {
	ID    ID
	Entry T
}

// IDMap is a helper for recording and checking for IDs. Not safe for concurrent use.
type IDMap[T any] struct {
	m map[uint64]IDMapEntry[T]
	h hash.Hash64
}

func NewIDMap[T any]() *IDMap[T] {
	return &IDMap[T]{
		m: map[uint64]IDMapEntry[T]{},
		h: fnv.New64(),
	}
}

// tokenForID returns a token for use in a hash map given a span id and span kind
// buffer must be a 4 byte slice and is reused for writing the span kind to the hashing function
// kind is used along with the actual id b/c in zipkin snapshots id is not guaranteed to be unique
// as it is shared between client and server spans.
func (m *IDMap[T]) tokenFor(id ID) uint64 {
	m.h.Reset()
	_, _ = m.h.Write(id)
	return m.h.Sum64()
}

func (m *IDMap[T]) Set(id ID, val T) {
	m.m[m.tokenFor(id)] = IDMapEntry[T]{id, val}
}

func (m *IDMap[T]) Has(id ID) bool {
	_, ok := m.m[m.tokenFor(id)]
	return ok
}

func (m *IDMap[T]) Get(id ID) (T, bool) {
	v, ok := m.m[m.tokenFor(id)]
	return v.Entry, ok
}

func (m *IDMap[T]) Len() int {
	return len(m.m)
}

func (m *IDMap[T]) EntriesSortedByID() []IDMapEntry[T] {
	// Copy and sort entries by ID
	entries := make([]IDMapEntry[T], 0, len(m.m))
	for _, e := range m.m {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].ID, entries[j].ID) == -1
	})

	return entries
}
