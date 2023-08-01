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

package deepql

// MustExtractFetchSpansRequest parses the given traceql query and returns
// the storage layer conditions. Panics if the query fails to parse.
func MustExtractFetchSpansRequest(query string) FetchSnapshotRequest {
	c, err := ExtractFetchSpansRequest(query)
	if err != nil {
		panic(err)
	}
	return c
}

// ExtractFetchSpansRequest parses the given traceql query and returns
// the storage layer conditions. Returns an error if the query fails to parse.
func ExtractFetchSpansRequest(query string) (FetchSnapshotRequest, error) {
	ast, err := Parse(query)
	if err != nil {
		return FetchSnapshotRequest{}, err
	}

	req := FetchSnapshotRequest{
		AllConditions: true,
	}

	ast.Pipeline.extractConditions(&req)
	return req, nil
}
