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
	"bytes"
	"context"
	"fmt"
	"github.com/intergral/deep/pkg/deepql"
	"math/rand"
	"path"
	"testing"
	"time"

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
	req := deepql.MustExtractFetchSpansRequest(`{ .span.foo = "bar" || duration > 1s }`)

	req.StartTimeUnixNanos = uint64(1000 * time.Second)
	req.EndTimeUnixNanos = uint64(1001 * time.Second)

	resp, err := b.Fetch(ctx, req, common.DefaultSearchOptions())
	require.NoError(t, err, "search request:", req)

	spanSet, err := resp.Results.Next(ctx)
	require.NoError(t, err, "search request:", req)

	fmt.Println("-----------")
	fmt.Println(resp.Results.(*snapshotMetadataIterator).iter)
	fmt.Println("-----------")
	fmt.Println(spanSet)
}

func TestBackendBlockSearchDeepql(t *testing.T) {
	numSnapshots := 250
	snapshots := make([]*Snapshot, 0, numSnapshots)
	wantedSnapIdx := rand.Intn(numSnapshots)
	wantedSnapshotID := test.ValidSnapshotID(nil)
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

	searchesThatMatch := []deepql.FetchSnapshotRequest{
		{}, // Empty request
		{
			// Time range inside snapshot
			StartTimeUnixNanos: uint64(1100 * 1e9),
			EndTimeUnixNanos:   uint64(1600 * 1e9),
		},
		{
			// Time range overlap start
			StartTimeUnixNanos: uint64(900 * 1e9),
			EndTimeUnixNanos:   uint64(1500 * 1e9),
		},
		{
			// Time range overlap end
			StartTimeUnixNanos: uint64(1500 * 1e9),
			EndTimeUnixNanos:   uint64(2100 * 1e9),
		},
		// Intrinsics
		deepql.MustExtractFetchSpansRequest(`{` + LabelDuration + ` =  100s}`),
		deepql.MustExtractFetchSpansRequest(`{` + LabelDuration + ` >  99s}`),
		deepql.MustExtractFetchSpansRequest(`{` + LabelDuration + ` >= 100s}`),
		deepql.MustExtractFetchSpansRequest(`{` + LabelDuration + ` <  101s}`),
		deepql.MustExtractFetchSpansRequest(`{` + LabelDuration + ` <= 100s}`),
		deepql.MustExtractFetchSpansRequest(`{` + LabelDuration + ` <= 100s}`),
		//deepql.MustExtractFetchSpansRequest(`{` + LabelStatus + ` = error}`),
		//deepql.MustExtractFetchSpansRequest(`{` + LabelStatus + ` = 2}`),
		//deepql.MustExtractFetchSpansRequest(`{` + LabelKind + ` = client }`),
		// Resource well-known attributes
		deepql.MustExtractFetchSpansRequest(`{.` + LabelServiceName + ` = "spanservicename"}`), // Overridden at span
		deepql.MustExtractFetchSpansRequest(`{.` + LabelCluster + ` = "cluster"}`),
		deepql.MustExtractFetchSpansRequest(`{.` + LabelNamespace + ` = "namespace"}`),
		deepql.MustExtractFetchSpansRequest(`{.` + LabelPod + ` = "pod"}`),
		deepql.MustExtractFetchSpansRequest(`{.` + LabelContainer + ` = "container"}`),
		deepql.MustExtractFetchSpansRequest(`{.` + LabelK8sNamespaceName + ` = "k8snamespace"}`),
		deepql.MustExtractFetchSpansRequest(`{.` + LabelK8sClusterName + ` = "k8scluster"}`),
		deepql.MustExtractFetchSpansRequest(`{.` + LabelK8sPodName + ` = "k8spod"}`),
		deepql.MustExtractFetchSpansRequest(`{.` + LabelK8sContainerName + ` = "k8scontainer"}`),
		deepql.MustExtractFetchSpansRequest(`{resource.` + LabelCluster + ` = "cluster"}`),
		deepql.MustExtractFetchSpansRequest(`{resource.` + LabelNamespace + ` = "namespace"}`),
		deepql.MustExtractFetchSpansRequest(`{resource.` + LabelPod + ` = "pod"}`),
		deepql.MustExtractFetchSpansRequest(`{resource.` + LabelContainer + ` = "container"}`),
		deepql.MustExtractFetchSpansRequest(`{resource.` + LabelK8sNamespaceName + ` = "k8snamespace"}`),
		deepql.MustExtractFetchSpansRequest(`{resource.` + LabelK8sClusterName + ` = "k8scluster"}`),
		deepql.MustExtractFetchSpansRequest(`{resource.` + LabelK8sPodName + ` = "k8spod"}`),
		deepql.MustExtractFetchSpansRequest(`{resource.` + LabelK8sContainerName + ` = "k8scontainer"}`),
		// Span well-known attributes
		//deepql.MustExtractFetchSpansRequest(`{.` + LabelHTTPStatusCode + ` = 500}`),
		//deepql.MustExtractFetchSpansRequest(`{.` + LabelHTTPMethod + ` = "get"}`),
		//deepql.MustExtractFetchSpansRequest(`{.` + LabelHTTPUrl + ` = "url/hello/world"}`),
		//deepql.MustExtractFetchSpansRequest(`{span.` + LabelHTTPStatusCode + ` = 500}`),
		//deepql.MustExtractFetchSpansRequest(`{span.` + LabelHTTPMethod + ` = "get"}`),
		//deepql.MustExtractFetchSpansRequest(`{span.` + LabelHTTPUrl + ` = "url/hello/world"}`),
		// Basic data types and operations
		//deepql.MustExtractFetchSpansRequest(`{.float = 456.78}`),      // Float ==
		//deepql.MustExtractFetchSpansRequest(`{.float != 456.79}`),     // Float !=
		//deepql.MustExtractFetchSpansRequest(`{.float > 456.7}`),       // Float >
		//deepql.MustExtractFetchSpansRequest(`{.float >= 456.78}`),     // Float >=
		//deepql.MustExtractFetchSpansRequest(`{.float < 456.781}`),     // Float <
		//deepql.MustExtractFetchSpansRequest(`{.bool = false}`),        // Bool ==
		//deepql.MustExtractFetchSpansRequest(`{.bool != true}`),        // Bool !=
		//deepql.MustExtractFetchSpansRequest(`{.bar = 123}`),           // Int ==
		//deepql.MustExtractFetchSpansRequest(`{.bar != 124}`),          // Int !=
		//deepql.MustExtractFetchSpansRequest(`{.bar > 122}`),           // Int >
		//deepql.MustExtractFetchSpansRequest(`{.bar >= 123}`),          // Int >=
		//deepql.MustExtractFetchSpansRequest(`{.bar < 124}`),           // Int <
		//deepql.MustExtractFetchSpansRequest(`{.bar <= 123}`),          // Int <=
		deepql.MustExtractFetchSpansRequest(`{.foo = "def"}`),         // String ==
		deepql.MustExtractFetchSpansRequest(`{.foo != "deg"}`),        // String !=
		deepql.MustExtractFetchSpansRequest(`{.foo =~ "d.*"}`),        // String Regex
		deepql.MustExtractFetchSpansRequest(`{resource.foo = "abc"}`), // Resource-level only
		//deepql.MustExtractFetchSpansRequest(`{.foo}`),                 // Projection only

		// Edge cases
		deepql.MustExtractFetchSpansRequest(`{.` + LabelServiceName + ` = "spanservicename"}`), // service.name present on span
		deepql.MustExtractFetchSpansRequest(`{.foo = "def"}`),
	}

	for _, req := range searchesThatMatch {
		resp, err := b.Fetch(ctx, req, common.DefaultSearchOptions())
		require.NoError(t, err, "search request:", req)

		found := false
		for {
			spanSet, err := resp.Results.Next(ctx)
			require.NoError(t, err, "search request:", req)
			if spanSet == nil {
				break
			}
			found = bytes.Equal(spanSet.SnapshotID, wantedSnapshotID)
			if found {
				break
			}
		}
		require.True(t, found, "search request:", req)
	}

	searchesThatDontMatch := []deepql.FetchSnapshotRequest{
		// TODO - Should the below query return data or not?  It does match the resource
		// makeReq(parse(t, `{.foo = "abc"}`)),                           // This should not return results because the span has overridden this attribute to "def".
		deepql.MustExtractFetchSpansRequest(`{.foo =~ "xyz.*"}`),                            // Regex IN
		deepql.MustExtractFetchSpansRequest(`{` + LabelDuration + ` >  100s}`),              // Intrinsic: duration
		deepql.MustExtractFetchSpansRequest(`{.` + LabelServiceName + ` = "notmyservice"}`), // Well-known attribute: service.name not match
		{
			// Time range after snapshot
			StartTimeUnixNanos: uint64(3000 * time.Second),
			EndTimeUnixNanos:   uint64(4000 * time.Second),
		},
		{
			// Time range before snapshot
			StartTimeUnixNanos: uint64(600 * time.Second),
			EndTimeUnixNanos:   uint64(700 * time.Second),
		},
		{
			// Matches some conditions but not all
			// Mix of span and resource columns
			AllConditions: true,
			Conditions: []deepql.Condition{
				parse(t, `{resource.cluster = "cluster"}`),     // match
				parse(t, `{resource.namespace = "namespace"}`), // match
				parse(t, `{.foo = "baz"}`),                     // no match
			},
		},
		{
			// Matches some conditions but not all
			// Mix of resource columns
			AllConditions: true,
			Conditions: []deepql.Condition{
				parse(t, `{resource.cluster = "notcluster"}`),  // no match
				parse(t, `{resource.namespace = "namespace"}`), // match
				parse(t, `{resource.foo = "abc"}`),             // match
			},
		},
		{
			// Matches some conditions but not all
			// Only resource generic attr lookups
			AllConditions: true,
			Conditions: []deepql.Condition{
				parse(t, `{resource.foo = "abc"}`), // match
				parse(t, `{resource.bar = 123}`),   // no match
			},
		},
		{
			// Mix of duration with other conditions
			AllConditions: true,
			Conditions: []deepql.Condition{
				parse(t, `{`+LabelDuration+` = 100s }`), // Match
				parse(t, `{resource.bar = 123}`),        // no match
			},
		},
	}

	for _, req := range searchesThatDontMatch {
		resp, err := b.Fetch(ctx, req, common.DefaultSearchOptions())
		require.NoError(t, err, "search request:", req)

		for {
			spanSet, err := resp.Results.Next(ctx)
			require.NoError(t, err, "search request:", req)
			if spanSet == nil {
				break
			}
			require.NotEqual(t, wantedSnapshotID, spanSet.SnapshotID, "search request:", req)
		}
	}
}

