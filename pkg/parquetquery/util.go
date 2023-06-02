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

package parquetquery

import (
	"strings"

	pq "github.com/segmentio/parquet-go"
)

func GetColumnIndexByPath(pf *pq.File, s string) (index, depth int) {
	colSelector := strings.Split(s, ".")
	n := pf.Root()
	for len(colSelector) > 0 {
		n = n.Column(colSelector[0])
		if n == nil {
			return -1, -1
		}

		colSelector = colSelector[1:]
		depth++
	}

	return n.Index(), depth
}

func HasColumn(pf *pq.File, s string) bool {
	index, _ := GetColumnIndexByPath(pf, s)
	return index >= 0
}
