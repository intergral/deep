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
	"testing"

	"github.com/intergral/deep/pkg/receivers/config/client"
	receivers "github.com/intergral/deep/pkg/receivers/types"
	tp "github.com/intergral/go-deep-proto/tracepoint/v1"
	"google.golang.org/grpc/metadata"

	"github.com/intergral/deep/pkg/util"
	"github.com/stretchr/testify/require"
)

type assertFunc func(*testing.T, context.Context)

type testConsumer struct {
	t          *testing.T
	assertFunc assertFunc
}

func newAssertingConsumer(t *testing.T, assertFunc assertFunc) receivers.ProcessSnapshots {
	return testConsumer{
		t:          t,
		assertFunc: assertFunc,
	}.ConsumeSnapshots
}

func (tc testConsumer) ConsumeSnapshots(ctx context.Context, _ *tp.Snapshot) (*tp.SnapshotResponse, error) {
	tc.assertFunc(tc.t, ctx)
	return nil, nil
}

func TestFakeTenantMiddleware(t *testing.T) {
	m := FakeTenantMiddleware()

	t.Run("injects tenant id", func(t *testing.T) {
		consumer := newAssertingConsumer(t, func(t *testing.T, ctx context.Context) {
			tenantID, err := util.ExtractTenantID(ctx)
			require.NoError(t, err)
			require.Equal(t, tenantID, util.FakeTenantID)
		})

		ctx := context.Background()
		_, err := m.WrapSnapshots(consumer)(ctx, nil)
		require.NoError(t, err)
	})
}

func TestMultiTenancyMiddleware(t *testing.T) {
	m := MultiTenancyMiddleware()

	t.Run("injects tenant id grpc", func(t *testing.T) {
		tenantID := "test-tenant-id"

		consumer := newAssertingConsumer(t, func(t *testing.T, ctx context.Context) {
			extractTenantID, err := util.ExtractTenantID(ctx)
			require.NoError(t, err)
			require.Equal(t, tenantID, extractTenantID)
		})

		ctx := metadata.NewIncomingContext(
			context.Background(),
			metadata.Pairs(util.TenantIDHeaderName, tenantID),
		)
		_, err := m.WrapSnapshots(consumer)(ctx, nil)
		require.NoError(t, err)
	})

	t.Run("injects tenant id http", func(t *testing.T) {
		tenantID := "test-tenant-id"

		consumer := newAssertingConsumer(t, func(t *testing.T, ctx context.Context) {
			extractTenantID, err := util.ExtractTenantID(ctx)
			require.NoError(t, err)
			require.Equal(t, extractTenantID, tenantID)
		})

		info := client.Info{
			Metadata: client.NewMetadata(map[string][]string{
				util.TenantIDHeaderName: {tenantID},
			}),
		}

		ctx := client.NewContext(context.Background(), info)
		_, err := m.WrapSnapshots(consumer)(ctx, nil)
		require.NoError(t, err)
	})

	t.Run("returns error if tenant id cannot be extracted", func(t *testing.T) {
		// no need to assert anything, because the wrapped function is never called
		consumer := newAssertingConsumer(t, func(t *testing.T, ctx context.Context) {})
		ctx := context.Background()

		_, err := m.WrapSnapshots(consumer)(ctx, nil)
		require.EqualError(t, err, "no org id")
	})
}
