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

package util

import (
	"sort"
	"strings"
)

type DistinctStringCollector struct {
	values   map[string]struct{}
	maxLen   int
	currLen  int
	totalLen int
}

// NewDistinctStringCollector with the given maximum data size. This is calculated
// as the total length of the recorded strings. For ease of use, maximum=0
// is interpreted as unlimited.
func NewDistinctStringCollector(maxDataSize int) *DistinctStringCollector {
	return &DistinctStringCollector{
		values: make(map[string]struct{}),
		maxLen: maxDataSize,
	}
}

func (d *DistinctStringCollector) Collect(s string) {
	if _, ok := d.values[s]; ok {
		// Already present
		return
	}

	// New entry
	d.totalLen += len(s)

	// Can it fit?
	if d.maxLen > 0 && d.currLen+len(s) > d.maxLen {
		// No
		return
	}

	// Clone instead of referencing original
	s = strings.Clone(s)

	d.values[s] = struct{}{}
	d.currLen += len(s)
}

// Strings returns the final list of distinct values collected and sorted.
func (d *DistinctStringCollector) Strings() []string {
	ss := make([]string, 0, len(d.values))

	for k := range d.values {
		ss = append(ss, k)
	}

	sort.Strings(ss)
	return ss
}

// Exceeded indicates if some values were lost because the maximum size limit was met.
func (d *DistinctStringCollector) Exceeded() bool {
	return d.totalLen > d.currLen
}

// TotalDataSize is the total size of all distinct strings encountered.
func (d *DistinctStringCollector) TotalDataSize() int {
	return d.totalLen
}
