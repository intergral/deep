/*
 * Copyright (C) 2024  Intergral GmbH
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

	"golang.org/x/exp/slices"
)

func Remove[S ~[]E, E any](array S, index int) S {
	return slices.Delete(array, index, index+1)
}

func RemoveAll[S ~[]E, E any](slice S, idxs ...int) S {
	if len(idxs) == 1 {
		return Remove(slice, idxs[0])
	}

	// Sort indices in descending order
	sort.Sort(sort.Reverse(sort.IntSlice(idxs)))

	for _, index := range idxs {
		slice = append(slice[:index], slice[index+1:]...)
	}

	return slice
}
