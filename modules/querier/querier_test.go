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

package querier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/intergral/deep/pkg/util"

	"github.com/intergral/deep/modules/ingester/client"
	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/pkg/deeppb"
	"github.com/stretchr/testify/require"
	"github.com/uber-go/atomic"
)

func TestQuerierUsesSearchExternalEndpoint(t *testing.T) {
	numExternalRequests := atomic.NewInt32(0)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		numExternalRequests.Inc()
	}))
	defer srv.Close()

	tests := []struct {
		cfg              Config
		queriesToExecute int
		externalExpected int32
	}{
		// SearchExternalEndpoints is respected
		{
			cfg: Config{
				Search: SearchConfig{
					ExternalEndpoints: []string{srv.URL},
				},
			},
			queriesToExecute: 3,
			externalExpected: 3,
		},
		// No SearchExternalEndpoints causes the querier to service everything internally
		{
			cfg:              Config{},
			queriesToExecute: 3,
			externalExpected: 0,
		},
		// SearchPreferSelf is respected. this test won't pass b/c SearchBlock fails instantly and so
		//  all 3 queries are executed locally and nothing is proxied to the external endpoint.
		//  we'd have to mock the storage.Store interface to get this to pass. it's a big interface.
		// {
		// 	cfg: Config{
		// 		SearchExternalEndpoints: []string{srv.URL},
		// 		SearchPreferSelf:        2,
		// 	},
		// 	queriesToExecute: 3,
		// 	externalExpected: 1,
		// },
	}

	ctx := util.InjectTenantID(context.Background(), "blerg")

	for _, tc := range tests {
		numExternalRequests.Store(0)

		o, err := overrides.NewOverrides(overrides.Limits{})
		require.NoError(t, err)

		q, err := New(tc.cfg, client.Config{}, nil, nil, o)
		require.NoError(t, err)

		for i := 0; i < tc.queriesToExecute; i++ {
			// ignore error purposefully here. all queries will error, but we don't care
			// numExternalRequests will tell us what we need to know
			_, _ = q.SearchBlock(ctx, &deeppb.SearchBlockRequest{})
		}

		require.Equal(t, tc.externalExpected, numExternalRequests.Load())
	}
}
