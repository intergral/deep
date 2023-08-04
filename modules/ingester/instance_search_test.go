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

package ingester

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"fmt"
	"github.com/google/uuid"
	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/pkg/deeppb"
	v1 "github.com/intergral/deep/pkg/deeppb/common/v1"
	"github.com/intergral/deep/pkg/model"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestInstanceSearch(t *testing.T) {
	i, ingester, tempDir := defaultInstanceAndTmpDir(t)

	var tagKey = "foo"
	var tagValue = "bar"
	ids, _ := writeSnapshotsForSearch(t, i, tagKey, tagValue, false)

	var req = &deeppb.SearchRequest{
		Tags: map[string]string{},
	}
	req.Tags[tagKey] = tagValue
	req.Limit = uint32(len(ids)) + 1

	// Test after appending to WAL. writeSnapshotsForSearch() makes sure all snapshots are in the wal
	sr, err := i.Search(context.Background(), req)
	assert.NoError(t, err)
	assert.Len(t, sr.Snapshots, len(ids))
	checkEqual(t, ids, sr)

	// Test after cutting new headblock
	blockID, err := i.CutBlockIfReady(0, 0, true)
	require.NoError(t, err)
	assert.NotEqual(t, blockID, uuid.Nil)

	sr, err = i.Search(context.Background(), req)
	assert.NoError(t, err)
	assert.Len(t, sr.Snapshots, len(ids))
	checkEqual(t, ids, sr)

	// Test after completing a block
	err = i.CompleteBlock(blockID)
	require.NoError(t, err)

	sr, err = i.Search(context.Background(), req)
	assert.NoError(t, err)
	assert.Len(t, sr.Snapshots, len(ids))
	checkEqual(t, ids, sr)

	err = ingester.stopping(nil)
	require.NoError(t, err)

	// create new ingester.  this should replay wal!
	ingester, _, _ = defaultIngester(t, tempDir)

	i, ok := ingester.getInstanceByID("fake")
	require.True(t, ok)

	sr, err = i.Search(context.Background(), req)
	assert.NoError(t, err)
	assert.Len(t, sr.Snapshots, len(ids))
	checkEqual(t, ids, sr)

	err = ingester.stopping(nil)
	require.NoError(t, err)
}

// TestInstanceSearchDeepQL is duplicate of TestInstanceSearch for now
func TestInstanceSearchDeepQL(t *testing.T) {
	queries := []string{
		`{ .service.name = "test-service" }`,
		`{ duration >= 1s }`,
		`{ duration >= 1s && .service.name = "test-service" }`,
	}

	for _, query := range queries {
		t.Run(fmt.Sprintf("Query:%s", query), func(t *testing.T) {
			i, ingester, tmpDir := defaultInstanceAndTmpDir(t)
			// pushSnapshotsToInstance creates snapshots with:
			// `service.name = "test-service"` and duration >= 1s
			_, ids := pushSnapshotsToInstance(t, i, 10)

			req := &deeppb.SearchRequest{Query: query, Limit: 20}

			// Test live snapshots
			sr, err := i.Search(context.Background(), req)
			assert.NoError(t, err)
			assert.Len(t, sr.Snapshots, 0)

			// Test after appending to WAL
			require.NoError(t, i.CutSnapshots(0, true))
			assert.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))

			sr, err = i.Search(context.Background(), req)
			assert.NoError(t, err)
			assert.Len(t, sr.Snapshots, len(ids))
			checkEqual(t, ids, sr)

			// Test after cutting new headBlock
			blockID, err := i.CutBlockIfReady(0, 0, true)
			require.NoError(t, err)
			assert.NotEqual(t, blockID, uuid.Nil)

			sr, err = i.Search(context.Background(), req)
			assert.NoError(t, err)
			assert.Len(t, sr.Snapshots, len(ids))
			checkEqual(t, ids, sr)

			// Test after completing a block
			err = i.CompleteBlock(blockID)
			require.NoError(t, err)

			sr, err = i.Search(context.Background(), req)
			assert.NoError(t, err)
			assert.Len(t, sr.Snapshots, len(ids))
			checkEqual(t, ids, sr)

			// Test after clearing the completing block
			err = i.ClearCompletingBlock(blockID)
			require.NoError(t, err)

			sr, err = i.Search(context.Background(), req)
			assert.NoError(t, err)
			assert.Len(t, sr.Snapshots, len(ids))
			checkEqual(t, ids, sr)

			err = ingester.stopping(nil)
			require.NoError(t, err)

			// create new ingester.  this should replay wal!
			ingester, _, _ = defaultIngester(t, tmpDir)

			i, ok := ingester.getInstanceByID("fake")
			require.True(t, ok)

			sr, err = i.Search(context.Background(), req)
			assert.NoError(t, err)
			assert.Len(t, sr.Snapshots, len(ids))
			checkEqual(t, ids, sr)

			err = ingester.stopping(nil)
			require.NoError(t, err)
		})
	}
}

