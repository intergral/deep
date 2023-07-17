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

package app

import (
	"context"
	"net/http"

	"github.com/intergral/deep/pkg/util"
	"github.com/weaveworks/common/middleware"
	"google.golang.org/grpc"
)

var fakeHTTPAuthMiddleware = middleware.Func(func(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := util.InjectTenantID(r.Context(), util.FakeTenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
})

var fakeGRPCAuthUniaryMiddleware = func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	ctx = util.InjectTenantID(ctx, util.FakeTenantID)
	return handler(ctx, req)
}

var fakeGRPCAuthStreamMiddleware = func(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	ctx := util.InjectTenantID(ss.Context(), util.FakeTenantID)
	return handler(srv, serverStream{
		ctx:          ctx,
		ServerStream: ss,
	})
}

type serverStream struct {
	ctx context.Context
	grpc.ServerStream
}

func (ss serverStream) Context() context.Context {
	return ss.ctx
}
