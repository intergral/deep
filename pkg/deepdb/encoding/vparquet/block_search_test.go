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
	"math/rand"
	"path"
	"testing"

	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deeppb"
	deep_io "github.com/intergral/deep/pkg/io"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/require"
)

func TestBackendBlockSearch(t *testing.T) {
	snapshot := fullyPopulatedTestSnapshot(test.ValidSnapshotID(nil))

	// make a bunch of snapshots and include our snapshot above
	total := 1000
	insertAt := rand.Intn(total)
	allSnapshots := make([]*Snapshot, 0, total)
	for i := 0; i < total; i++ {
		if i == insertAt {
			allSnapshots = append(allSnapshots, snapshot)
			continue
		}

		id := test.ValidSnapshotID(nil)
		pbSnapshot := test.GenerateSnapshot(10, &test.GenerateOptions{Id: id, ServiceName: "test-service"})
		pqSnapshot := snapshotToParquet(id, pbSnapshot, nil)
		allSnapshots = append(allSnapshots, pqSnapshot)
	}

	b := makeBackendBlockWithSnapshots(t, allSnapshots)
	ctx := context.TODO()

	// Helper function to make a tag search
	makeReq := func(k, v string) *deeppb.SearchRequest {
		return &deeppb.SearchRequest{
			Tags: map[string]string{
				k: v,
			},
		}
	}

	// Matches
	searchesThatMatch := []*deeppb.SearchRequest{
		{
			// Empty request
		},
		{
			MinDurationMs: 99000,
			MaxDurationMs: 101000,
		},
		{
			Start: 1000,
			End:   2000,
		},
		{
			// Overlaps start
			Start: 1499,
			End:   1501,
		},

		// Well-known resource attributes
		makeReq(LabelServiceName, "service"),
		makeReq(LabelCluster, "cluster"),
		makeReq(LabelNamespace, "namespace"),
		makeReq(LabelPod, "pod"),
		makeReq(LabelContainer, "container"),
		makeReq(LabelK8sClusterName, "k8scluster"),
		makeReq(LabelK8sNamespaceName, "k8snamespace"),
		makeReq(LabelK8sPodName, "k8spod"),
		makeReq(LabelK8sContainerName, "k8scontainer"),

		// attributes
		makeReq("foo", "def"),

		// Multiple
		{
			Tags: map[string]string{
				"service.name": "test-service-name",
				"foo":          "def",
			},
		},
	}
	expected := &deeppb.SnapshotSearchMetadata{
		SnapshotID:        util.SnapshotIDToHexString(snapshot.ID),
		ServiceName:       "test-service-name",
		FilePath:          snapshot.Tracepoint.Path,
		LineNo:            snapshot.Tracepoint.LineNumber,
		StartTimeUnixNano: snapshot.TsNanos,
		DurationNano:      snapshot.DurationNanos,
	}

	findInResults := func(id string, res []*deeppb.SnapshotSearchMetadata) *deeppb.SnapshotSearchMetadata {
		for _, r := range res {
			if r.SnapshotID == id {
				return r
			}
		}
		return nil
	}

	for _, req := range searchesThatMatch {
		res, err := b.Search(ctx, req, common.DefaultSearchOptions())
		require.NoError(t, err)

		meta := findInResults(expected.SnapshotID, res.Snapshots)
		require.NotNil(t, meta, "search request:", req)
		require.Equal(t, expected, meta, "search request:", req)
	}

	// Excludes
	searchesThatDontMatch := []*deeppb.SearchRequest{
		{
			MinDurationMs: 101000,
		},
		{
			MaxDurationMs: 99000,
		},
		{
			Start: 100,
			End:   200,
		},

		// Well-known resource attributes
		makeReq(LabelServiceName, "foo"),
		makeReq(LabelCluster, "foo"),
		makeReq(LabelNamespace, "foo"),
		makeReq(LabelPod, "foo"),
		makeReq(LabelContainer, "foo"),

		// attributes
		makeReq("foo", "baz"),

		// Multiple
		{
			Tags: map[string]string{
				"http.status_code": "500",
				"service.name":     "asdf",
			},
		},
	}
	for _, req := range searchesThatDontMatch {
		res, err := b.Search(ctx, req, common.DefaultSearchOptions())
		require.NoError(t, err)
		meta := findInResults(expected.SnapshotID, res.Snapshots)
		require.Nil(t, meta, "search request:", req)
	}
}

func makeBackendBlockWithSnapshots(t *testing.T, trs []*Snapshot) *backendBlock {
	rawR, rawW, _, err := local.New(&local.Config{
		Path: t.TempDir(),
	})
	require.NoError(t, err)

	r := backend.NewReader(rawR)
	w := backend.NewWriter(rawW)
	ctx := context.Background()

	cfg := &common.BlockConfig{
		BloomFP:             0.01,
		BloomShardSizeBytes: 100 * 1024,
	}

	meta := backend.NewBlockMeta("fake", uuid.New(), VersionString, backend.EncNone, "")
	meta.TotalObjects = 1

	s := newStreamingBlock(ctx, cfg, meta, r, w, deep_io.NewBufferedWriter)

	for i, tr := range trs {
		err = s.Add(tr, 0)
		require.NoError(t, err)
		if i%100 == 0 {
			_, err := s.Flush()
			require.NoError(t, err)
		}
	}

	_, err = s.Complete()
	require.NoError(t, err)

	b := newBackendBlock(s.meta, r)

	return b
}

func BenchmarkBackendBlockSearchSnapshots(b *testing.B) {
	testCases := []struct {
		name string
		tags map[string]string
	}{
		{"noMatch", map[string]string{"foo": "bar"}},
		{"partialMatch", map[string]string{"foo": "bar", "component": "gRPC"}},
		{"service.name", map[string]string{"service.name": "a"}},
	}

	ctx := context.TODO()
	tenantID := "1"
	blockID := uuid.MustParse("3685ee3d-cbbf-4f36-bf28-93447a19dea6")

	r, _, _, err := local.New(&local.Config{
		Path: path.Join("/Users/marty/src/tmp/"),
	})
	require.NoError(b, err)

	rr := backend.NewReader(r)
	meta, err := rr.BlockMeta(ctx, blockID, tenantID)
	require.NoError(b, err)

	block := newBackendBlock(meta, rr)

	opts := common.DefaultSearchOptions()
	opts.StartPage = 10
	opts.TotalPages = 10

	for _, tc := range testCases {

		req := &deeppb.SearchRequest{
			Tags:  tc.tags,
			Limit: 20,
		}

		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			bytesRead := 0
			for i := 0; i < b.N; i++ {
				resp, err := block.Search(ctx, req, opts)
				require.NoError(b, err)
				bytesRead += int(resp.Metrics.InspectedBytes)
			}
			b.SetBytes(int64(bytesRead) / int64(b.N))
			b.ReportMetric(float64(bytesRead)/float64(b.N), "bytes/op")
		})
	}
}
