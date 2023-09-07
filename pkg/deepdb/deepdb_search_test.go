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

package deepdb

import (
	"context"
	"fmt"
	"math/rand"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/intergral/deep/pkg/deeppb"
	v1_common "github.com/intergral/deep/pkg/deeppb/common/v1"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/deepql"

	"github.com/go-kit/log"
	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/wal"
	"github.com/intergral/deep/pkg/model"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/require"
)

// todo this needs more clean up and work
func TestSearchCompleteBlock(t *testing.T) {
	for _, v := range encoding.AllEncodings() {
		vers := v.Version()
		t.Run(vers, func(t *testing.T) {
			testSearchCompleteBlock(t, vers)
		})
	}
}

func testSearchCompleteBlock(t *testing.T, blockVersion string) {
	runCompleteBlockSearchTest(t, blockVersion, func(_ *deeptp.Snapshot, wantMeta *deeppb.SnapshotSearchMetadata, searchesThatMatch, searchesThatDontMatch []*deeppb.SearchRequest, meta *backend.BlockMeta, r Reader) {
		ctx := context.Background()

		for _, req := range searchesThatMatch {
			res, err := r.Search(ctx, meta, req, common.DefaultSearchOptions())
			if err == common.ErrUnsupported {
				return
			}
			require.NoError(t, err, "search request: %+v", req)
			require.Equal(t, wantMeta, actualForExpectedMeta(wantMeta, res), "search request: %v", req)
		}

		for _, req := range searchesThatDontMatch {
			res, err := r.Search(ctx, meta, req, common.DefaultSearchOptions())
			require.NoError(t, err, "search request: %+v", req)
			require.Nil(t, actualForExpectedMeta(wantMeta, res), "search request: %v", req)
		}
	})
}

// TestDeepQLCompleteBlock tests basic deepql tag matching conditions and
// aligns with the feature set and testing of the tags search
func TestDeepQLCompleteBlock(t *testing.T) {
	for _, v := range encoding.AllEncodings() {
		vers := v.Version()
		t.Run(vers, func(t *testing.T) {
			testDeepQLCompleteBlock(t, vers)
		})
	}
}

func testDeepQLCompleteBlock(t *testing.T, blockVersion string) {
	e := deepql.NewEngine()

	runCompleteBlockSearchTest(t, blockVersion, func(_ *deeptp.Snapshot, wantMeta *deeppb.SnapshotSearchMetadata, searchesThatMatch, searchesThatDontMatch []*deeppb.SearchRequest, meta *backend.BlockMeta, r Reader) {
		ctx := context.Background()

		for _, req := range searchesThatMatch {
			fetcher := deepql.NewSnapshotResultFetcherWrapper(func(ctx context.Context, req deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
				return r.Fetch(ctx, meta, req, common.DefaultSearchOptions())
			})

			res, err := e.Execute(ctx, req, fetcher)
			require.NoError(t, err, "search request: %+v", req)
			actual := actualForExpectedMeta(wantMeta, res)
			require.NotNil(t, actual, "search request: %v", req)
			require.Equal(t, wantMeta, actual, "search request: %v", req)
		}

		for _, req := range searchesThatDontMatch {
			fetcher := deepql.NewSnapshotResultFetcherWrapper(func(ctx context.Context, req deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
				return r.Fetch(ctx, meta, req, common.DefaultSearchOptions())
			})

			res, err := e.Execute(ctx, req, fetcher)
			require.NoError(t, err, "search request: %+v", req)
			require.Nil(t, actualForExpectedMeta(wantMeta, res), "search request: %v", req)
		}
	})
}

// TestAdvancedDeepQLCompleteBlock uses the actual snapshot data to construct complex deepql queries
// it is supposed to cover all major deepql features. if you see one missing add it!
func TestAdvancedDeepQLCompleteBlock(t *testing.T) {
	for _, v := range encoding.AllEncodings() {
		vers := v.Version()
		t.Run(vers, func(t *testing.T) {
			testAdvancedDeepQLCompleteBlock(t, vers)
		})
	}
}

