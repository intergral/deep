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
	"fmt"
	"math/rand"
	"path"
	"testing"

	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/deepql"
	"github.com/intergral/deep/pkg/util"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/util/test"
)

func TestOne(t *testing.T) {
	wantTr := fullyPopulatedTestSnapshot(nil)
	b := makeBackendBlockWithSnapshots(t, []*Snapshot{wantTr})
	ctx := context.Background()
	req := &deeppb.SearchRequest{
		Query: `{ foo = "def" duration > 1s }`,
		Start: 1000,
		End:   10001,
	}

	engine := &deepql.Engine{}
	resp, err := engine.ExecuteSearch(ctx, req, func(ctx context.Context, request deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
		return b.Fetch(ctx, request, common.DefaultSearchOptions())
	})

	require.NoError(t, err, "search request:", req.Query)

	snapshot := resp.Snapshots[0]
	if snapshot.SnapshotID != wantTr.IDText {
		t.Error("Snapshot if doesn't match")
	}
}

func TestBackendBlockSearchDeepql(t *testing.T) {
	numSnapshots := 250
	snapshots := make([]*Snapshot, 0, numSnapshots)
	wantedSnapIdx := rand.Intn(numSnapshots)
	wantedSnapshotID := test.ValidSnapshotID(nil)
	wantedHexId := util.SnapshotIDToHexString(wantedSnapshotID)
	for i := 0; i < numSnapshots; i++ {
		if i == wantedSnapIdx {
			snapshots = append(snapshots, fullyPopulatedTestSnapshot(wantedSnapshotID))
			continue
		}

		id := test.ValidSnapshotID(nil)
		snapshot := snapshotToParquet(id, test.GenerateSnapshot(i, &test.GenerateOptions{Id: id}), nil)
		snapshots = append(snapshots, snapshot)
	}

	b := makeBackendBlockWithSnapshots(t, snapshots)
	ctx := context.Background()

	searchesThatMatch := []*deeppb.SearchRequest{
		{
			Query: "{}",
		}, // Empty request
		{
			// Time range inside snapshot
			Start: 1100,
			End:   1600,
			Query: "{}",
		},
		{
			// Time range overlap start
			Start: 900,
			End:   1500,
			Query: "{}",
		},
		{
			// Time range overlap end
			Start: 1500,
			End:   2100,
			Query: "{}",
		},
		// Intrinsics
		{Query: `{` + LabelDuration + ` =  100s}`},
		{Query: `{` + LabelDuration + ` >  99s}`},
		{Query: `{` + LabelDuration + ` >= 100s}`},
		{Query: `{` + LabelDuration + ` <  101s}`},
		{Query: `{` + LabelDuration + ` <= 100s}`},
		{Query: `{` + LabelDuration + ` <= 100s}`},

		// Resource well-known attributes
		{Query: `{` + LabelServiceName + ` = "test-service-name"}`},
		{Query: `{` + LabelCluster + ` = "cluster"}`},
		{Query: `{` + LabelNamespace + ` = "namespace"}`},
		{Query: `{` + LabelPod + ` = "pod"}`},
		{Query: `{` + LabelContainer + ` = "container"}`},
		{Query: `{` + LabelK8sNamespaceName + ` = "k8snamespace"}`},
		{Query: `{` + LabelK8sClusterName + ` = "k8scluster"}`},
		{Query: `{` + LabelK8sPodName + ` = "k8spod"}`},
		{Query: `{` + LabelK8sContainerName + ` = "k8scontainer"}`},
		{Query: `{resource.` + LabelCluster + ` = "cluster"}`},
		{Query: `{resource.` + LabelNamespace + ` = "namespace"}`},
		{Query: `{resource.` + LabelPod + ` = "pod"}`},
		{Query: `{resource.` + LabelContainer + ` = "container"}`},
		{Query: `{resource.` + LabelK8sNamespaceName + ` = "k8snamespace"}`},
		{Query: `{resource.` + LabelK8sClusterName + ` = "k8scluster"}`},
		{Query: `{resource.` + LabelK8sPodName + ` = "k8spod"}`},
		{Query: `{resource.` + LabelK8sContainerName + ` = "k8scontainer"}`},
		// Basic data types and operations
		{Query: `{float = 456.78}`},       // Float ==
		{Query: `{float != 456.79}`},      // Float !=
		{Query: `{float > 456.7}`},        // Float >
		{Query: `{float >= 456.78}`},      // Float >=
		{Query: `{float < 456.781}`},      // Float <
		{Query: `{bool = false}`},         // Bool ==
		{Query: `{bool != true}`},         // Bool !=
		{Query: `{bar = 123}`},            // Int ==
		{Query: `{bar != 124}`},           // Int !=
		{Query: `{bar > 122}`},            // Int >
		{Query: `{bar >= 123}`},           // Int >=
		{Query: `{bar < 124}`},            // Int <
		{Query: `{bar <= 123}`},           // Int <=
		{Query: `{foo = "def"}`},          // String ==
		{Query: `{foo != "deg"}`},         // String !=
		{Query: `{foo =~ "d.*"}`},         // String Regex
		{Query: `{resource.foo = "abc"}`}, // Resource-level only

		// Edge cases
		{Query: `{` + LabelServiceName + ` = "test-service-name"}`},
		{Query: `{foo = "def"}`},
	}

	engine := deepql.NewEngine()
	for _, req := range searchesThatMatch {
		t.Run(fmt.Sprintf("should find: %s", req.Query), func(t *testing.T) {
			resp, err := engine.ExecuteSearch(ctx, req, func(ctx context.Context, request deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
				return b.Fetch(ctx, request, common.DefaultSearchOptions())
			})
			require.NoError(t, err, "search request: %s", req.Query)

			found := false

			for _, snapshot := range resp.Snapshots {
				if snapshot.SnapshotID == wantedHexId {
					found = true
					break
				}
			}

			require.True(t, found, "search request: %s", req.Query)
		})
	}

	searchesThatDontMatch := []*deeppb.SearchRequest{
		// TODO - Should the below query return data or not?  It does match the resource
		{Query: `{foo =~ "xyz.*"}`},                            // Regex IN
		{Query: `{` + LabelDuration + ` >  100s}`},             // Intrinsic: duration
		{Query: `{` + LabelServiceName + ` = "notmyservice"}`}, // Well-known attribute: service.name not match
		{
			// Time range after snapshot
			Start: 3000 * 1000,
			End:   4000 * 1000,
			Query: "{}",
		},
		{
			// Time range before snapshot
			Start: 600 * 1000,
			End:   700 * 1000,
			Query: "{}",
		},
		{Query: `{resource.cluster = "cluster" resource.namespace = "namespace" foo = "baz"}`},
		{Query: `{resource.cluster = "notcluster" resource.namespace = "namespace" resource.foo = "abc"}`},
		{Query: `{resource.foo = "abc" resource.bar = "123"}`},
		{Query: `{` + LabelDuration + ` = 100s resource.bar = 123}`},
	}

	for _, req := range searchesThatDontMatch {
		t.Run(fmt.Sprintf("shouldn't find: %s", req.Query), func(t *testing.T) {
			resp, err := engine.ExecuteSearch(ctx, req, func(ctx context.Context, request deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
				return b.Fetch(ctx, request, common.DefaultSearchOptions())
			})
			require.NoError(t, err, "search request: %s", req.Query)

			found := false

			for _, snapshot := range resp.Snapshots {
				if snapshot.SnapshotID == wantedHexId {
					found = true
					break
				}
			}

			require.False(t, found, "search request: %s", req.Query)
		})
	}
}

