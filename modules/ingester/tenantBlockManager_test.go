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
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/pkg/deeppb"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/model"
	v1 "github.com/intergral/deep/pkg/model/v1"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

const testTenantID = "fake"

type ringCountMock struct {
	count int
}

func (m *ringCountMock) HealthyInstancesCount() int {
	return m.count
}

func TestInstance(t *testing.T) {
	request := makeRequest([]byte{})

	i, ingester := defaultInstance(t)

	err := i.PushBytesRequest(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))

	err = i.CutSnapshots(0, true)
	require.NoError(t, err)
	require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))

	blockID, err := i.CutBlockIfReady(0, 0, false)
	require.NoError(t, err, "unexpected error cutting block")
	require.NotEqual(t, blockID, uuid.Nil)

	err = i.CompleteBlock(blockID)
	require.NoError(t, err, "unexpected error completing block")

	block := i.GetBlockToBeFlushed(blockID)
	require.NotNil(t, block)
	require.Len(t, i.completingBlocks, 1)
	require.Len(t, i.completeBlocks, 1)

	err = ingester.store.WriteBlock(context.Background(), block)
	require.NoError(t, err)

	err = i.ClearFlushedBlocks(30 * time.Hour)
	require.NoError(t, err)
	require.Len(t, i.completeBlocks, 1)

	err = i.ClearFlushedBlocks(0)
	require.NoError(t, err)
	require.Len(t, i.completeBlocks, 0)

	err = i.resetHeadBlock()
	require.NoError(t, err, "unexpected error resetting block")

	require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))
}

func TestInstanceFind(t *testing.T) {
	i, ingester := defaultInstance(t)

	numSnapshots := 10
	snapshots, ids := pushSnapshotsToInstance(t, i, numSnapshots)

	queryAll(t, i, ids, snapshots)

	err := i.CutSnapshots(0, true)
	require.NoError(t, err)
	require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))

	for j := 0; j < numSnapshots; j++ {
		snapshotBytes, err := model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForWrite(snapshots[j], 0)
		require.NoError(t, err)

		err = i.PushBytes(context.Background(), ids[j], snapshotBytes)
		require.NoError(t, err)
	}

	queryAll(t, i, ids, snapshots)

	blockID, err := i.CutBlockIfReady(0, 0, true)
	require.NoError(t, err)
	require.NotEqual(t, blockID, uuid.Nil)

	queryAll(t, i, ids, snapshots)

	err = i.CompleteBlock(blockID)
	require.NoError(t, err)

	queryAll(t, i, ids, snapshots)

	err = i.ClearCompletingBlock(blockID)
	require.NoError(t, err)

	queryAll(t, i, ids, snapshots)

	localBlock := i.GetBlockToBeFlushed(blockID)
	require.NotNil(t, localBlock)

	err = ingester.store.WriteBlock(context.Background(), localBlock)
	require.NoError(t, err)

	queryAll(t, i, ids, snapshots)
}

