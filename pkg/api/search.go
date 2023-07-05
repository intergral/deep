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

package api

import (
	"github.com/intergral/deep/pkg/deeppb"
	"net/http"
)

// IsBackendSearch returns true if the request has a start, end and tags parameter and is the /api/search path
func IsBackendSearch(r *http.Request) bool {
	q := r.URL.Query()
	return q.Get(urlParamStart) != "" && q.Get(urlParamEnd) != ""
}

// IsSearchBlock returns true if the request appears to be for backend blocks. It is not exhaustive
// and only looks for blockID
func IsSearchBlock(r *http.Request) bool {
	q := r.URL.Query()

	return q.Get(urlParamBlockID) != ""
}

// IsDeepQLQuery returns true if the request contains a deepQL query.
func IsDeepQLQuery(r *deeppb.SearchRequest) bool {
	return len(r.Query) > 0
}