func testAdvancedDeepQLCompleteBlock(t *testing.T, blockVersion string) {
	e := deepql.NewEngine()

	runCompleteBlockSearchTest(t, blockVersion, func(wantedSnapshot *deeptp.Snapshot, wantMeta *deeppb.SnapshotSearchMetadata, _, _ []*deeppb.SearchRequest, meta *backend.BlockMeta, r Reader) {
		ctx := context.Background()

		// collect some info about wantTr to use below
		trueConditions := [][]string{}
		falseConditions := []string{
			fmt.Sprintf(".name=`%v`", test.RandomString()),
			fmt.Sprintf("duration>%dh", rand.Intn(10)+1),
		}
		totalSpans := 0
		trueAttrC, falseAttrC := conditionsForAttributes(wantedSnapshot.Attributes, "")
		falseConditions = append(falseConditions, falseAttrC...)
		trueConditions = append(trueConditions, trueAttrC)
		trueResourceC, falseResourceC := conditionsForAttributes(wantedSnapshot.Resource, "")
		falseConditions = append(falseConditions, falseResourceC...)
		trueConditions = append(trueConditions, trueResourceC)

		rando := func(s []string) string {
			return s[rand.Intn(len(s))]
		}

		searchesThatMatch := []*deeppb.SearchRequest{
			// conditions
			{Query: fmt.Sprintf("{%s && %s && %s && %s && %s}", rando(trueConditions[0]), rando(trueConditions[0]), rando(trueConditions[0]), rando(trueConditions[0]), rando(trueConditions[0]))},
			{Query: fmt.Sprintf("{%s || %s || %s || %s || %s}", rando(falseConditions), rando(falseConditions), rando(falseConditions), rando(trueConditions[0]), rando(falseConditions))},
			{Query: fmt.Sprintf("{(%s && %s) || %s}", rando(falseConditions), rando(falseConditions), rando(trueConditions[0]))},
			// snapshots
			{Query: fmt.Sprintf("{%s} && {%s}", rando(trueConditions[0]), rando(trueConditions[1]))},
			{Query: fmt.Sprintf("{%s} || {%s}", rando(trueConditions[0]), rando(falseConditions))},
			{Query: fmt.Sprintf("{%s} && {%s} && {%s} && {%s} && {%s}", rando(trueConditions[0]), rando(trueConditions[0]), rando(trueConditions[0]), rando(trueConditions[0]), rando(trueConditions[0]))},
			{Query: fmt.Sprintf("{%s} || {%s} || {%s} || {%s} || {%s}", rando(falseConditions), rando(falseConditions), rando(falseConditions), rando(trueConditions[0]), rando(falseConditions))},
			{Query: fmt.Sprintf("{%s && %s} || {%s}", rando(falseConditions), rando(falseConditions), rando(trueConditions[0]))},
			// pipelines
			{Query: fmt.Sprintf("{%s} | {%s}", rando(trueConditions[0]), rando(trueConditions[0]))},
			{Query: fmt.Sprintf("{%s || %s} | {%s}", rando(falseConditions), rando(trueConditions[0]), rando(trueConditions[0]))},
			// pipeline expressions
			{Query: fmt.Sprintf("({%s} | count() > 0) && ({%s} | count() > 0)", rando(trueConditions[0]), rando(trueConditions[1]))},
			{Query: fmt.Sprintf("({%s} | count() > 0) || ({%s} | count() > 0)", rando(trueConditions[0]), rando(falseConditions))},
			// counts
			{Query: fmt.Sprintf("{} | count() = %d", totalSpans)},
			{Query: fmt.Sprintf("{} | count() != %d", totalSpans+1)},
			{Query: fmt.Sprintf("{ true } && { true } | count() = %d", totalSpans)},
			{Query: fmt.Sprintf("{ true } || { true } | count() = %d", totalSpans)},
			{Query: fmt.Sprintf("{ %s && %s } | count() = 1", rando(trueConditions[0]), rando(trueConditions[0]))},
			// avgs/min/max/sum
			//{Query: fmt.Sprintf("{ %s && %s } && { %s && %s } | avg(duration) = %dns",
			//	rando(trueConditions[0]), rando(trueConditions[0]),
			//	rando(trueConditions[1]), rando(trueConditions[1]),
			//	(durationBySpan[0]+durationBySpan[1])/2)},
			//{Query: fmt.Sprintf("{ %s && %s } && { %s && %s } | min(duration) = %dns",
			//	rando(trueConditions[0]), rando(trueConditions[0]),
			//	rando(trueConditions[1]), rando(trueConditions[1]),
			//	math.Min64(int64(durationBySpan[0]), int64(durationBySpan[1])))},
			//{Query: fmt.Sprintf("{ %s && %s } && { %s && %s } | max(duration) = %dns",
			//	rando(trueConditions[0]), rando(trueConditions[0]),
			//	rando(trueConditions[1]), rando(trueConditions[1]),
			//	math.Max64(int64(durationBySpan[0]), int64(durationBySpan[1])))},
			//{Query: fmt.Sprintf("{ %s && %s } && { %s && %s } | sum(duration) = %dns",
			//	rando(trueConditions[0]), rando(trueConditions[0]),
			//	rando(trueConditions[1]), rando(trueConditions[1]),
			//	durationBySpan[0]+durationBySpan[1])},
		}
		searchesThatDontMatch := []*deeppb.SearchRequest{
			{Query: "{duration>=9h || .line=9}"},
			{Query: "{.id=`28ab414c9f0d34f39d4ba28442215d14`}"},
			{Query: "{.id=`28ab414c9f0d34f39d4ba28442215d14` || duration>=9h}"},
			{Query: "{.line=9}"},
			{Query: "{.path=`VgWUyqEfOK`}"},
			{Query: "{.path=`VgWUyqEfOK` || .line=9}"},
			//// conditions
			{Query: fmt.Sprintf("{%s && %s}", rando(trueConditions[0]), rando(falseConditions))},
			{Query: fmt.Sprintf("{%s || %s}", rando(falseConditions), rando(falseConditions))},
			//{Query: fmt.Sprintf("{%s && (%s || %s)}", rando(falseConditions), rando(falseConditions), rando(trueConditions[0]))},
			//// snapshots
			//{Query: fmt.Sprintf("{%s} && {%s}", rando(trueConditions[0]), rando(falseConditions))},
			//{Query: fmt.Sprintf("{%s} || {%s}", rando(falseConditions), rando(falseConditions))},
			//{Query: fmt.Sprintf("{%s && %s} || {%s}", rando(falseConditions), rando(falseConditions), rando(falseConditions))},
			//// pipelines
			//{Query: fmt.Sprintf("{%s} | {%s}", rando(trueConditions[0]), rando(falseConditions))},
			//{Query: fmt.Sprintf("{%s} | {%s}", rando(falseConditions), rando(trueConditions[0]))},
			//{Query: fmt.Sprintf("{%s || %s} | {%s}", rando(falseConditions), rando(trueConditions[0]), rando(falseConditions))},
			//// pipeline expressions
			//{Query: fmt.Sprintf("({%s} | count() > 0) && ({%s} | count() > 0)", rando(trueConditions[0]), rando(falseConditions))},
			//{Query: fmt.Sprintf("({%s} | count() > 0) || ({%s} | count() > 0)", rando(falseConditions), rando(falseConditions))},
			//// counts
			//{Query: fmt.Sprintf("{} | count() = %d", totalSpans+1)},
			//{Query: fmt.Sprintf("{} | count() != %d", totalSpans)},
			//{Query: fmt.Sprintf("{} | count() < %d", totalSpans)},
			//{Query: fmt.Sprintf("{} | count() > %d", totalSpans)},
			//// avgs
			//{Query: "{ } | avg(.dne) != 0"},
			//{Query: "{ } | avg(duration) < 0"},
			//{Query: "{ } | min(duration) < 0"},
			//{Query: "{ } | max(duration) < 0"},
			//{Query: "{ } | sum(duration) < 0"},
		}

		for _, req := range searchesThatMatch {
			fetcher := deepql.NewSnapshotResultFetcherWrapper(func(ctx context.Context, req deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
				return r.Fetch(ctx, meta, req, common.DefaultSearchOptions())
			})

			res, err := e.Execute(ctx, req, fetcher)
			require.NoError(t, err, "search request: %+v", req)
			actual := actualForExpectedMeta(wantMeta, res)
			require.NotNil(t, actual, "search request: %v", req)
			require.Equal(t, wantMeta, actual, "search request: %v", req)
		}

		for _, req := range searchesThatDontMatch {
			fetcher := deepql.NewSnapshotResultFetcherWrapper(func(ctx context.Context, req deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
				return r.Fetch(ctx, meta, req, common.DefaultSearchOptions())
			})

			res, err := e.Execute(ctx, req, fetcher)
			require.NoError(t, err, "search request: %+v", req)
			require.Nil(t, actualForExpectedMeta(wantMeta, res), "search request: %v", req)
		}
	})
}