// pushSnapshotsToInstance makes and pushes numSnapshots in the ingester tenantBlockManager,
// returns snapshots and snapshot ids
func pushSnapshotsToInstance(t *testing.T, i *tenantBlockManager, numSnapshots int) ([]*deep_tp.Snapshot, [][]byte) {
	var ids [][]byte
	var snapshots []*deep_tp.Snapshot

	for j := 0; j < numSnapshots; j++ {
		id := make([]byte, 16)
		_, err := crand.Read(id)
		require.NoError(t, err)

		snapshot := test.GenerateSnapshot(10, &test.GenerateOptions{Id: id, ServiceName: "test-service", DurationNanos: 1 * 1e9})

		snapshotBytes, err := model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForWrite(snapshot, 0)
		require.NoError(t, err)

		err = i.PushBytes(context.Background(), id, snapshotBytes)
		require.NoError(t, err)
		require.Equal(t, int(i.snapshotCount.Load()), len(i.liveSnapshots))

		ids = append(ids, id)
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, ids
}

func queryAll(t *testing.T, i *tenantBlockManager, ids [][]byte, snapshots []*deep_tp.Snapshot) {
	for j, id := range ids {
		snapshot, err := i.FindSnapshotByID(context.Background(), id)
		require.NoError(t, err)
		assert.True(t, proto.Equal(snapshots[j], snapshot), "Snapshots not equal")
	}
}

func TestInstanceDoesNotRace(t *testing.T) {
	i, ingester := defaultInstance(t)
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
		request := makeRequest([]byte{})
		if len(i.liveSnapshots) < 9000 {
			err := i.PushBytesRequest(context.Background(), request)
			require.NoError(t, err, "error pushing snapshots")
		}
	})

	go concurrent(func() {
		err := i.CutSnapshots(0, true)
		require.NoError(t, err, "error cutting complete snapshots")
	})

	go concurrent(func() {
		blockID, _ := i.CutBlockIfReady(0, 0, false)
		if blockID != uuid.Nil {
			err := i.CompleteBlock(blockID)
			require.NoError(t, err, "unexpected error completing block")
			block := i.GetBlockToBeFlushed(blockID)
			require.NotNil(t, block)
			err = ingester.store.WriteBlock(context.Background(), block)
			require.NoError(t, err, "error writing block")
		}
	})

	go concurrent(func() {
		err := i.ClearFlushedBlocks(0)
		require.NoError(t, err, "error clearing flushed blocks")
	})

	go concurrent(func() {
		_, err := i.FindSnapshotByID(context.Background(), []byte{0x01})
		require.NoError(t, err, "error finding snapshot by id")
	})

	time.Sleep(100 * time.Millisecond)
	close(end)
	// Wait for go funcs to quit before
	// exiting and cleaning up
	time.Sleep(2 * time.Second)
}