func makeReq(conditions ...deepql.Condition) deepql.FetchSnapshotRequest {
	return deepql.FetchSnapshotRequest{
		Conditions: conditions,
	}
}

func parse(t *testing.T, q string) deepql.Condition {

	req, err := deepql.ExtractFetchSpansRequest(q)
	require.NoError(t, err, "query:", q)

	return req.Conditions[0]
}

func fullyPopulatedTestSnapshot(id common.ID) *Snapshot {
	snapshot := test.GenerateSnapshot(0, &test.GenerateOptions{Id: id, ServiceName: "spanservicename", Resource: map[string]string{
		"cluster":            "cluster",
		"namespace":          "namespace",
		"pod":                "pod",
		"container":          "container",
		"k8s.namespace.name": "k8snamespace",
		"k8s.cluster.name":   "k8scluster",
		"k8s.pod.name":       "k8spod",
		"k8s.container.name": "k8scontainer",
		"foo":                "abc",
	}, Attrs: map[string]string{
		"foo": "def",
	}})
	snapshot.TsNanos = 1500 * 1e9
	snapshot.DurationNanos = 100 * 1e9

	return snapshotToParquet(snapshot.ID, snapshot, nil)
}

func BenchmarkBackendBlockdeepql(b *testing.B) {
	testCases := []struct {
		name string
		req  deepql.FetchSnapshotRequest
	}{
		// span
		{"spanAttNameNoMatch", deepql.MustExtractFetchSpansRequest("{ span.foo = `bar` }")},
		{"spanAttValNoMatch", deepql.MustExtractFetchSpansRequest("{ span.bloom = `bar` }")},
		{"spanAttValMatch", deepql.MustExtractFetchSpansRequest("{ span.bloom > 0 }")},
		{"spanAttIntrinsicNoMatch", deepql.MustExtractFetchSpansRequest("{ name = `asdfasdf` }")},
		{"spanAttIntrinsicMatch", deepql.MustExtractFetchSpansRequest("{ name = `gcs.ReadRange` }")},

		// resource
		{"resourceAttNameNoMatch", deepql.MustExtractFetchSpansRequest("{ resource.foo = `bar` }")},
		{"resourceAttValNoMatch", deepql.MustExtractFetchSpansRequest("{ resource.module.path = `bar` }")},
		{"resourceAttValMatch", deepql.MustExtractFetchSpansRequest("{ resource.os.type = `linux` }")},
		{"resourceAttIntrinsicNoMatch", deepql.MustExtractFetchSpansRequest("{ resource.service.name = `a` }")},
		{"resourceAttIntrinsicMatch", deepql.MustExtractFetchSpansRequest("{ resource.service.name = `deep-query-frontend` }")},

		// mixed
		{"mixedNameNoMatch", deepql.MustExtractFetchSpansRequest("{ .foo = `bar` }")},
		{"mixedValNoMatch", deepql.MustExtractFetchSpansRequest("{ .bloom = `bar` }")},
		{"mixedValMixedMatchAnd", deepql.MustExtractFetchSpansRequest("{ resource.foo = `bar` && name = `gcs.ReadRange` }")},
		{"mixedValMixedMatchOr", deepql.MustExtractFetchSpansRequest("{ resource.foo = `bar` || name = `gcs.ReadRange` }")},
		{"mixedValBothMatch", deepql.MustExtractFetchSpansRequest("{ resource.service.name = `query-frontend` && name = `gcs.ReadRange` }")},
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

	for _, tc := range testCases {

		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			bytesRead := 0

			for i := 0; i < b.N; i++ {
				resp, err := block.Fetch(ctx, tc.req, opts)
				require.NoError(b, err)
				require.NotNil(b, resp)

				// Read first 20 results (if any)
				for i := 0; i < 20; i++ {
					ss, err := resp.Results.Next(ctx)
					require.NoError(b, err)
					if ss == nil {
						break
					}
				}
				bytesRead += int(resp.Bytes())
			}
			b.SetBytes(int64(bytesRead) / int64(b.N))
			b.ReportMetric(float64(bytesRead)/float64(b.N)/1000.0/1000.0, "MB_io/op")
		})
	}
}