func checkEqual(t *testing.T, ids [][]byte, sr *deeppb.SearchResponse) {
	for _, meta := range sr.Snapshots {
		snapshotID, err := util.HexStringToSnapshotID(meta.SnapshotID)
		assert.NoError(t, err)

		present := false
		for _, id := range ids {
			if bytes.Equal(snapshotID, id) {
				present = true
			}
		}
		assert.True(t, present)
	}
}

func TestInstanceSearchTags(t *testing.T) {
	i, _ := defaultInstance(t)

	// add dummy search data
	var tagKey = "foo"
	var tagValue = "bar"

	_, expectedTagValues := writeSnapshotsForSearch(t, i, tagKey, tagValue, true)

	userCtx := util.InjectTenantID(context.Background(), "fake")

	// Test after appending to WAL
	testSearchTagsAndValues(t, userCtx, i, tagKey, expectedTagValues)

	// Test after cutting new headblock
	blockID, err := i.CutBlockIfReady(0, 0, true)
	require.NoError(t, err)
	assert.NotEqual(t, blockID, uuid.Nil)

	testSearchTagsAndValues(t, userCtx, i, tagKey, expectedTagValues)

	// Test after completing a block
	err = i.CompleteBlock(blockID)
	require.NoError(t, err)

	testSearchTagsAndValues(t, userCtx, i, tagKey, expectedTagValues)
}

// nolint:revive,unparam
func testSearchTagsAndValues(t *testing.T, ctx context.Context, i *tenantBlockManager, tagName string, expectedTagValues []string) {
	sr, err := i.SearchTags(ctx)
	require.NoError(t, err)
	srv, err := i.SearchTagValues(ctx, tagName)
	require.NoError(t, err)

	sort.Strings(srv.TagValues)
	sort.Strings(expectedTagValues)
	assert.Contains(t, sr.TagNames, tagName)
	assert.Equal(t, expectedTagValues, srv.TagValues)
}

// TestInstanceSearchMaxBytesPerTagValuesQueryReturnsPartial confirms that SearchTagValues returns
// partial results if the bytes of the found tag value exceeds the MaxBytesPerTagValuesQuery limit
func TestInstanceSearchMaxBytesPerTagValuesQueryReturnsPartial(t *testing.T) {
	limits, err := overrides.NewOverrides(overrides.Limits{
		MaxBytesPerTagValuesQuery: 10,
	})
	assert.NoError(t, err, "unexpected error creating limits")
	limiter := NewLimiter(limits, &ringCountMock{count: 1}, 1)

	tempDir := t.TempDir()

	ingester, _, _ := defaultIngester(t, tempDir)
	ingester.limiter = limiter
	i, err := ingester.getOrCreateInstance("fake")
	assert.NoError(t, err, "unexpected error creating new tenantBlockManager")

	var tagKey = "foo"
	var tagValue = "bar"

	_, _ = writeSnapshotsForSearch(t, i, tagKey, tagValue, true)

	userCtx := util.InjectTenantID(context.Background(), "fake")
	resp, err := i.SearchTagValues(userCtx, tagKey)
	require.NoError(t, err)
	require.Equal(t, 2, len(resp.TagValues)) // Only two values of the form "bar123" fit in the 10 byte limit above.
}

