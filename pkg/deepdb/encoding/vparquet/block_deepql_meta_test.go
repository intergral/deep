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

package vparquet

import (
	"context"
	"testing"

	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepql"
	"github.com/stretchr/testify/require"
)

func TestBackendBlockSearchFetchMetaData(t *testing.T) {
	snapshot := fullyPopulatedTestSnapshot(nil)
	b := makeBackendBlockWithSnapshots(t, []*Snapshot{snapshot})
	ctx := context.Background()

	// Helper functions to make requests

	testCases := []struct {
		req             deepql.FetchSnapshotRequest
		expectedResults []*deepql.SnapshotResult
	}{
		{
			// Empty request returns 1 snapshot with all spans
			deepql.FetchSnapshotRequest{},
			[]*deepql.SnapshotResult{{
				SnapshotID:         snapshot.ID,
				ServiceName:        "test-service-name",
				FilePath:           snapshot.Tracepoint.Path,
				LineNo:             snapshot.Tracepoint.LineNumber,
				StartTimeUnixNanos: snapshot.TsNanos,
				DurationNanos:      snapshot.DurationNanos,
			}},
		},
		{
			// doesn't match anything
			deepql.FetchSnapshotRequest{
				StartTimeUnixNanos: 0,
				EndTimeUnixNanos:   0,
				Conditions: []deepql.Condition{
					{
						Attribute: "xyz",
						Op:        deepql.OpEqual,
						Operands: []deepql.Static{
							deepql.NewStaticString("xyz"),
						},
					},
				},
			},
			nil,
		},
	}

	for _, tc := range testCases {
		req := tc.req
		resp, err := b.Fetch(ctx, req, common.DefaultSearchOptions())
		require.NoError(t, err, "search request:", req)

		// Turn iterator into slice
		var ss []*deepql.SnapshotResult
		for {
			snapshotResult, err := resp.Results.Next(ctx)
			require.NoError(t, err)
			if snapshotResult == nil {
				break
			}
			ss = append(ss, snapshotResult)
		}

		require.Equal(t, tc.expectedResults, ss, "search request:", req)
	}
}
