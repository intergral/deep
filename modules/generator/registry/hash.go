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

package registry

import "github.com/cespare/xxhash/v2"

// separatorByte is a byte that cannot occur in valid UTF-8 sequences
var separatorByte = []byte{255}

// hashLabelValues generates a unique hash for the label values of a metric series. It expects that
// labelValues will always have the same length.
func hashLabelValues(labelValues []string) uint64 {
	h := xxhash.New()
	for _, v := range labelValues {
		_, _ = h.WriteString(v)
		_, _ = h.Write(separatorByte)
	}
	return h.Sum64()
}
