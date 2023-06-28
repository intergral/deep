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

package snapshotreceiver

import (
	"context"
	"github.com/intergral/deep/pkg/receivers/config/client"
	receivers "github.com/intergral/deep/pkg/receivers/types"
	"github.com/intergral/deep/pkg/util"
	pb "github.com/intergral/go-deep-proto/poll/v1"
	tp "github.com/intergral/go-deep-proto/tracepoint/v1"
	"github.com/weaveworks/common/user"

	"github.com/intergral/deep/pkg/util/log"
)

// Middleware is used to intercept GRPC and HTTP calls from receivers and ensure
// that the x-scope-orgid header is available
type Middleware interface {
	WrapPoll(poll receivers.ProcessPoll) receivers.ProcessPoll
	WrapSnapshots(snap receivers.ProcessSnapshots) receivers.ProcessSnapshots
}

// fakeTenantMiddleware creates a middleware that puts all requests into util.FakeTenantID scope.
type fakeTenantMiddleware struct{}

func FakeTenantMiddleware() Middleware {
	return &fakeTenantMiddleware{}
}

func (m *fakeTenantMiddleware) WrapPoll(poll receivers.ProcessPoll) receivers.ProcessPoll {
	return func(ctx context.Context, pollRequest *pb.PollRequest) (*pb.PollResponse, error) {
		ctx = user.InjectOrgID(ctx, util.FakeTenantID)
		return poll(ctx, pollRequest)
	}
}

func (m *fakeTenantMiddleware) WrapSnapshots(snap receivers.ProcessSnapshots) receivers.ProcessSnapshots {
	return func(ctx context.Context, in *tp.Snapshot) (*tp.SnapshotResponse, error) {
		ctx = user.InjectOrgID(ctx, util.FakeTenantID)
		return snap(ctx, in)
	}
}

// multiTenancyMiddleware is the main middleware that will look for the user.OrgIDHeaderName header
// and attach it to the context
type multiTenancyMiddleware struct{}

func MultiTenancyMiddleware() Middleware {
	return &multiTenancyMiddleware{}
}

func (m *multiTenancyMiddleware) WrapPoll(poll receivers.ProcessPoll) receivers.ProcessPoll {
	return func(ctx context.Context, pollRequest *pb.PollRequest) (*pb.PollResponse, error) {
		idCtx, err := m.findAttachOrgId(ctx)
		if err != nil {
			return nil, err
		}
		return poll(idCtx, pollRequest)
	}
}

func (m *multiTenancyMiddleware) WrapSnapshots(snap receivers.ProcessSnapshots) receivers.ProcessSnapshots {
	return func(ctx context.Context, in *tp.Snapshot) (*tp.SnapshotResponse, error) {
		idCtx, err := m.findAttachOrgId(ctx)
		if err != nil {
			return nil, err
		}
		return snap(idCtx, in)
	}
}

func (m *multiTenancyMiddleware) findAttachOrgId(ctx context.Context) (context.Context, error) {
	var err error
	_, ctx, err = user.ExtractFromGRPCRequest(ctx)
	if err != nil {
		// Maybe its a HTTP request.
		info := client.FromContext(ctx)
		orgIDs := info.Metadata.Get(user.OrgIDHeaderName)
		if len(orgIDs) == 0 {
			log.Logger.Log("msg", "failed to extract org id from both grpc and HTTP", "err", err)
			return ctx, err
		}

		if len(orgIDs) > 1 {
			log.Logger.Log("msg", "more than one orgID found", "orgIDs", orgIDs)
			return ctx, err
		}

		ctx = user.InjectOrgID(ctx, orgIDs[0])
		return ctx, nil
	}
	return ctx, err
}
