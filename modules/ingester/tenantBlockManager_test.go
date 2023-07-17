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

// todo
//import (
//	"context"
//	crand "crypto/rand"
//	"encoding/binary"
//	"github.com/intergral/deep/pkg/deeppb"
//	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
//	"math/rand"
//	"testing"
//	"time"
//
//	"github.com/google/uuid"
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/require"
//	"github.com/weaveworks/common/user"
//
//	"github.com/intergral/deep/modules/overrides"
//	"github.com/intergral/deep/pkg/model"
//	"github.com/intergral/deep/pkg/model/trace"
//	"github.com/intergral/deep/pkg/tempopb"
//	v1_trace "github.com/intergral/deep/pkg/tempopb/trace/v1"
//	"github.com/intergral/deep/pkg/util/test"
//)
//
//const testTenantID = "fake"
//
//type ringCountMock struct {
//	count int
//}
//
//func (m *ringCountMock) HealthyInstancesCount() int {
//	return m.count
//}
//
//func TestInstance(t *testing.T) {
//	request := makeRequest([]byte{})
//
//	i, ingester := defaultInstance(t)
//
//	err := i.PushBytesRequest(context.Background(), request)
//	require.NoError(t, err)
//	require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))
//
//	err = i.CutSnapshots(0, true)
//	require.NoError(t, err)
//	require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))
//
//	blockID, err := i.CutBlockIfReady(0, 0, false)
//	require.NoError(t, err, "unexpected error cutting block")
//	require.NotEqual(t, blockID, uuid.Nil)
//
//	err = i.CompleteBlock(blockID)
//	require.NoError(t, err, "unexpected error completing block")
//
//	block := i.GetBlockToBeFlushed(blockID)
//	require.NotNil(t, block)
//	require.Len(t, i.completingBlocks, 1)
//	require.Len(t, i.completeBlocks, 1)
//
//	err = ingester.store.WriteBlock(context.Background(), block)
//	require.NoError(t, err)
//
//	err = i.ClearFlushedBlocks(30 * time.Hour)
//	require.NoError(t, err)
//	require.Len(t, i.completeBlocks, 1)
//
//	err = i.ClearFlushedBlocks(0)
//	require.NoError(t, err)
//	require.Len(t, i.completeBlocks, 0)
//
//	err = i.resetHeadBlock()
//	require.NoError(t, err, "unexpected error resetting block")
//
//	require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))
//}
//
//func TestInstanceFind(t *testing.T) {
//	i, ingester := defaultInstance(t)
//
//	numTraces := 10
//	traces, ids := pushTracesToInstance(t, i, numTraces)
//
//	queryAll(t, i, ids, traces)
//
//	err := i.CutSnapshots(0, true)
//	require.NoError(t, err)
//	require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))
//
//	for j := 0; j < numTraces; j++ {
//		traceBytes, err := model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForWrite(traces[j], 0, 0)
//		require.NoError(t, err)
//
//		err = i.PushBytes(context.Background(), ids[j], traceBytes)
//		require.NoError(t, err)
//	}
//
//	queryAll(t, i, ids, traces)
//
//	blockID, err := i.CutBlockIfReady(0, 0, true)
//	require.NoError(t, err)
//	require.NotEqual(t, blockID, uuid.Nil)
//
//	queryAll(t, i, ids, traces)
//
//	err = i.CompleteBlock(blockID)
//	require.NoError(t, err)
//
//	queryAll(t, i, ids, traces)
//
//	err = i.ClearCompletingBlock(blockID)
//	require.NoError(t, err)
//
//	queryAll(t, i, ids, traces)
//
//	localBlock := i.GetBlockToBeFlushed(blockID)
//	require.NotNil(t, localBlock)
//
//	err = ingester.store.WriteBlock(context.Background(), localBlock)
//	require.NoError(t, err)
//
//	queryAll(t, i, ids, traces)
//}
//
//// pushTracesToInstance makes and pushes numTraces in the ingester tenantBlockManager,
//// returns traces and trace ids
//func pushTracesToInstance(t *testing.T, i *tenantBlockManager, numTraces int) ([]*tempopb.Trace, [][]byte) {
//	var ids [][]byte
//	var traces []*tempopb.Trace
//
//	for j := 0; j < numTraces; j++ {
//		id := make([]byte, 16)
//		_, err := crand.Read(id)
//		require.NoError(t, err)
//
//		testTrace := test.MakeTrace(10, id)
//		trace.SortTrace(testTrace)
//		traceBytes, err := model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForWrite(testTrace, 0, 0)
//		require.NoError(t, err)
//
//		err = i.PushBytes(context.Background(), id, traceBytes)
//		require.NoError(t, err)
//		require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))
//
//		ids = append(ids, id)
//		traces = append(traces, testTrace)
//	}
//	return traces, ids
//}
//
//func queryAll(t *testing.T, i *tenantBlockManager, ids [][]byte, traces []*tempopb.Trace) {
//	for j, id := range ids {
//		trace, err := i.FindTraceByID(context.Background(), id)
//		require.NoError(t, err)
//		require.Equal(t, traces[j], trace)
//	}
//}
//
//func TestInstanceDoesNotRace(t *testing.T) {
//	i, ingester := defaultInstance(t)
//	end := make(chan struct{})
//
//	concurrent := func(f func()) {
//		for {
//			select {
//			case <-end:
//				return
//			default:
//				f()
//			}
//		}
//	}
//	go concurrent(func() {
//		request := makeRequest([]byte{})
//		err := i.PushBytesRequest(context.Background(), request)
//		require.NoError(t, err, "error pushing traces")
//	})
//
//	go concurrent(func() {
//		err := i.CutSnapshots(0, true)
//		require.NoError(t, err, "error cutting complete traces")
//	})
//
//	go concurrent(func() {
//		blockID, _ := i.CutBlockIfReady(0, 0, false)
//		if blockID != uuid.Nil {
//			err := i.CompleteBlock(blockID)
//			require.NoError(t, err, "unexpected error completing block")
//			block := i.GetBlockToBeFlushed(blockID)
//			require.NotNil(t, block)
//			err = ingester.store.WriteBlock(context.Background(), block)
//			require.NoError(t, err, "error writing block")
//		}
//	})
//
//	go concurrent(func() {
//		err := i.ClearFlushedBlocks(0)
//		require.NoError(t, err, "error clearing flushed blocks")
//	})
//
//	go concurrent(func() {
//		_, err := i.FindTraceByID(context.Background(), []byte{0x01})
//		require.NoError(t, err, "error finding trace by id")
//	})
//
//	time.Sleep(100 * time.Millisecond)
//	close(end)
//	// Wait for go funcs to quit before
//	// exiting and cleaning up
//	time.Sleep(2 * time.Second)
//}
//
//func TestInstanceLimits(t *testing.T) {
//	limits, err := overrides.NewOverrides(overrides.Limits{
//		MaxBytesPerSnapshot:   1000,
//		MaxLocalSnapshotsPerUser: 4,
//	})
//	require.NoError(t, err, "unexpected error creating limits")
//	limiter := NewLimiter(limits, &ringCountMock{count: 1}, 1)
//
//	ingester, _, _ := defaultIngester(t, t.TempDir())
//	ingester.limiter = limiter
//
//	type push struct {
//		req          *tempopb.PushBytesRequest
//		expectsError bool
//	}
//
//	tests := []struct {
//		name   string
//		pushes []push
//	}{
//		{
//			name: "bytes - succeeds",
//			pushes: []push{
//				{
//					req: makeRequestWithByteLimit(300, []byte{}),
//				},
//				{
//					req: makeRequestWithByteLimit(500, []byte{}),
//				},
//				{
//					req: makeRequestWithByteLimit(100, []byte{}),
//				},
//			},
//		},
//		{
//			name: "bytes - one fails",
//			pushes: []push{
//				{
//					req: makeRequestWithByteLimit(300, []byte{}),
//				},
//				{
//					req:          makeRequestWithByteLimit(1500, []byte{}),
//					expectsError: true,
//				},
//				{
//					req: makeRequestWithByteLimit(600, []byte{}),
//				},
//			},
//		},
//		{
//			name: "bytes - multiple pushes same trace",
//			pushes: []push{
//				{
//					req: makeRequestWithByteLimit(500, []byte{0x01}),
//				},
//				{
//					req:          makeRequestWithByteLimit(700, []byte{0x01}),
//					expectsError: true,
//				},
//			},
//		},
//		{
//			name: "max traces - too many",
//			pushes: []push{
//				{
//					req: makeRequestWithByteLimit(100, []byte{}),
//				},
//				{
//					req: makeRequestWithByteLimit(100, []byte{}),
//				},
//				{
//					req: makeRequestWithByteLimit(100, []byte{}),
//				},
//				{
//					req: makeRequestWithByteLimit(100, []byte{}),
//				},
//				{
//					req:          makeRequestWithByteLimit(100, []byte{}),
//					expectsError: true,
//				},
//			},
//		},
//	}
//
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			delete(ingester.instances, testTenantID) // force recreate tenantBlockManager to reset limits
//			i, err := ingester.getOrCreateInstance(testTenantID)
//			require.NoError(t, err, "unexpected error creating new tenantBlockManager")
//
//			for j, push := range tt.pushes {
//				err = i.PushBytesRequest(context.Background(), push.req)
//
//				require.Equalf(t, push.expectsError, err != nil, "push %d failed: %w", j, err)
//			}
//		})
//	}
//}
//
//func TestInstanceCutCompleteTraces(t *testing.T) {
//	id := make([]byte, 16)
//	_, err := crand.Read(id)
//	require.NoError(t, err)
//
//	pastTrace := &liveSnapshot{
//		snapshotId: id,
//		lastAppend: time.Now().Add(-time.Hour),
//	}
//
//	id = make([]byte, 16)
//	_, err = crand.Read(id)
//	require.NoError(t, err)
//
//	nowTrace := &liveSnapshot{
//		snapshotId: id,
//		lastAppend: time.Now().Add(time.Hour),
//	}
//
//	tt := []struct {
//		name             string
//		cutoff           time.Duration
//		immediate        bool
//		input            []*liveSnapshot
//		expectedExist    []*liveSnapshot
//		expectedNotExist []*liveSnapshot
//	}{
//		{
//			name:      "empty",
//			cutoff:    0,
//			immediate: false,
//		},
//		{
//			name:             "cut immediate",
//			cutoff:           0,
//			immediate:        true,
//			input:            []*liveSnapshot{pastTrace, nowTrace},
//			expectedNotExist: []*liveSnapshot{pastTrace, nowTrace},
//		},
//		{
//			name:             "cut recent",
//			cutoff:           0,
//			immediate:        false,
//			input:            []*liveSnapshot{pastTrace, nowTrace},
//			expectedExist:    []*liveSnapshot{nowTrace},
//			expectedNotExist: []*liveSnapshot{pastTrace},
//		},
//		{
//			name:             "cut all time",
//			cutoff:           2 * time.Hour,
//			immediate:        false,
//			input:            []*liveSnapshot{pastTrace, nowTrace},
//			expectedNotExist: []*liveSnapshot{pastTrace, nowTrace},
//		},
//	}
//
//	for _, tc := range tt {
//		t.Run(tc.name, func(t *testing.T) {
//			tenantBlockManager, _ := defaultInstance(t)
//
//			for _, trace := range tc.input {
//				fp := tenantBlockManager.tokenForSnapshotID(trace.snapshotId)
//				tenantBlockManager.liveSnapshots[fp] = trace
//			}
//
//			err := tenantBlockManager.CutSnapshots(tc.cutoff, tc.immediate)
//			require.NoError(t, err)
//
//			require.Equal(t, len(tc.expectedExist), len(tenantBlockManager.liveSnapshots))
//			for _, expectedExist := range tc.expectedExist {
//				_, ok := tenantBlockManager.liveSnapshots[tenantBlockManager.tokenForSnapshotID(expectedExist.snapshotId)]
//				require.True(t, ok)
//			}
//
//			for _, expectedNotExist := range tc.expectedNotExist {
//				_, ok := tenantBlockManager.liveSnapshots[tenantBlockManager.tokenForSnapshotID(expectedNotExist.snapshotId)]
//				require.False(t, ok)
//			}
//		})
//	}
//}
//
//func TestInstanceCutBlockIfReady(t *testing.T) {
//	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)
//
//	tt := []struct {
//		name               string
//		maxBlockLifetime   time.Duration
//		maxBlockBytes      uint64
//		immediate          bool
//		pushCount          int
//		expectedToCutBlock bool
//	}{
//		{
//			name:               "empty",
//			expectedToCutBlock: false,
//		},
//		{
//			name:               "doesnt cut anything",
//			pushCount:          1,
//			expectedToCutBlock: false,
//		},
//		{
//			name:               "cut immediate",
//			immediate:          true,
//			pushCount:          1,
//			expectedToCutBlock: true,
//		},
//		{
//			name:               "cut based on block lifetime",
//			maxBlockLifetime:   time.Microsecond,
//			pushCount:          1,
//			expectedToCutBlock: true,
//		},
//		{
//			name:               "cut based on block size",
//			maxBlockBytes:      10,
//			pushCount:          10,
//			expectedToCutBlock: true,
//		},
//	}
//
//	for _, tc := range tt {
//		t.Run(tc.name, func(t *testing.T) {
//			tenantBlockManager, _ := defaultInstance(t)
//
//			for i := 0; i < tc.pushCount; i++ {
//				tr := test.MakeTrace(1, uuid.Nil[:])
//				bytes, err := dec.PrepareForWrite(tr, 0, 0)
//				require.NoError(t, err)
//				err = tenantBlockManager.PushBytes(context.Background(), uuid.Nil[:], bytes)
//				require.NoError(t, err)
//			}
//
//			// Defaults
//			if tc.maxBlockBytes == 0 {
//				tc.maxBlockBytes = 100000
//			}
//			if tc.maxBlockLifetime == 0 {
//				tc.maxBlockLifetime = time.Hour
//			}
//
//			lastCutTime := tenantBlockManager.lastBlockCut
//
//			// Cut all traces to headblock for testing
//			err := tenantBlockManager.CutSnapshots(0, true)
//			require.NoError(t, err)
//
//			blockID, err := tenantBlockManager.CutBlockIfReady(tc.maxBlockLifetime, tc.maxBlockBytes, tc.immediate)
//			require.NoError(t, err)
//
//			err = tenantBlockManager.CompleteBlock(blockID)
//			if tc.expectedToCutBlock {
//				require.NoError(t, err, "unexpected error completing block")
//			}
//
//			// Wait for goroutine to finish flushing to avoid test flakiness
//			if tc.expectedToCutBlock {
//				time.Sleep(time.Millisecond * 250)
//			}
//
//			require.Equal(t, tc.expectedToCutBlock, tenantBlockManager.lastBlockCut.After(lastCutTime))
//		})
//	}
//}
//
//func TestInstanceMetrics(t *testing.T) {
//	i, _ := defaultInstance(t)
//	cutAndVerify := func(v int) {
//		err := i.CutSnapshots(0, true)
//		require.NoError(t, err)
//
//		liveTraces, err := test.GetGaugeVecValue(metricLiveTraces, testTenantID)
//		require.NoError(t, err)
//		require.Equal(t, v, int(liveTraces))
//	}
//
//	cutAndVerify(0)
//
//	// Push some traces
//	count := 100
//	for j := 0; j < count; j++ {
//		request := makeRequest([]byte{})
//		err := i.PushBytesRequest(context.Background(), request)
//		require.NoError(t, err)
//	}
//	cutAndVerify(count)
//	cutAndVerify(0)
//}
//
//func TestInstanceFailsLargeTracesEvenAfterFlushing(t *testing.T) {
//	ctx := context.Background()
//	maxTraceBytes := 1000
//	id := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
//
//	limits, err := overrides.NewOverrides(overrides.Limits{
//		MaxBytesPerSnapshot: maxTraceBytes,
//	})
//	require.NoError(t, err)
//	limiter := NewLimiter(limits, &ringCountMock{count: 1}, 1)
//
//	ingester, _, _ := defaultIngester(t, t.TempDir())
//	ingester.limiter = limiter
//	i, err := ingester.getOrCreateInstance(testTenantID)
//	require.NoError(t, err)
//
//	req := makeRequestWithByteLimit(maxTraceBytes-200, id)
//	reqSize := 0
//	for _, b := range req.Traces {
//		reqSize += len(b.Slice)
//	}
//
//	// Fill up trace to max
//	err = i.PushBytesRequest(ctx, req)
//	require.NoError(t, err)
//
//	// Pushing again fails
//	err = i.PushBytesRequest(ctx, req)
//	require.Contains(t, err.Error(), (newSnapshotTooLargeError(id, i.instanceID, maxTraceBytes, reqSize)).Error())
//
//	// Pushing still fails after flush
//	err = i.CutCompleteTraces(0, true)
//	require.NoError(t, err)
//	err = i.PushBytesRequest(ctx, req)
//	require.Contains(t, err.Error(), (newSnapshotTooLargeError(id, i.instanceID, maxTraceBytes, reqSize)).Error())
//
//	// Cut block and then pushing works again
//	_, err = i.CutBlockIfReady(0, 0, true)
//	require.NoError(t, err)
//	err = i.PushBytesRequest(ctx, req)
//	require.NoError(t, err)
//}
//
//func TestSortByteSlices(t *testing.T) {
//	numTraces := 100
//
//	// create first trace
//	traceBytes := &tempopb.TraceBytes{
//		Traces: make([][]byte, numTraces),
//	}
//	for i := range traceBytes.Traces {
//		traceBytes.Traces[i] = make([]byte, rand.Intn(10))
//		_, err := crand.Read(traceBytes.Traces[i])
//		require.NoError(t, err)
//	}
//
//	// dupe
//	traceBytes2 := &tempopb.TraceBytes{
//		Traces: make([][]byte, numTraces),
//	}
//	for i := range traceBytes.Traces {
//		traceBytes2.Traces[i] = make([]byte, len(traceBytes.Traces[i]))
//		copy(traceBytes2.Traces[i], traceBytes.Traces[i])
//	}
//
//	// randomize dupe
//	rand.Shuffle(len(traceBytes2.Traces), func(i, j int) {
//		traceBytes2.Traces[i], traceBytes2.Traces[j] = traceBytes2.Traces[j], traceBytes2.Traces[i]
//	})
//
//	assert.NotEqual(t, traceBytes, traceBytes2)
//
//	// sort and compare
//	sortByteSlices(traceBytes.Traces)
//	sortByteSlices(traceBytes2.Traces)
//
//	assert.Equal(t, traceBytes, traceBytes2)
//}
//
//func defaultInstance(t testing.TB) (*tenantBlockManager, *Ingester) {
//	tenantBlockManager, ingester, _ := defaultInstanceAndTmpDir(t)
//	return tenantBlockManager, ingester
//}
//
//func defaultInstanceAndTmpDir(t testing.TB) (*tenantBlockManager, *Ingester, string) {
//	tmpDir := t.TempDir()
//
//	ingester, _, _ := defaultIngester(t, tmpDir)
//	tenantBlockManager, err := ingester.getOrCreateInstance(testTenantID)
//	require.NoError(t, err, "unexpected error creating new tenantBlockManager")
//
//	return tenantBlockManager, ingester, tmpDir
//}
//
//func BenchmarkInstancePush(b *testing.B) {
//	tenantBlockManager, _ := defaultInstance(b)
//	request := makeRequest([]byte{})
//
//	b.ResetTimer()
//	for i := 0; i < b.N; i++ {
//		// Rotate trace ID
//		binary.LittleEndian.PutUint32(request.Ids[0].Slice, uint32(i))
//		err := tenantBlockManager.PushBytesRequest(context.Background(), request)
//		require.NoError(b, err)
//	}
//}
//
//func BenchmarkInstancePushExistingTrace(b *testing.B) {
//	tenantBlockManager, _ := defaultInstance(b)
//	request := makeRequest([]byte{})
//
//	b.ResetTimer()
//	for i := 0; i < b.N; i++ {
//		err := tenantBlockManager.PushBytesRequest(context.Background(), request)
//		require.NoError(b, err)
//	}
//}
//
//func BenchmarkInstanceFindTraceByIDFromCompleteBlock(b *testing.B) {
//	tenantBlockManager, _ := defaultInstance(b)
//	traceID := test.ValidTraceID([]byte{1, 2, 3, 4, 5, 6, 7, 8})
//	request := makeRequest(traceID)
//	err := tenantBlockManager.PushBytesRequest(context.Background(), request)
//	require.NoError(b, err)
//
//	// force the trace to be in a complete block
//	err = tenantBlockManager.CutSnapshots(0, true)
//	require.NoError(b, err)
//	id, err := tenantBlockManager.CutBlockIfReady(0, 0, true)
//	require.NoError(b, err)
//	err = tenantBlockManager.CompleteBlock(id)
//	require.NoError(b, err)
//
//	require.Equal(b, 1, len(tenantBlockManager.completeBlocks))
//
//	b.ResetTimer()
//	for i := 0; i < b.N; i++ {
//		trace, err := tenantBlockManager.FindTraceByID(context.Background(), traceID)
//		require.NotNil(b, trace)
//		require.NoError(b, err)
//	}
//}
//
//func BenchmarkInstanceSearchCompleteParquet(b *testing.B) {
//	benchmarkInstanceSearch(b)
//}
//func TestInstanceSearchCompleteParquet(t *testing.T) {
//	benchmarkInstanceSearch(t)
//}
//func benchmarkInstanceSearch(b testing.TB) {
//	tenantBlockManager, _ := defaultInstance(b)
//	for i := 0; i < 1000; i++ {
//		request := makeRequest(nil)
//		err := tenantBlockManager.PushBytesRequest(context.Background(), request)
//		require.NoError(b, err)
//
//		if i%100 == 0 {
//			err := tenantBlockManager.CutSnapshots(0, true)
//			require.NoError(b, err)
//		}
//	}
//
//	// force the traces to be in a complete block
//	id, err := tenantBlockManager.CutBlockIfReady(0, 0, true)
//	require.NoError(b, err)
//	err = tenantBlockManager.CompleteBlock(id)
//	require.NoError(b, err)
//
//	require.Equal(b, 1, len(tenantBlockManager.completeBlocks))
//
//	ctx := context.Background()
//	ctx = user.InjectOrgID(ctx, testTenantID)
//
//	if rt, ok := b.(*testing.B); ok {
//		rt.ResetTimer()
//		for i := 0; i < rt.N; i++ {
//			resp, err := tenantBlockManager.SearchTags(ctx)
//			require.NoError(b, err)
//			require.NotNil(b, resp)
//		}
//		return
//	}
//
//	for i := 0; i < 100; i++ {
//		resp, err := tenantBlockManager.SearchTags(ctx)
//		require.NoError(b, err)
//		require.NotNil(b, resp)
//	}
//}
//
//func makeRequest(id []byte) *deeppb.PushBytesRequest {
//	buffer, _ := model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForWrite(&deep_tp.Snapshot{}, 0)
//
//	id = test.ValidTraceID(id)
//	return &deeppb.PushBytesRequest{
//		Snapshot: buffer,
//		Id:       id,
//	}
//}
//
//// Note that this fn will generate a request with size **close to** maxBytes
//func makeRequestWithByteLimit(maxBytes int, traceID []byte) *tempopb.PushBytesRequest {
//	traceID = test.ValidTraceID(traceID)
//	batch := test.MakeBatch(1, traceID)
//
//	for batch.Size() < maxBytes {
//		batch.ScopeSpans[0].Spans = append(batch.ScopeSpans[0].Spans, test.MakeSpanWithAttributeCount(traceID, 0))
//	}
//
//	return makePushBytesRequest(traceID, batch)
//}
//
//func makePushBytesRequest(traceID []byte, batch *v1_trace.ResourceSpans) *tempopb.PushBytesRequest {
//	trace := &tempopb.Trace{Batches: []*v1_trace.ResourceSpans{batch}}
//
//	buffer, err := model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForWrite(trace, 0, 0)
//	if err != nil {
//		panic(err)
//	}
//
//	return &tempopb.PushBytesRequest{
//		Ids: []tempopb.PreallocBytes{{
//			Slice: traceID,
//		}},
//		Traces: []tempopb.PreallocBytes{{
//			Slice: buffer,
//		}},
//	}
//}