func TestInstanceLimits(t *testing.T) {
	limits, err := overrides.NewOverrides(overrides.Limits{
		MaxBytesPerSnapshot:        1000,
		MaxLocalSnapshotsPerTenant: 4,
	})
	require.NoError(t, err, "unexpected error creating limits")
	limiter := NewLimiter(limits, &ringCountMock{count: 1}, 1)

	ingester, _, _ := defaultIngester(t, t.TempDir())
	ingester.limiter = limiter

	type push struct {
		req          *deeppb.PushBytesRequest
		expectsError bool
	}

	tests := []struct {
		name   string
		pushes []push
	}{
		{
			name: "bytes - succeeds",
			pushes: []push{
				{
					req: makeRequestWithByteLimit(300, []byte{}),
				},
				{
					req: makeRequestWithByteLimit(500, []byte{}),
				},
				{
					req: makeRequestWithByteLimit(100, []byte{}),
				},
			},
		},
		{
			name: "bytes - one fails",
			pushes: []push{
				{
					req: makeRequestWithByteLimit(300, []byte{}),
				},
				{
					req:          makeRequestWithByteLimit(1500, []byte{}),
					expectsError: true,
				},
				{
					req: makeRequestWithByteLimit(600, []byte{}),
				},
			},
		},
		{
			name: "bytes - multiple pushes same snapshot",
			pushes: []push{
				{
					req: makeRequestWithByteLimit(500, []byte{0x01}),
				},
				{
					req:          makeRequestWithByteLimit(700, []byte{0x01}),
					expectsError: true,
				},
			},
		},
		{
			name: "max snapshots - too many",
			pushes: []push{
				{
					req: makeRequestWithByteLimit(100, []byte{}),
				},
				{
					req: makeRequestWithByteLimit(100, []byte{}),
				},
				{
					req: makeRequestWithByteLimit(100, []byte{}),
				},
				{
					req: makeRequestWithByteLimit(100, []byte{}),
				},
				{
					req:          makeRequestWithByteLimit(100, []byte{}),
					expectsError: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delete(ingester.instances, testTenantID) // force recreate tenantBlockManager to reset limits
			i, err := ingester.getOrCreateInstance(testTenantID)
			require.NoError(t, err, "unexpected error creating new tenantBlockManager")

			for j, push := range tt.pushes {
				err = i.PushBytesRequest(context.Background(), push.req)

				require.Equalf(t, push.expectsError, err != nil, "push %d failed: %w", j, err)
			}
		})
	}
}

func TestInstanceCutCompleteSnapshots(t *testing.T) {
	id := make([]byte, 16)
	_, err := crand.Read(id)
	require.NoError(t, err)

	pastSnapshot := &liveSnapshot{
		snapshotId: id,
		lastAppend: time.Now().Add(-time.Hour),
	}
	snapshot := test.GenerateSnapshot(1, &test.GenerateOptions{Id: id})
	write, err := v1.NewObjectDecoder().PrepareForWrite(snapshot, 0)
	require.NoError(t, err)
	pastSnapshot.bytes = write

	id = make([]byte, 16)
	_, err = crand.Read(id)
	require.NoError(t, err)

	nowSnapshot := &liveSnapshot{
		snapshotId: id,
		lastAppend: time.Now().Add(time.Hour),
	}
	snapshot = test.GenerateSnapshot(1, &test.GenerateOptions{Id: id})
	write, err = v1.NewObjectDecoder().PrepareForWrite(snapshot, 0)
	require.NoError(t, err)
	nowSnapshot.bytes = write

	tt := []struct {
		name             string
		cutoff           time.Duration
		immediate        bool
		input            []*liveSnapshot
		expectedExist    []*liveSnapshot
		expectedNotExist []*liveSnapshot
	}{
		{
			name:      "empty",
			cutoff:    0,
			immediate: false,
		},
		{
			name:             "cut immediate",
			cutoff:           0,
			immediate:        true,
			input:            []*liveSnapshot{pastSnapshot, nowSnapshot},
			expectedNotExist: []*liveSnapshot{pastSnapshot, nowSnapshot},
		},
		{
			name:             "cut recent",
			cutoff:           0,
			immediate:        false,
			input:            []*liveSnapshot{pastSnapshot, nowSnapshot},
			expectedExist:    []*liveSnapshot{nowSnapshot},
			expectedNotExist: []*liveSnapshot{pastSnapshot},
		},
		{
			name:             "cut all time",
			cutoff:           2 * time.Hour,
			immediate:        false,
			input:            []*liveSnapshot{pastSnapshot, nowSnapshot},
			expectedNotExist: []*liveSnapshot{pastSnapshot, nowSnapshot},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tenantBlockManager, _ := defaultInstance(t)

			for _, snapshot := range tc.input {
				fp := tenantBlockManager.tokenForSnapshotID(snapshot.snapshotId)
				tenantBlockManager.liveSnapshots[fp] = snapshot
			}

			err := tenantBlockManager.CutSnapshots(tc.cutoff, tc.immediate)
			require.NoError(t, err)

			require.Equal(t, len(tc.expectedExist), len(tenantBlockManager.liveSnapshots))
			for _, expectedExist := range tc.expectedExist {
				_, ok := tenantBlockManager.liveSnapshots[tenantBlockManager.tokenForSnapshotID(expectedExist.snapshotId)]
				require.True(t, ok)
			}

			for _, expectedNotExist := range tc.expectedNotExist {
				_, ok := tenantBlockManager.liveSnapshots[tenantBlockManager.tokenForSnapshotID(expectedNotExist.snapshotId)]
				require.False(t, ok)
			}
		})
	}
}

func TestInstanceCutBlockIfReady(t *testing.T) {
	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	tt := []struct {
		name               string
		maxBlockLifetime   time.Duration
		maxBlockBytes      uint64
		immediate          bool
		pushCount          int
		expectedToCutBlock bool
	}{
		{
			name:               "empty",
			expectedToCutBlock: false,
		},
		{
			name:               "doesnt cut anything",
			pushCount:          1,
			expectedToCutBlock: false,
		},
		{
			name:               "cut immediate",
			immediate:          true,
			pushCount:          1,
			expectedToCutBlock: true,
		},
		{
			name:               "cut based on block lifetime",
			maxBlockLifetime:   time.Microsecond,
			pushCount:          1,
			expectedToCutBlock: true,
		},
		{
			name:               "cut based on block size",
			maxBlockBytes:      10,
			pushCount:          10,
			expectedToCutBlock: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			tenantBlockManager, _ := defaultInstance(t)

			for i := 0; i < tc.pushCount; i++ {
				tr := test.GenerateSnapshot(1, &test.GenerateOptions{Id: uuid.Nil[:]})
				bytes, err := dec.PrepareForWrite(tr, 0)
				require.NoError(t, err)
				err = tenantBlockManager.PushBytes(context.Background(), uuid.Nil[:], bytes)
				require.NoError(t, err)
			}

			// Defaults
			if tc.maxBlockBytes == 0 {
				tc.maxBlockBytes = 100000
			}
			if tc.maxBlockLifetime == 0 {
				tc.maxBlockLifetime = time.Hour
			}

			lastCutTime := tenantBlockManager.lastBlockCut

			// Cut all snapshots to headblock for testing
			err := tenantBlockManager.CutSnapshots(0, true)
			require.NoError(t, err)

			blockID, err := tenantBlockManager.CutBlockIfReady(tc.maxBlockLifetime, tc.maxBlockBytes, tc.immediate)
			require.NoError(t, err)

			err = tenantBlockManager.CompleteBlock(blockID)
			if tc.expectedToCutBlock {
				require.NoError(t, err, "unexpected error completing block")
			}

			// Wait for goroutine to finish flushing to avoid test flakiness
			if tc.expectedToCutBlock {
				time.Sleep(time.Millisecond * 250)
			}

			require.Equal(t, tc.expectedToCutBlock, tenantBlockManager.lastBlockCut.After(lastCutTime))
		})
	}
}

