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
	"hash/fnv"
)

// TokenFor generates a token used for finding ingesters from ring
func TokenFor(userID string, b []byte) uint32 {
	h := fnv.New32()
	_, _ = h.Write([]byte(userID))
	_, _ = h.Write(b)
	return h.Sum32()
}

// TokenForSnapshotID generates a hashed value for a trace id
func TokenForSnapshotID(b []byte) uint32 {
	h := fnv.New32()
	_, _ = h.Write(b)
	return h.Sum32()
}
