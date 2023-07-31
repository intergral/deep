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
	"encoding/json"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/stretchr/testify/assert"
	"path"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
	"github.com/intergral/deep/pkg/deepdb/pool"
	"github.com/intergral/deep/pkg/deepdb/wal"
	"github.com/intergral/deep/pkg/model"
	"github.com/intergral/deep/pkg/util/test"
)

type mockSharder struct {
}

func (m *mockSharder) Owns(hash string) bool {
	return true
}

func (m *mockSharder) RecordDiscardedSnapshots(count int, tenantID string, snapshotID string) {}

type mockJobSharder struct{}

func (m *mockJobSharder) Owns(_ string) bool { return true }

type mockOverrides struct {
	blockRetention      time.Duration
	maxBytesPerSnapshot int
}

func (m *mockOverrides) BlockRetentionForTenant(_ string) time.Duration {
	return m.blockRetention
}

func (m *mockOverrides) MaxBytesPerSnapshotForTenant(_ string) int {
	return m.maxBytesPerSnapshot
}

func TestCompactionRoundtrip(t *testing.T) {
	testEncodings := []string{vparquet.VersionString}
	for _, enc := range testEncodings {
		t.Run(enc, func(t *testing.T) {
			testCompactionRoundtrip(t, enc)
		})
	}
}

