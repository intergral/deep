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

package frontend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/intergral/deep/pkg/deeppb"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/util"

	"github.com/go-kit/log"
	"github.com/golang/protobuf/proto"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBlockBoundaries(t *testing.T) {
	tests := []struct {
		name        string
		queryShards int
		expected    [][]byte
	}{
		{
			name:        "single shard",
			queryShards: 1,
			expected: [][]byte{
				{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
				{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			},
		},
		{
			name:        "multiple shards",
			queryShards: 4,
			expected: [][]byte{
				{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},  // 0
				{0x3f, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}, // 0x3f = 255/4 * 1
				{0x7e, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}, // 0x7e = 255/4 * 2
				{0xbd, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}, // 0xbd = 255/4 * 3
				{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bb := createBlockBoundaries(tt.queryShards)
			assert.Len(t, bb, len(tt.expected))

			for i := 0; i < len(bb); i++ {
				assert.Equal(t, tt.expected[i], bb[i])
			}
		})
	}
}

func TestBuildShardedRequests(t *testing.T) {
	queryShards := 2

	sharder := &shardQuery{
		queryShards:     queryShards,
		blockBoundaries: createBlockBoundaries(queryShards - 1),
	}

	ctx := util.InjectTenantID(context.Background(), "blerg")
	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)

	shardedReqs, err := sharder.buildShardedRequests(req)
	require.NoError(t, err)
	require.Len(t, shardedReqs, queryShards)

	require.Equal(t, "/querier?mode=ingesters", shardedReqs[0].RequestURI)
	require.Equal(t, "/querier?blockEnd=ffffffffffffffffffffffffffffffff&blockStart=00000000000000000000000000000000&mode=blocks", shardedReqs[1].RequestURI)
}

func TestShardingWareDoRequest(t *testing.T) {
	snapshot := test.GenerateSnapshot(10, nil)
	snapshot1 := snapshot
	snapshot2 := snapshot

	tests := []struct {
		name                string
		status1             int
		status2             int
		snapshot1           *deeptp.Snapshot
		snapshot2           *deeptp.Snapshot
		err1                error
		err2                error
		failedBlockQueries1 int
		failedBlockQueries2 int
		expectedStatus      int
		expectedSnapshot    *deeptp.Snapshot
		expectedError       error
	}{
		{
			name:           "404",
			status1:        404,
			status2:        404,
			expectedStatus: 404,
		},
		{
			name:           "400",
			status1:        400,
			status2:        400,
			expectedStatus: 500,
		},
		{
			name:           "500+404",
			status1:        500,
			status2:        404,
			expectedStatus: 500,
		},
		{
			name:           "404+500",
			status1:        404,
			status2:        500,
			expectedStatus: 500,
		},
		{
			name:           "500+200",
			status1:        500,
			status2:        200,
			snapshot2:      snapshot2,
			expectedStatus: 500,
		},
		{
			name:           "200+500",
			status1:        200,
			snapshot1:      snapshot1,
			status2:        500,
			expectedStatus: 200,
		},
		{
			name:           "503+200",
			status1:        503,
			status2:        200,
			snapshot2:      snapshot2,
			expectedStatus: 500,
		},
		{
			name:           "200+503",
			status1:        200,
			snapshot1:      snapshot1,
			status2:        503,
			expectedStatus: 200,
		},
		{
			name:             "200+404",
			status1:          200,
			snapshot1:        snapshot1,
			status2:          404,
			expectedStatus:   200,
			expectedSnapshot: snapshot1,
		},
		{
			name:             "404+200",
			status1:          404,
			status2:          200,
			snapshot2:        snapshot1,
			expectedStatus:   200,
			expectedSnapshot: snapshot1,
		},
		{
			name:             "200+200",
			status1:          200,
			snapshot1:        snapshot1,
			status2:          200,
			snapshot2:        snapshot2,
			expectedStatus:   200,
			expectedSnapshot: snapshot,
		},
		{
			name:             "200+err",
			status1:          200,
			snapshot1:        snapshot1,
			err2:             errors.New("booo"),
			expectedStatus:   200,
			expectedSnapshot: snapshot1,
		},
		{
			name:          "err+200",
			err1:          errors.New("booo"),
			status2:       200,
			snapshot2:     snapshot1,
			expectedError: errors.New("booo"),
		},
		{
			name:          "500+err",
			status1:       500,
			snapshot1:     snapshot1,
			err2:          errors.New("booo"),
			expectedError: errors.New("booo"),
		},
		{
			name:                "failedBlocks under: 200+200",
			status1:             200,
			snapshot1:           snapshot1,
			status2:             200,
			snapshot2:           snapshot2,
			failedBlockQueries1: 1,
			failedBlockQueries2: 1,
			expectedStatus:      200,
			expectedSnapshot:    snapshot,
		},
		{
			name:                "failedBlocks over: 200+200",
			status1:             200,
			snapshot1:           snapshot1,
			status2:             200,
			snapshot2:           snapshot2,
			failedBlockQueries1: 0,
			failedBlockQueries2: 5,
			expectedError:       nil,
			expectedSnapshot:    snapshot1,
			expectedStatus:      200,
		},
		{
			name:                "failedBlocks under: 200+404",
			status1:             200,
			snapshot1:           snapshot1,
			status2:             404,
			failedBlockQueries1: 1,
			failedBlockQueries2: 0,
			expectedStatus:      200,
			expectedSnapshot:    snapshot1,
		},
		{
			name:                "failedBlocks under: 404+200",
			status1:             200,
			snapshot1:           snapshot1,
			status2:             404,
			failedBlockQueries1: 0,
			failedBlockQueries2: 1,
			expectedStatus:      200,
			expectedSnapshot:    snapshot1,
		},
		{
			name:                "failedBlocks over: 404+200",
			status1:             200,
			snapshot1:           snapshot1,
			status2:             404,
			failedBlockQueries1: 0,
			failedBlockQueries2: 5,
			expectedError:       nil,
			expectedSnapshot:    snapshot1,
			expectedStatus:      200,
		},
		{
			name:                "failedBlocks over: 404+404",
			status1:             404,
			status2:             404,
			failedBlockQueries1: 0,
			failedBlockQueries2: 5,
			expectedError:       errors.New("too many failed block queries 5 (max 2)"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sharder := newSnapshotByIDSharder(2, 2, testSLOcfg, log.NewNopLogger())
			wg1 := sync.WaitGroup{}
			wg1.Add(1)
			next := RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
				var testSnapshot *deeptp.Snapshot
				var statusCode int
				var err error
				var failedBlockQueries int
				if strings.HasSuffix(r.RequestURI, "?mode=ingesters") {
					testSnapshot = tc.snapshot1
					statusCode = tc.status1
					err = tc.err1
					failedBlockQueries = tc.failedBlockQueries1
					defer wg1.Done()
				} else {
					testSnapshot = tc.snapshot2
					err = tc.err2
					statusCode = tc.status2
					failedBlockQueries = tc.failedBlockQueries2
					// try to make this request wait till the above is complete
					wg1.Wait()
				}

				if err != nil {
					return nil, err
				}

				resBytes := []byte("error occurred")
				if statusCode != 500 {
					if testSnapshot != nil {
						resBytes, err = proto.Marshal(&deeppb.SnapshotByIDResponse{
							Snapshot: testSnapshot,
							Metrics: &deeppb.SnapshotByIDMetrics{
								FailedBlocks: uint32(failedBlockQueries),
							},
						})
						require.NoError(t, err)
					} else {
						resBytes, err = proto.Marshal(&deeppb.SnapshotByIDResponse{
							Metrics: &deeppb.SnapshotByIDMetrics{
								FailedBlocks: uint32(failedBlockQueries),
							},
						})
						require.NoError(t, err)
					}
				}
				return &http.Response{
					Body:       io.NopCloser(bytes.NewReader(resBytes)),
					StatusCode: statusCode,
				}, nil
			})

			testRT := NewRoundTripper(next, sharder)

			req := httptest.NewRequest("GET", "/api/snapshots/1234", nil)
			ctx := req.Context()
			ctx = util.InjectTenantID(ctx, "blerg")
			req = req.WithContext(ctx)

			resp, err := testRT.RoundTrip(req)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
			if tc.expectedStatus == http.StatusOK {
				assert.Equal(t, "application/protobuf", resp.Header.Get("Content-Type"))
			}
			if tc.expectedSnapshot != nil {
				actualResp := &deeppb.SnapshotByIDResponse{}
				snapBytes, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				err = proto.Unmarshal(snapBytes, actualResp)
				require.NoError(t, err)

				assert.True(t, proto.Equal(tc.expectedSnapshot, actualResp.Snapshot))
			}
		})
	}
}