// writes snapshots to the given tenantBlockManager along with search data. returns
// ids expected to be returned from a tag search and strings expected to
// be returned from a tag value search
// nolint:revive,unparam
func writeSnapshotsForSearch(t *testing.T, i *tenantBlockManager, tagKey string, tagValue string, postFixValue bool) ([][]byte, []string) {
	// This matches the encoding for live snapshots, since
	// we are pushing to the tenantBlockManager directly it must match.
	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	numSnapshots := 100
	ids := [][]byte{}
	expectedTagValues := []string{}

	for j := 0; j < numSnapshots; j++ {
		id := make([]byte, 16)
		_, err := crand.Read(id)
		require.NoError(t, err)

		tv := tagValue
		if postFixValue {
			tv = tv + strconv.Itoa(j)
		}
		kv := &v1.KeyValue{Key: tagKey, Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: tv}}}
		expectedTagValues = append(expectedTagValues, tv)
		ids = append(ids, id)

		snapshot := test.GenerateSnapshot(10, &test.GenerateOptions{Id: id})
		snapshot.Attributes = append(snapshot.Attributes, kv)

		snapshotBytes, err := dec.PrepareForWrite(snapshot, 0)
		require.NoError(t, err)

		// searchData will be nil if not
		err = i.PushBytes(context.Background(), id, snapshotBytes)
		require.NoError(t, err)

		assert.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))
	}

	// snapshots have to be cut to show up in searches
	err := i.CutSnapshots(0, true)
	require.NoError(t, err)

	return ids, expectedTagValues
}

func TestInstanceSearchNoData(t *testing.T) {
	i, _ := defaultInstance(t)

	var req = &deeppb.SearchRequest{
		Tags: map[string]string{},
	}

	sr, err := i.Search(context.Background(), req)
	assert.NoError(t, err)
	require.Len(t, sr.Snapshots, 0)
}

func TestInstanceSearchDoesNotRace(t *testing.T) {
	ingester, _, _ := defaultIngester(t, t.TempDir())
	i, err := ingester.getOrCreateInstance("fake")
	require.NoError(t, err)

	// This matches the encoding for live snapshots, since
	// we are pushing to the tenantBlockManager directly it must match.
	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	// add dummy search data
	var tagKey = "foo"
	var tagValue = "bar"

	var req = &deeppb.SearchRequest{
		Tags: map[string]string{tagKey: tagValue},
	}

	end := make(chan struct{})

	concurrent := func(f func()) {
		for {
			select {
			case <-end:
				return
			default:
				f()
			}
		}
	}

	go concurrent(func() {
		// more than 10k exceeeds live snapshots
		id := make([]byte, 16)
		_, err := crand.Read(id)
		require.NoError(t, err)

		snapshot := test.GenerateSnapshot(10, &test.GenerateOptions{Id: id})
		snapshotBytes, err := dec.PrepareForWrite(snapshot, 0)
		require.NoError(t, err)

		if len(i.liveSnapshots) < 9000 {
			// searchData will be nil if not
			err = i.PushBytes(context.Background(), id, snapshotBytes)
			assert.NoError(t, err)
		}
	})

	go concurrent(func() {
		err := i.CutSnapshots(0, true)
		require.NoError(t, err, "error cutting complete snapshots")
	})

	go concurrent(func() {
		_, err := i.FindSnapshotByID(context.Background(), []byte{0x01})
		assert.NoError(t, err, "error finding snapshot by id")
	})

	go concurrent(func() {
		// Cut wal, complete, delete wal, then flush
		blockID, _ := i.CutBlockIfReady(0, 0, true)
		if blockID != uuid.Nil {
			err := i.CompleteBlock(blockID)
			fmt.Println("complete block", blockID, err)
			require.NoError(t, err)
			err = i.ClearCompletingBlock(blockID)
			require.NoError(t, err)
			block := i.GetBlockToBeFlushed(blockID)
			require.NotNil(t, block)
			err = ingester.store.WriteBlock(context.Background(), block)
			require.NoError(t, err)
		}
	})

	go concurrent(func() {
		err = i.ClearFlushedBlocks(0)
		require.NoError(t, err)
	})

	go concurrent(func() {
		_, err := i.Search(context.Background(), req)
		require.NoError(t, err, "error finding snapshot by id")
	})

	go concurrent(func() {
		// SearchTags queries now require userID in ctx
		ctx := util.InjectTenantID(context.Background(), "test")
		_, err := i.SearchTags(ctx)
		require.NoError(t, err, "error getting search tags")
	})

	go concurrent(func() {
		// SearchTagValues queries now require userID in ctx
		ctx := util.InjectTenantID(context.Background(), "test")
		_, err := i.SearchTagValues(ctx, tagKey)
		require.NoError(t, err, "error getting search tag values")
	})

	time.Sleep(2000 * time.Millisecond)
	close(end)
	// Wait for go funcs to quit before
	// exiting and cleaning up
	time.Sleep(2 * time.Second)
}