func fullyPopulatedTestSnapshot(id common.ID) *Snapshot {
	snapshot := test.GenerateSnapshot(0, &test.GenerateOptions{Id: id, LogMsg: true, ServiceName: "test-service-name", Resource: map[string]interface{}{
		"cluster":            "cluster",
		"namespace":          "namespace",
		"pod":                "pod",
		"container":          "container",
		"k8s.namespace.name": "k8snamespace",
		"k8s.cluster.name":   "k8scluster",
		"k8s.pod.name":       "k8spod",
		"k8s.container.name": "k8scontainer",
		"foo":                "abc",
	}, Attrs: map[string]interface{}{
		"foo":   "def",
		"float": 456.78,
		"bool":  false,
		"bar":   123,
	}})
	snapshot.TsNanos = 1500 * 1e9
	snapshot.DurationNanos = 100 * 1e9

	return snapshotToParquet(snapshot.ID, snapshot, nil)
}

func BenchmarkBackendBlockdeepql(b *testing.B) {
	testCases := []struct {
		name string
		req  *deeppb.SearchRequest
	}{
		// snapshot
		{"spanAttNameNoMatch", &deeppb.SearchRequest{Query: "{ span.foo = `bar` }"}},
		{"spanAttValNoMatch", &deeppb.SearchRequest{Query: "{ span.bloom = `bar` }"}},
		{"spanAttValMatch", &deeppb.SearchRequest{Query: "{ span.bloom > 0 }"}},
		{"spanAttIntrinsicNoMatch", &deeppb.SearchRequest{Query: "{ name = `asdfasdf` }"}},
		{"spanAttIntrinsicMatch", &deeppb.SearchRequest{Query: "{ name = `gcs.ReadRange` }"}},

		// resource
		{"resourceAttNameNoMatch", &deeppb.SearchRequest{Query: "{ resource.foo = `bar` }"}},
		{"resourceAttValNoMatch", &deeppb.SearchRequest{Query: "{ resource.module.path = `bar` }"}},
		{"resourceAttValMatch", &deeppb.SearchRequest{Query: "{ resource.os.type = `linux` }"}},
		{"resourceAttIntrinsicNoMatch", &deeppb.SearchRequest{Query: "{ resource.service.name = `a` }"}},
		{"resourceAttIntrinsicMatch", &deeppb.SearchRequest{Query: "{ resource.service.name = `deep-query-frontend` }"}},

		// mixed
		{"mixedNameNoMatch", &deeppb.SearchRequest{Query: "{ .foo = `bar` }"}},
		{"mixedValNoMatch", &deeppb.SearchRequest{Query: "{ .bloom = `bar` }"}},
		{"mixedValMixedMatchAnd", &deeppb.SearchRequest{Query: "{ resource.foo = `bar` && name = `gcs.ReadRange` }"}},
		{"mixedValMixedMatchOr", &deeppb.SearchRequest{Query: "{ resource.foo = `bar` || name = `gcs.ReadRange` }"}},
		{"mixedValBothMatch", &deeppb.SearchRequest{Query: "{ resource.service.name = `query-frontend` && name = `gcs.ReadRange` }"}},
	}

	ctx := context.TODO()
	tenantID := "1"
	blockID := uuid.MustParse("149e41d2-cc4d-4f71-b355-3377eabc94c8")

	r, _, _, err := local.New(&local.Config{
		Path: path.Join("/home/joe/testblock/"),
	})
	require.NoError(b, err)

	rr := backend.NewReader(r)
	meta, err := rr.BlockMeta(ctx, blockID, tenantID)
	require.NoError(b, err)

	opts := common.DefaultSearchOptions()
	opts.StartPage = 10
	opts.TotalPages = 10

	block := newBackendBlock(meta, rr)
	_, _, err = block.openForSearch(ctx, opts)
	require.NoError(b, err)

	engine := deepql.NewEngine()

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			bytesRead := 0

			for i := 0; i < b.N; i++ {
				tc.req.Limit = 20
				resp, err := engine.ExecuteSearch(ctx, tc.req, func(ctx context.Context, request deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
					return block.Fetch(ctx, request, opts)
				})
				require.NoError(b, err)
				require.NotNil(b, resp)

				bytesRead += int(resp.Metrics.InspectedBytes)
			}
			b.SetBytes(int64(bytesRead) / int64(b.N))
			b.ReportMetric(float64(bytesRead)/float64(b.N)/1000.0/1000.0, "MB_io/op")
		})
	}
}