func TestInstanceMetrics(t *testing.T) {
	i, _ := defaultInstance(t)
	cutAndVerify := func(v int) {
		err := i.CutSnapshots(0, true)
		require.NoError(t, err)

		liveSnapshots, err := test.GetGaugeVecValue(metricLiveSnapshots, testTenantID)
		require.NoError(t, err)
		require.Equal(t, v, int(liveSnapshots))
	}

	cutAndVerify(0)

	// Push some snapshots
	count := 100
	for j := 0; j < count; j++ {
		request := makeRequest([]byte{})
		err := i.PushBytesRequest(context.Background(), request)
		require.NoError(t, err)
	}
	cutAndVerify(count)
	cutAndVerify(0)
}

func TestInstanceFailsLargeSnapshotsEvenAfterFlushing(t *testing.T) {
	ctx := context.Background()
	maxSnapshotBytes := 1000
	id := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	limits, err := overrides.NewOverrides(overrides.Limits{
		MaxBytesPerSnapshot: maxSnapshotBytes,
	})
	require.NoError(t, err)
	limiter := NewLimiter(limits, &ringCountMock{count: 1}, 1)

	ingester, _, _ := defaultIngester(t, t.TempDir())
	ingester.limiter = limiter
	i, err := ingester.getOrCreateInstance(testTenantID)
	require.NoError(t, err)

	req := makeRequestWithByteLimit(maxSnapshotBytes-200, id)
	reqSize := len(req.Snapshot)

	// Fill up snapshot to max
	err = i.PushBytesRequest(ctx, req)
	require.NoError(t, err)

	// Pushing again fails
	err = i.PushBytesRequest(ctx, req)
	require.Contains(t, err.Error(), (newSnapshotTooLargeError(id, i.tenantID, maxSnapshotBytes, reqSize)).Error())

	// Pushing still fails after flush
	err = i.CutSnapshots(0, true)
	require.NoError(t, err)
	err = i.PushBytesRequest(ctx, req)
	require.Contains(t, err.Error(), (newSnapshotTooLargeError(id, i.tenantID, maxSnapshotBytes, reqSize)).Error())

	// Cut block and then pushing works again
	_, err = i.CutBlockIfReady(0, 0, true)
	require.NoError(t, err)
	err = i.PushBytesRequest(ctx, req)
	require.NoError(t, err)
}

func defaultInstance(t testing.TB) (*tenantBlockManager, *Ingester) {
	tenantBlockManager, ingester, _ := defaultInstanceAndTmpDir(t)
	return tenantBlockManager, ingester
}

func defaultInstanceAndTmpDir(t testing.TB) (*tenantBlockManager, *Ingester, string) {
	tmpDir := t.TempDir()

	ingester, _, _ := defaultIngester(t, tmpDir)
	tenantBlockManager, err := ingester.getOrCreateInstance(testTenantID)
	require.NoError(t, err, "unexpected error creating new tenantBlockManager")

	return tenantBlockManager, ingester, tmpDir
}