func TestWALBlockDeletedDuringSearch(t *testing.T) {
	i, _ := defaultInstance(t)

	// This matches the encoding for live snapshots, since
	// we are pushing to the tenantBlockManager directly it must match.
	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	end := make(chan struct{})

	concurrent := func(f func()) {
		for {
			select {
			case <-end:
				return
			default:
				f()
			}
		}
	}

	for j := 0; j < 500; j++ {
		id := make([]byte, 16)
		_, err := crand.Read(id)
		require.NoError(t, err)

		snapshot := test.GenerateSnapshot(10, &test.GenerateOptions{Id: id})
		snapshotBytes, err := dec.PrepareForWrite(snapshot, 0)
		require.NoError(t, err)

		err = i.PushBytes(context.Background(), id, snapshotBytes)
		require.NoError(t, err)
	}

	err := i.CutSnapshots(0, true)
	require.NoError(t, err)

	blockID, err := i.CutBlockIfReady(0, 0, true)
	require.NoError(t, err)

	go concurrent(func() {
		_, err := i.Search(context.Background(), &deeppb.SearchRequest{
			Tags: map[string]string{
				// Not present in the data, so it will be an exhaustive
				// search
				"wuv": "xyz",
			},
		})
		require.NoError(t, err)
	})

	// Let search get going
	time.Sleep(100 * time.Millisecond)

	err = i.ClearCompletingBlock(blockID)
	require.NoError(t, err)

	// Wait for go funcs to quit before
	// exiting and cleaning up
	close(end)
	time.Sleep(2 * time.Second)
}

func TestInstanceSearchMetrics(t *testing.T) {
	i, _ := defaultInstance(t)

	// This matches the encoding for live snapshots, since
	// we are pushing to the tenantBlockManager directly it must match.
	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	numSnapshots := uint32(500)
	numBytes := uint64(0)
	for j := uint32(0); j < numSnapshots; j++ {
		id := test.ValidSnapshotID(nil)

		// Snapshot bytes have to be pushed in the expected data encoding
		snapshot := test.GenerateSnapshot(10, &test.GenerateOptions{Id: id})

		snapshotBytes, err := dec.PrepareForWrite(snapshot, 0)
		require.NoError(t, err)

		err = i.PushBytes(context.Background(), id, snapshotBytes)
		require.NoError(t, err)

		assert.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))
	}

	search := func() *deeppb.SearchMetrics {
		sr, err := i.Search(context.Background(), &deeppb.SearchRequest{
			Tags: map[string]string{"foo": "bar"},
		})
		require.NoError(t, err)
		return sr.Metrics
	}

	// Live snapshots
	m := search()
	require.Equal(t, uint32(0), m.InspectedSnapshots) // we don't search live snapshots
	require.Equal(t, uint64(0), m.InspectedBytes)     // we don't search live snapshots
	require.Equal(t, uint32(1), m.InspectedBlocks)    // 1 head block

	// Test after appending to WAL
	err := i.CutSnapshots(0, true)
	require.NoError(t, err)
	m = search()
	require.Equal(t, numSnapshots, m.InspectedSnapshots)
	require.Less(t, numBytes, m.InspectedBytes)
	require.Equal(t, uint32(1), m.InspectedBlocks) // 1 head block

	// Test after cutting new headblock
	blockID, err := i.CutBlockIfReady(0, 0, true)
	require.NoError(t, err)
	m = search()
	require.Equal(t, numSnapshots, m.InspectedSnapshots)
	require.Less(t, numBytes, m.InspectedBytes)
	require.Equal(t, uint32(2), m.InspectedBlocks) // 1 head block, 1 completing block

	// Test after completing a block
	err = i.CompleteBlock(blockID)
	require.NoError(t, err)
	err = i.ClearCompletingBlock(blockID)
	require.NoError(t, err)
	m = search()
	require.Equal(t, numSnapshots, m.InspectedSnapshots)
	require.Equal(t, uint32(2), m.InspectedBlocks) // 1 head block, 1 complete block
}

