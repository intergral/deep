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
	"context"
	"github.com/weaveworks/common/user"
	"net/http"
)

const (
	// TenantIDHeaderName denotes the TenantID the request has been authenticated as
	TenantIDHeaderName = user.OrgIDHeaderName
)

// ExtractTenantID will extract the tenant ID from the context
func ExtractTenantID(ctx context.Context) (string, error) {
	// we wrap the get org ID, so we can keep a consistent naming in Deep. Everything should be tenantID, never orgID or userID.
	return user.ExtractOrgID(ctx)
}

// InjectTenantID will inject the tenant ID into the context
func InjectTenantID(ctx context.Context, tenantID string) context.Context {
	// we wrap the get org ID, so we can keep a consistent naming in Deep. Everything should be tenantID, never orgID or userID.
	return user.InjectOrgID(ctx, tenantID)
}

// InjectTenantIDIntoHTTPRequest will inject the tenant ID into the request
func InjectTenantIDIntoHTTPRequest(ctx context.Context, r *http.Request) error {
	// we wrap the get org ID, so we can keep a consistent naming in Deep. Everything should be tenantID, never orgID or userID.
	return user.InjectUserIDIntoHTTPRequest(ctx, r)
}
