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

package math

// Max returns the maximum of two ints
func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Min returns the minimum of two ints
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Max64 returns the maximum of two int64s
func Max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Min64 returns the minimum of two int64s
func Min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