func BenchmarkInstanceSearchUnderLoad(b *testing.B) {
	ctx := context.TODO()

	i, _ := defaultInstance(b)

	// This matches the encoding for live snapshots, since
	// we are pushing to the tenantBlockManager directly it must match.
	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	end := make(chan struct{})

	concurrent := func(f func()) {
		for {
			select {
			case <-end:
				return
			default:
				f()
			}
		}
	}

	// Push data
	var snapshotPushed atomic.Int32
	for j := 0; j < 2; j++ {
		go concurrent(func() {
			id := test.ValidSnapshotID(nil)

			snapshot := test.GenerateSnapshot(10, &test.GenerateOptions{Id: id})
			snapshotBytes, err := dec.PrepareForWrite(snapshot, 0)
			require.NoError(b, err)

			// searchData will be nil if not
			err = i.PushBytes(context.Background(), id, snapshotBytes)
			require.NoError(b, err)

			snapshotPushed.Add(1)
		})
	}

	cuts := 0
	go concurrent(func() {
		time.Sleep(250 * time.Millisecond)
		err := i.CutSnapshots(0, true)
		require.NoError(b, err, "error cutting complete snapshots")
		cuts++
	})

	go concurrent(func() {
		// Slow this down to prevent "too many open files" error
		time.Sleep(100 * time.Millisecond)
		_, err := i.CutBlockIfReady(0, 0, true)
		require.NoError(b, err)
	})

	var searches atomic.Int32
	var bytesInspected atomic.Uint64
	var snapshotsInspected atomic.Uint32

	for j := 0; j < 2; j++ {
		go concurrent(func() {
			// time.Sleep(1 * time.Millisecond)
			var req = &deeppb.SearchRequest{}
			resp, err := i.Search(ctx, req)
			require.NoError(b, err)
			searches.Add(1)
			bytesInspected.Add(resp.Metrics.InspectedBytes)
			snapshotsInspected.Add(resp.Metrics.InspectedSnapshots)
		})
	}

	b.ResetTimer()
	start := time.Now()
	time.Sleep(time.Duration(b.N) * time.Millisecond)
	elapsed := time.Since(start)

	fmt.Printf("Instance search throughput under load: %v elapsed %.2f MB = %.2f MiB/s throughput inspected %.2f snapshots/s pushed %.2f snapshots/s %.2f searches/s %.2f cuts/s\n",
		elapsed,
		float64(bytesInspected.Load())/(1024*1024),
		float64(bytesInspected.Load())/(elapsed.Seconds())/(1024*1024),
		float64(snapshotsInspected.Load())/(elapsed.Seconds()),
		float64(snapshotPushed.Load())/(elapsed.Seconds()),
		float64(searches.Load())/(elapsed.Seconds()),
		float64(cuts)/(elapsed.Seconds()),
	)

	b.StopTimer()
	close(end)
	// Wait for go funcs to quit before
	// exiting and cleaning up
	time.Sleep(1 * time.Second)
}
