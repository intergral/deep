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
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsBackendSearch(t *testing.T) {
	assert.False(t, IsBackendSearch(httptest.NewRequest("GET", "/api/search", nil)))
	assert.False(t, IsBackendSearch(httptest.NewRequest("GET", "/api/search/?start=1", nil)))

	assert.True(t, IsBackendSearch(httptest.NewRequest("GET", "/api/search/?start=1&end=2", nil)))
	assert.True(t, IsBackendSearch(httptest.NewRequest("GET", "/api/search?start=1&end=2&tags=test", nil)))
	assert.True(t, IsBackendSearch(httptest.NewRequest("GET", "/api/search/?start=1&end=2&tags=test", nil)))
	assert.True(t, IsBackendSearch(httptest.NewRequest("GET", "/querier/api/search?start=1&end=2&tags=test", nil)))
	assert.True(t, IsBackendSearch(httptest.NewRequest("GET", "/querier/api/search/?start=1&end=2&tags=test", nil)))
}

func TestIsSearchBlock(t *testing.T) {
	assert.False(t, IsSearchBlock(httptest.NewRequest("GET", "/api/search", nil)))
	assert.False(t, IsSearchBlock(httptest.NewRequest("GET", "/api/search/?start=1", nil)))

	assert.True(t, IsSearchBlock(httptest.NewRequest("GET", "/api/search?blockID=blerg", nil)))
	assert.True(t, IsSearchBlock(httptest.NewRequest("GET", "/api/search/?blockID=blerg", nil)))
	assert.True(t, IsSearchBlock(httptest.NewRequest("GET", "/querier/api/search?blockID=blerg", nil)))
	assert.True(t, IsSearchBlock(httptest.NewRequest("GET", "/querier/api/search/?blockID=blerg", nil)))
}