func conditionsForAttributes(atts []*v1_common.KeyValue, scope string) ([]string, []string) {
	trueConditions := []string{}
	falseConditions := []string{}

	for _, a := range atts {
		switch v := a.GetValue().Value.(type) {
		case *v1_common.AnyValue_StringValue:
			trueConditions = append(trueConditions, fmt.Sprintf("%s.%v=`%v`", scope, a.Key, v.StringValue))
			trueConditions = append(trueConditions, fmt.Sprintf(".%v=`%v`", a.Key, v.StringValue))
			falseConditions = append(falseConditions, fmt.Sprintf("%s.%v=`%v`", scope, a.Key, test.RandomString()))
			falseConditions = append(falseConditions, fmt.Sprintf(".%v=`%v`", a.Key, test.RandomString()))
		case *v1_common.AnyValue_BoolValue:
			trueConditions = append(trueConditions, fmt.Sprintf("%s.%v=%t", scope, a.Key, v.BoolValue))
			trueConditions = append(trueConditions, fmt.Sprintf(".%v=%t", a.Key, v.BoolValue))
			// tough to add an always false condition here
		case *v1_common.AnyValue_IntValue:
			trueConditions = append(trueConditions, fmt.Sprintf("%s.%v=%d", scope, a.Key, v.IntValue))
			trueConditions = append(trueConditions, fmt.Sprintf(".%v=%d", a.Key, v.IntValue))
			falseConditions = append(falseConditions, fmt.Sprintf("%s.%v=%d", scope, a.Key, rand.Intn(1000)+20000))
			falseConditions = append(falseConditions, fmt.Sprintf(".%v=%d", a.Key, rand.Intn(1000)+20000))
		case *v1_common.AnyValue_DoubleValue:
			trueConditions = append(trueConditions, fmt.Sprintf("%s.%v=%f", scope, a.Key, v.DoubleValue))
			trueConditions = append(trueConditions, fmt.Sprintf(".%v=%f", a.Key, v.DoubleValue))
			falseConditions = append(falseConditions, fmt.Sprintf("%s.%v=%f", scope, a.Key, rand.Float64()))
			falseConditions = append(falseConditions, fmt.Sprintf(".%v=%f", a.Key, rand.Float64()))
		}
	}

	return trueConditions, falseConditions
}