func testCompactionRoundtrip(t *testing.T, targetBlockVersion string) {
	tempDir := t.TempDir()

	r, w, _, _, c, err := New(&Config{
		Backend: "local",
		Pool: &pool.Config{
			MaxWorkers: 10,
			QueueDepth: 100,
		},
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             targetBlockVersion,
			Encoding:            backend.EncLZ4_4M,
			RowGroupSizeBytes:   30_000_000,
		},
		WAL: &wal.Config{
			Filepath: path.Join(tempDir, "wal"),
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	require.NoError(t, err)

	ctx := context.Background()
	c.EnableCompaction(ctx, &CompactorConfig{
		ChunkSizeBytes:          10_000_000,
		FlushSizeBytes:          10_000_000,
		MaxCompactionRange:      24 * time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})

	walBlocks := w.WAL()
	require.NoError(t, err)

	blockCount := 4
	recordCount := 100

	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	allReqs := make([]*deeptp.Snapshot, 0, blockCount*recordCount)
	allIds := make([]common.ID, 0, blockCount*recordCount)

	for i := 0; i < blockCount; i++ {
		blockID := uuid.New()
		head, err := walBlocks.NewBlock(blockID, testTenantID, model.CurrentEncoding)
		require.NoError(t, err)

		for j := 0; j < recordCount; j++ {
			req := test.GenerateSnapshot(j, nil)
			id := req.ID

			writeSnapshotToWal(t, head, dec, id, req, 0)

			allReqs = append(allReqs, req)
			allIds = append(allIds, id)
		}

		_, err = w.CompleteBlock(ctx, head)
		require.NoError(t, err)
	}

	rw := r.(*readerWriter)

	expectedBlockCount := blockCount
	expectedCompactedCount := 0
	checkBlocklists(t, uuid.Nil, expectedBlockCount, expectedCompactedCount, rw)

	blocksPerCompaction := inputBlocks - outputBlocks

	rw.pollBlocklist()

	blocklist := rw.blocklist.Metas(testTenantID)
	blockSelector := newTimeWindowBlockSelector(blocklist, rw.compactorCfg.MaxCompactionRange, 10000, 1024*1024*1024, defaultMinInputBlocks, 2)

	expectedCompactions := len(blocklist) / inputBlocks
	compactions := 0
	for {
		blocks, _ := blockSelector.BlocksToCompact()
		if len(blocks) == 0 {
			break
		}
		require.Len(t, blocks, inputBlocks)

		compactions++
		err := rw.compact(context.Background(), blocks, testTenantID)
		require.NoError(t, err)

		expectedBlockCount -= blocksPerCompaction
		expectedCompactedCount += inputBlocks
		checkBlocklists(t, uuid.Nil, expectedBlockCount, expectedCompactedCount, rw)
	}

	require.Equal(t, expectedCompactions, compactions)

	// do we have the right number of records
	var records int
	for _, meta := range rw.blocklist.Metas(testTenantID) {
		records += meta.TotalObjects
	}
	require.Equal(t, blockCount*recordCount, records)

	// now see if we can find our ids
	for i, id := range allIds {
		snapshot, failedBlocks, err := rw.FindSnapshot(context.Background(), testTenantID, id, BlockIDMin, BlockIDMax, 0, 0)
		require.NoError(t, err)
		require.Nil(t, failedBlocks)
		require.NotNil(t, snapshot)

		if !proto.Equal(allReqs[i], snapshot) {
			wantJSON, _ := json.MarshalIndent(allReqs[i], "", "  ")
			gotJSON, _ := json.MarshalIndent(snapshot, "", "  ")
			require.Equal(t, wantJSON, gotJSON)
		}
	}
}

func TestCompactionUpdatesBlocklist(t *testing.T) {
	tempDir := t.TempDir()

	r, w, _, _, c, err := New(&Config{
		Backend: "local",
		Pool: &pool.Config{
			MaxWorkers: 10,
			QueueDepth: 100,
		},
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             encoding.DefaultEncoding().Version(),
			Encoding:            backend.EncNone,
		},
		WAL: &wal.Config{
			Filepath: path.Join(tempDir, "wal"),
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	require.NoError(t, err)

	ctx := context.Background()
	c.EnableCompaction(ctx, &CompactorConfig{
		ChunkSizeBytes:          10,
		MaxCompactionRange:      24 * time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})

	// Cut x blocks with y records each
	blockCount := 5
	recordCount := 1
	_, ids := cutTestBlocks(t, w, testTenantID, blockCount, recordCount)

	rw := r.(*readerWriter)
	rw.pollBlocklist()

	// compact everything
	err = rw.compact(ctx, rw.blocklist.Metas(testTenantID), testTenantID)
	require.NoError(t, err)

	// New blocklist contains 1 compacted block with everything
	blocks := rw.blocklist.Metas(testTenantID)
	require.Equal(t, 1, len(blocks))
	require.Equal(t, uint8(1), blocks[0].CompactionLevel)
	require.Equal(t, blockCount*recordCount, blocks[0].TotalObjects)

	// Compacted list contains all old blocks
	require.Equal(t, blockCount, len(rw.blocklist.CompactedMetas(testTenantID)))

	// Make sure all expected snapshots are found.
	for i := 0; i < blockCount; i++ {
		for j := 0; j < recordCount; j++ {
			snapshot, failedBlocks, err := rw.FindSnapshot(context.TODO(), testTenantID, ids[i], BlockIDMin, BlockIDMax, 0, 0)
			require.NotNil(t, snapshot)
			require.NoError(t, err)
			require.Nil(t, failedBlocks)
		}
	}
}

func TestCompactionMetrics(t *testing.T) {
	tempDir := t.TempDir()

	r, w, _, _, c, err := New(&Config{
		Backend: "local",
		Pool: &pool.Config{
			MaxWorkers: 10,
			QueueDepth: 100,
		},
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             encoding.DefaultEncoding().Version(),
			Encoding:            backend.EncNone,
		},
		WAL: &wal.Config{
			Filepath: path.Join(tempDir, "wal"),
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	assert.NoError(t, err)

	ctx := context.Background()
	c.EnableCompaction(ctx, &CompactorConfig{
		ChunkSizeBytes:          10,
		MaxCompactionRange:      24 * time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})

	// Cut x blocks with y records each
	blockCount := 5
	recordCount := 10
	cutTestBlocks(t, w, testTenantID, blockCount, recordCount)

	rw := r.(*readerWriter)
	rw.pollBlocklist()

	// Get starting metrics
	processedStart, err := test.GetCounterVecValue(metricCompactionObjectsWritten, "0")
	assert.NoError(t, err)

	blocksStart, err := test.GetCounterVecValue(metricCompactionBlocks, "0")
	assert.NoError(t, err)

	bytesStart, err := test.GetCounterVecValue(metricCompactionBytesWritten, "0")
	assert.NoError(t, err)

	// compact everything
	err = rw.compact(ctx, rw.blocklist.Metas(testTenantID), testTenantID)
	assert.NoError(t, err)

	// Check metric
	processedEnd, err := test.GetCounterVecValue(metricCompactionObjectsWritten, "0")
	assert.NoError(t, err)
	assert.Equal(t, float64(blockCount*recordCount), processedEnd-processedStart)

	blocksEnd, err := test.GetCounterVecValue(metricCompactionBlocks, "0")
	assert.NoError(t, err)
	assert.Equal(t, float64(blockCount), blocksEnd-blocksStart)

	bytesEnd, err := test.GetCounterVecValue(metricCompactionBytesWritten, "0")
	assert.NoError(t, err)
	assert.Greater(t, bytesEnd, bytesStart) // calculating the exact bytes requires knowledge of the bytes as written in the blocks.  just make sure it goes up
}

func TestCompactionIteratesThroughTenants(t *testing.T) {
	tempDir := t.TempDir()

	r, w, _, _, c, err := New(&Config{
		Backend: "local",
		Pool: &pool.Config{
			MaxWorkers: 10,
			QueueDepth: 100,
		},
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             encoding.DefaultEncoding().Version(),
			Encoding:            backend.EncLZ4_64k,
		},
		WAL: &wal.Config{
			Filepath: path.Join(tempDir, "wal"),
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	assert.NoError(t, err)

	ctx := context.Background()
	c.EnableCompaction(ctx, &CompactorConfig{
		ChunkSizeBytes:          10,
		MaxCompactionRange:      24 * time.Hour,
		MaxCompactionObjects:    1000,
		MaxBlockBytes:           1024 * 1024 * 1024,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})

	// Cut blocks for multiple tenants
	cutTestBlocks(t, w, testTenantID, 2, 2)
	cutTestBlocks(t, w, testTenantID2, 2, 2)

	rw := r.(*readerWriter)
	rw.pollBlocklist()

	assert.Equal(t, 2, len(rw.blocklist.Metas(testTenantID)))
	assert.Equal(t, 2, len(rw.blocklist.Metas(testTenantID2)))

	// Verify that tenant 2 compacted, tenant 1 is not
	// Compaction starts at index 1 for simplicity
	rw.doCompaction(ctx)
	assert.Equal(t, 2, len(rw.blocklist.Metas(testTenantID)))
	assert.Equal(t, 1, len(rw.blocklist.Metas(testTenantID2)))

	// Verify both tenants compacted after second run
	rw.doCompaction(ctx)
	assert.Equal(t, 1, len(rw.blocklist.Metas(testTenantID)))
	assert.Equal(t, 1, len(rw.blocklist.Metas(testTenantID2)))
}

func TestCompactionHonorsBlockStartEndTimes(t *testing.T) {

	testEncodings := []string{vparquet.VersionString}
	for _, enc := range testEncodings {
		t.Run(enc, func(t *testing.T) {
			testCompactionHonorsBlockStartEndTimes(t, enc)
		})
	}
}

func testCompactionHonorsBlockStartEndTimes(t *testing.T, targetBlockVersion string) {
	tempDir := t.TempDir()

	r, w, _, _, c, err := New(&Config{
		Backend: "local",
		Pool: &pool.Config{
			MaxWorkers: 10,
			QueueDepth: 100,
		},
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             targetBlockVersion,
			Encoding:            backend.EncNone,
			RowGroupSizeBytes:   30_000_000,
		},
		WAL: &wal.Config{
			Filepath:       path.Join(tempDir, "wal"),
			IngestionSlack: time.Since(time.Unix(0, 0)), // Let us use obvious start/end times below
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	require.NoError(t, err)

	ctx := context.Background()
	c.EnableCompaction(ctx, &CompactorConfig{
		ChunkSizeBytes:          10_000_000,
		FlushSizeBytes:          10_000_000,
		MaxCompactionRange:      24 * time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})

	cutTestBlockWithSnapshots(t, w, testTenantID, []testData{
		{test.ValidSnapshotID(nil), test.GenerateSnapshot(10, nil), 100},
		{test.ValidSnapshotID(nil), test.GenerateSnapshot(10, nil), 102},
	})
	cutTestBlockWithSnapshots(t, w, testTenantID, []testData{
		{test.ValidSnapshotID(nil), test.GenerateSnapshot(10, nil), 104},
		{test.ValidSnapshotID(nil), test.GenerateSnapshot(10, nil), 106},
	})

	rw := r.(*readerWriter)
	rw.pollBlocklist()

	// compact everything
	err = rw.compact(ctx, rw.blocklist.Metas(testTenantID), testTenantID)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// New blocklist contains 1 compacted block with min start and max end
	blocks := rw.blocklist.Metas(testTenantID)
	require.Equal(t, 1, len(blocks))
	require.Equal(t, uint8(1), blocks[0].CompactionLevel)
	require.Equal(t, 100, int(blocks[0].StartTime.Unix()))
	require.Equal(t, 106, int(blocks[0].EndTime.Unix()))
}

type testData struct {
	id    common.ID
	t     *deeptp.Snapshot
	start uint32
}

func cutTestBlockWithSnapshots(t testing.TB, w Writer, tenantID string, data []testData) common.BackendBlock {
	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	walBlock := w.WAL()

	head, err := walBlock.NewBlock(uuid.New(), tenantID, model.CurrentEncoding)
	require.NoError(t, err)

	for _, d := range data {
		writeSnapshotToWal(t, head, dec, d.id, d.t, d.start)
	}

	b, err := w.CompleteBlock(context.Background(), head)
	require.NoError(t, err)

	return b
}
func cutTestBlocks(t testing.TB, w Writer, tenantID string, blockCount int, recordCount int) ([]common.BackendBlock, [][]byte) {
	blocks := make([]common.BackendBlock, 0)
	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)
	var ids [][]byte
	walBlocks := w.WAL()
	for i := 0; i < blockCount; i++ {
		head, err := walBlocks.NewBlock(uuid.New(), tenantID, model.CurrentEncoding)
		require.NoError(t, err)

		for j := 0; j < recordCount; j++ {
			snapshot := test.GenerateSnapshot(j, nil)
			id := snapshot.ID
			ids = append(ids, id)
			now := uint32(time.Now().Unix())
			writeSnapshotToWal(t, head, dec, id, snapshot, now)
		}

		b, err := w.CompleteBlock(context.Background(), head)
		require.NoError(t, err)
		blocks = append(blocks, b)
	}

	return blocks, ids
}

func BenchmarkCompaction(b *testing.B) {
	testEncodings := []string{vparquet.VersionString}
	for _, enc := range testEncodings {
		b.Run(enc, func(b *testing.B) {
			benchmarkCompaction(b, enc)
		})
	}
}

func benchmarkCompaction(b *testing.B, targetBlockVersion string) {
	tempDir := b.TempDir()

	_, w, _, _, c, err := New(&Config{
		Backend: "local",
		Pool: &pool.Config{
			MaxWorkers: 10,
			QueueDepth: 100,
		},
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             targetBlockVersion,
			Encoding:            backend.EncZstd,
			RowGroupSizeBytes:   30_000_000,
		},
		WAL: &wal.Config{
			Filepath: path.Join(tempDir, "wal"),
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	require.NoError(b, err)

	rw := c.(*readerWriter)

	ctx := context.Background()
	c.EnableCompaction(ctx, &CompactorConfig{
		ChunkSizeBytes:     10_000_000,
		FlushSizeBytes:     10_000_000,
		IteratorBufferSize: DefaultIteratorBufferSize,
	}, &mockSharder{}, &mockOverrides{})

	snapshotCount := 20_000
	blockCount := 8

	// Cut input blocks
	blocks, _ := cutTestBlocks(b, w, testTenantID, blockCount, snapshotCount)
	metas := make([]*backend.BlockMeta, 0)
	for _, b := range blocks {
		metas = append(metas, b.BlockMeta())
	}

	b.ResetTimer()

	err = rw.compact(ctx, metas, testTenantID)
	require.NoError(b, err)
}
