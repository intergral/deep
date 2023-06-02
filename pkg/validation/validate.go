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

package validation

// ValidSnapshotID confirms that trace ids are 128 bits
func ValidSnapshotID(id []byte) bool {
	return len(id) == 16
}

// SmallestPositiveNonZeroIntPerTenant is returning the minimal positive and
// non-zero value of the supplied limit function for all given tenants. In many
// limits a value of 0 means unlimted so the method will return 0 only if all
// inputs have a limit of 0 or an empty tenant list is given.
func SmallestPositiveNonZeroIntPerTenant(tenantIDs []string, f func(string) int) int {
	var result *int
	for _, tenantID := range tenantIDs {
		v := f(tenantID)
		if v > 0 && (result == nil || v < *result) {
			result = &v
		}
	}
	if result == nil {
		return 0
	}
	return *result
}