func BenchmarkInstancePush(b *testing.B) {
	tenantBlockManager, _ := defaultInstance(b)
	request := makeRequest([]byte{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Rotate snapshot ID
		binary.LittleEndian.PutUint32(request.ID, uint32(i))
		err := tenantBlockManager.PushBytesRequest(context.Background(), request)
		require.NoError(b, err)
	}
}

func BenchmarkInstancePushExistingSnapshot(b *testing.B) {
	tenantBlockManager, _ := defaultInstance(b)
	request := makeRequest([]byte{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := tenantBlockManager.PushBytesRequest(context.Background(), request)
		require.NoError(b, err)
	}
}

func BenchmarkInstanceFindSnapshotByIDFromCompleteBlock(b *testing.B) {
	tenantBlockManager, _ := defaultInstance(b)
	snapshotID := test.ValidSnapshotID([]byte{1, 2, 3, 4, 5, 6, 7, 8})
	request := makeRequest(snapshotID)
	err := tenantBlockManager.PushBytesRequest(context.Background(), request)
	require.NoError(b, err)

	// force the snapshot to be in a complete block
	err = tenantBlockManager.CutSnapshots(0, true)
	require.NoError(b, err)
	id, err := tenantBlockManager.CutBlockIfReady(0, 0, true)
	require.NoError(b, err)
	err = tenantBlockManager.CompleteBlock(id)
	require.NoError(b, err)

	require.Equal(b, 1, len(tenantBlockManager.completeBlocks))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snapshot, err := tenantBlockManager.FindSnapshotByID(context.Background(), snapshotID)
		require.NotNil(b, snapshot)
		require.NoError(b, err)
	}
}

func BenchmarkInstanceSearchCompleteParquet(b *testing.B) {
	benchmarkInstanceSearch(b)
}
func TestInstanceSearchCompleteParquet(t *testing.T) {
	benchmarkInstanceSearch(t)
}
func benchmarkInstanceSearch(b testing.TB) {
	tenantBlockManager, _ := defaultInstance(b)
	for i := 0; i < 1000; i++ {
		request := makeRequest(nil)
		err := tenantBlockManager.PushBytesRequest(context.Background(), request)
		require.NoError(b, err)

		if i%100 == 0 {
			err := tenantBlockManager.CutSnapshots(0, true)
			require.NoError(b, err)
		}
	}

	// force the snapshots to be in a complete block
	id, err := tenantBlockManager.CutBlockIfReady(0, 0, true)
	require.NoError(b, err)
	err = tenantBlockManager.CompleteBlock(id)
	require.NoError(b, err)

	require.Equal(b, 1, len(tenantBlockManager.completeBlocks))

	ctx := context.Background()
	ctx = util.InjectTenantID(ctx, testTenantID)

	if rt, ok := b.(*testing.B); ok {
		rt.ResetTimer()
		for i := 0; i < rt.N; i++ {
			resp, err := tenantBlockManager.SearchTags(ctx)
			require.NoError(b, err)
			require.NotNil(b, resp)
		}
		return
	}

	for i := 0; i < 100; i++ {
		resp, err := tenantBlockManager.SearchTags(ctx)
		require.NoError(b, err)
		require.NotNil(b, resp)
	}
}

func makeRequest(id []byte) *deeppb.PushBytesRequest {
	buffer, _ := model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForWrite(&deep_tp.Snapshot{}, 0)

	id = test.ValidSnapshotID(id)
	return &deeppb.PushBytesRequest{
		Snapshot: buffer,
		ID:       id,
	}
}

// Note that this fn will generate a request with size **close to** maxBytes
func makeRequestWithByteLimit(maxBytes int, snapshotID []byte) *deeppb.PushBytesRequest {
	snapshotID = test.ValidSnapshotID(snapshotID)
	snapshot := test.GenerateSnapshot(1, &test.GenerateOptions{Id: snapshotID})

	for proto.Size(snapshot) < maxBytes {
		test.AppendFrame(snapshot)
	}

	return makePushBytesRequest(snapshotID, snapshot)
}

func makePushBytesRequest(snapshotID []byte, snapshot *deep_tp.Snapshot) *deeppb.PushBytesRequest {

	buffer, err := model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForWrite(snapshot, 0)
	if err != nil {
		panic(err)
	}

	return &deeppb.PushBytesRequest{
		Snapshot: buffer,
		ID:       snapshotID,
	}
}