func actualForExpectedMeta(wantMeta *deeppb.SnapshotSearchMetadata, res *deeppb.SearchResponse) *deeppb.SnapshotSearchMetadata {
	// find wantMeta in res
	for _, snapshot := range res.Snapshots {
		if snapshot.SnapshotID == wantMeta.SnapshotID {
			return snapshot
		}
	}

	return nil
}

type runnerFn func(*deeptp.Snapshot, *deeppb.SnapshotSearchMetadata, []*deeppb.SearchRequest, []*deeppb.SearchRequest, *backend.BlockMeta, Reader)

func runCompleteBlockSearchTest(t testing.TB, blockVersion string, runner runnerFn) {
	tempDir := t.TempDir()

	r, w, _, _, c, err := New(&Config{
		Backend: "local",
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             blockVersion,
			RowGroupSizeBytes:   10000,
		},
		WAL: &wal.Config{
			Filepath:       path.Join(tempDir, "wal"),
			IngestionSlack: time.Since(time.Time{}),
		},
		Search: &SearchConfig{
			ChunkSizeBytes:      1_000_000,
			ReadBufferCount:     8,
			ReadBufferSizeBytes: 4 * 1024 * 1024,
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	require.NoError(t, err)

	c.EnableCompaction(context.Background(), &CompactorConfig{
		ChunkSizeBytes:          10,
		MaxCompactionRange:      time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})
	rw := r.(*readerWriter)

	wantID, wantTr, start, _, wantMeta, searchesThatMatch, searchesThatDontMatch := searchTestSuite()

	// Write to wal
	walBlocks := w.WAL()
	head, err := walBlocks.NewBlock(uuid.New(), testTenantID, model.CurrentEncoding)
	require.NoError(t, err)
	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	totalSnapshots := 250
	wantTrIdx := rand.Intn(250)
	for i := 0; i < totalSnapshots; i++ {
		var tr *deeptp.Snapshot
		var id []byte
		if i == wantTrIdx {
			tr = wantTr
			id = wantID
		} else {
			id = test.ValidSnapshotID(nil)
			tr = test.GenerateSnapshot(i, &test.GenerateOptions{Id: id})
			// tr = nil
		}
		if tr == nil {
			continue
		}
		b1, err := dec.PrepareForWrite(tr, start)
		require.NoError(t, err)

		b2, err := dec.ToObject(b1)
		require.NoError(t, err)
		err = head.Append(id, b2, start)
		require.NoError(t, err)
	}

	// Complete block
	block, err := w.CompleteBlock(context.Background(), head)
	require.NoError(t, err)
	meta := block.BlockMeta()

	runner(wantTr, wantMeta, searchesThatMatch, searchesThatDontMatch, meta, rw)

	// todo: do some compaction and then call runner again
}

func stringKV(k, v string) *v1_common.KeyValue {
	return &v1_common.KeyValue{
		Key:   k,
		Value: &v1_common.AnyValue{Value: &v1_common.AnyValue_StringValue{StringValue: v}},
	}
}

func intKV(k string, v int) *v1_common.KeyValue {
	return &v1_common.KeyValue{
		Key:   k,
		Value: &v1_common.AnyValue{Value: &v1_common.AnyValue_IntValue{IntValue: int64(v)}},
	}
}

// Helper function to make a tag search
func makeReq(k, v string) *deeppb.SearchRequest {
	return &deeppb.SearchRequest{
		Tags: map[string]string{
			k: v,
		},
	}
}

func addDeepQL(req *deeppb.SearchRequest) {
	// todo: deepql concepts are different than search concepts. this code maps key/value pairs
	// from search to deepql. we can clean this up after we drop old search and move these tests into
	// the deepdb package.
	deepqlConditions := []string{}
	for k, v := range req.Tags {
		deepqlKey := k
		switch deepqlKey {
		case "root.service.name":
			deepqlKey = ".service.name"
		default:
			deepqlKey = "." + deepqlKey
		}

		deepqlVal := v
		switch deepqlKey {
		default:
			deepqlVal = fmt.Sprintf(`"%s"`, v)
		}
		deepqlConditions = append(deepqlConditions, fmt.Sprintf("%s=%s", deepqlKey, deepqlVal))
	}
	if req.MaxDurationMs != 0 {
		deepqlConditions = append(deepqlConditions, fmt.Sprintf("duration < %dms", req.MaxDurationMs))
	}
	if req.MinDurationMs != 0 {
		deepqlConditions = append(deepqlConditions, fmt.Sprintf("duration > %dms", req.MinDurationMs))
	}

	req.Query = "{" + strings.Join(deepqlConditions, "&&") + "}"
}

// searchTestSuite returns a set of search test cases that ensure
// search behavior is consistent across block types and modules.
// The return parameters are:
//   - snapshot ID
//   - snapshot - a fully-populated snapshot that is searched for every condition. If testing a
//     block format, then write this snapshot to the block.
//   - start, end - the unix second start/end times for the snapshot, i.e. slack-adjusted timestamps
//   - expected - The exact search result that should be returned for every matching request
//   - searchesThatMatch - List of search requests that are expected to match the snapshot
//   - searchesThatDontMatch - List of requests that don't match the snapshot
func searchTestSuite() (
	id []byte,
	tr *deeptp.Snapshot,
	start, end uint32,
	expected *deeppb.SnapshotSearchMetadata,
	searchesThatMatch []*deeppb.SearchRequest,
	searchesThatDontMatch []*deeppb.SearchRequest,
) {
	id = test.ValidSnapshotID(nil)

	start = 1000
	end = 1001

	tr = test.GenerateSnapshot(1, &test.GenerateOptions{Id: id})
	tr.TsNanos = 1500
	tr.DurationNanos = 1500 * 1000000

	expected = &deeppb.SnapshotSearchMetadata{
		SnapshotID:        util.SnapshotIDToHexString(id),
		ServiceName:       "deep-cli",
		FilePath:          tr.Tracepoint.Path,
		LineNo:            tr.Tracepoint.LineNumber,
		StartTimeUnixNano: tr.TsNanos,
		DurationNano:      tr.DurationNanos,
	}

	// Matches
	searchesThatMatch = []*deeppb.SearchRequest{
		{
			// Empty request
		},
		{
			MinDurationMs: 999,
			MaxDurationMs: 2001,
		},
		//{
		//	Start: 1000,
		//	End:   2000,
		//},
		//{
		//	// Overlaps start
		//	Start: 999,
		//	End:   1001,
		//},
		//{
		//	// Overlaps end
		//	Start: 1001,
		//	End:   1002,
		//},

		// Well-known resource attributes
		makeReq("frame", "single_frame"),
		makeReq("service.name", "deep-cli"),
		// makeReq("cluster", "MyCluster"),
		// makeReq("namespace", "MyNamespace"),
		// makeReq("pod", "MyPod"),
		// makeReq("container", "MyContainer"),
		// makeReq("k8s.cluster.name", "k8sCluster"),
		// makeReq("k8s.namespace.name", "k8sNamespace"),
		// makeReq("k8s.pod.name", "k8sPod"),
		// makeReq("k8s.container.name", "k8sContainer"),
		// makeReq("root.service.name", "RootService"),
		// makeReq("root.name", "RootSpan"),

		// Attributes
		// makeReq("foo", "Bar"),
		// Resource attributes
		// makeReq("bat", "Baz"),

		// Multiple
		//{
		//	Tags: map[string]string{
		//		"service.name": "MyService",
		//		"http.method":  "Get",
		//		"foo":          "Bar",
		//	},
		//},
	}

	// Excludes
	searchesThatDontMatch = []*deeppb.SearchRequest{
		{
			MinDurationMs: 2001,
		},
		{
			MaxDurationMs: 999,
		},
		{
			Start: 100,
			End:   200,
		},

		// Well-known resource attributes
		makeReq("service.name", "service"), // wrong case
		makeReq("cluster", "cluster"),      // wrong case
		makeReq("namespace", "namespace"),  // wrong case
		makeReq("pod", "pod"),              // wrong case
		makeReq("container", "container"),  // wrong case

		// Well-known attributes
		makeReq("http.method", "post"),
		makeReq("http.url", "asdf"),
		makeReq("http.status_code", "200"),
		makeReq("status.code", "ok"),
		makeReq("root.service.name", "NotRootService"),
		makeReq("root.name", "NotRootSpan"),

		// Attributes
		makeReq("foo", "baz"), // wrong case
	}

	// add deepql to all searches
	for _, req := range searchesThatDontMatch {
		addDeepQL(req)
	}
	for _, req := range searchesThatMatch {
		addDeepQL(req)
	}

	return
}
