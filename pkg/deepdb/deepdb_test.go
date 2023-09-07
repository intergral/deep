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
	"os"
	"path"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
	"github.com/intergral/deep/pkg/deepdb/wal"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/model"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTenantID  = "fake"
	testTenantID2 = "fake2"
)

type testConfigOption func(*Config)

func testConfig(t *testing.T, enc backend.Encoding, blocklistPoll time.Duration, opts ...testConfigOption) (Reader, Writer, Compactor, string) {
	tempDir := t.TempDir()

	cfg := &Config{
		Backend: "local",
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             encoding.DefaultEncoding().Version(),
			Encoding:            enc,
		},
		WAL: &wal.Config{
			Filepath: path.Join(tempDir, "wal"),
		},
		BlocklistPoll: blocklistPoll,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	r, w, _, _, c, err := New(cfg, log.NewNopLogger())
	require.NoError(t, err)
	return r, w, c, tempDir
}

func TestDB(t *testing.T) {
	r, w, c, _ := testConfig(t, backend.EncGZIP, 0)

	c.EnableCompaction(context.Background(), &CompactorConfig{
		ChunkSizeBytes:          10,
		MaxCompactionRange:      time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})

	blockID := uuid.New()

	wal := w.WAL()

	head, err := wal.NewBlock(blockID, testTenantID, model.CurrentEncoding)
	assert.NoError(t, err)

	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	// write
	numMsgs := 10
	reqs := make([]*deeptp.Snapshot, numMsgs)
	ids := make([]common.ID, numMsgs)
	for i := 0; i < numMsgs; i++ {
		ids[i] = test.ValidSnapshotID(nil)
		reqs[i] = test.GenerateSnapshot(i, &test.GenerateOptions{Id: ids[i]})
		writeSnapshotToWal(t, head, dec, ids[i], reqs[i], 0)
	}

	_, err = w.CompleteBlock(context.Background(), head)
	assert.NoError(t, err)

	// poll
	r.(*readerWriter).pollBlocklist()

	// read
	for i, id := range ids {
		bFound, failedBlocks, err := r.FindSnapshot(context.Background(), testTenantID, id, BlockIDMin, BlockIDMax, 0, 0)
		assert.NoError(t, err)
		assert.Nil(t, failedBlocks)
		assert.True(t, proto.Equal(bFound, reqs[i]))
	}
}

func TestBlockSharding(t *testing.T) {
	// push a req with some ID
	// cut headblock & write to backend
	// search with different shards and check if its respecting the params
	r, w, _, _ := testConfig(t, backend.EncLZ4_256k, 0)

	r.EnablePolling(&mockJobSharder{})

	// create block with known ID
	blockID := uuid.New()
	wal := w.WAL()

	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)
	head, err := wal.NewBlock(blockID, testTenantID, model.CurrentEncoding)
	assert.NoError(t, err)

	// add a snapshot to the block
	req := test.GenerateSnapshot(1, nil)
	writeSnapshotToWal(t, head, dec, req.ID, req, 0)

	// write block to backend
	_, err = w.CompleteBlock(context.Background(), head)
	assert.NoError(t, err)

	// poll
	r.(*readerWriter).pollBlocklist()

	// get blockID
	blocks := r.(*readerWriter).blocklist.Metas(testTenantID)
	assert.Len(t, blocks, 1)

	// check if it respects the blockstart/blockend params - case1: hit
	blockStart := uuid.MustParse(BlockIDMin).String()
	blockEnd := uuid.MustParse(BlockIDMax).String()
	bFound, failedBlocks, err := r.FindSnapshot(context.Background(), testTenantID, req.ID, blockStart, blockEnd, 0, 0)
	assert.NoError(t, err)
	assert.Nil(t, failedBlocks)
	assert.True(t, proto.Equal(bFound, req))

	// check if it respects the blockstart/blockend params - case2: miss
	blockStart = uuid.MustParse(BlockIDMin).String()
	blockEnd = uuid.MustParse(BlockIDMin).String()
	bFound, failedBlocks, err = r.FindSnapshot(context.Background(), testTenantID, req.ID, blockStart, blockEnd, 0, 0)
	assert.NoError(t, err)
	assert.Nil(t, failedBlocks)
}

func TestNilOnUnknownTenantID(t *testing.T) {
	r, _, _, _ := testConfig(t, backend.EncLZ4_256k, 0)

	buff, failedBlocks, err := r.FindSnapshot(context.Background(), "unknown", []byte{0x01}, BlockIDMin, BlockIDMax, 0, 0)
	assert.Nil(t, buff)
	assert.Nil(t, err)
	assert.Nil(t, failedBlocks)
}

func TestBlockCleanup(t *testing.T) {
	r, w, c, tempDir := testConfig(t, backend.EncLZ4_256k, 0)

	c.EnableCompaction(context.Background(), &CompactorConfig{
		ChunkSizeBytes:          10,
		MaxCompactionRange:      time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})

	blockID := uuid.New()

	wal := w.WAL()

	head, err := wal.NewBlock(blockID, testTenantID, model.CurrentEncoding)
	assert.NoError(t, err)

	_, err = w.CompleteBlock(context.Background(), head)
	assert.NoError(t, err)

	rw := r.(*readerWriter)

	// poll
	rw.pollBlocklist()

	assert.Len(t, rw.blocklist.Metas(testTenantID), 1)

	os.RemoveAll(tempDir + "/snapshots/" + testTenantID)

	// poll
	rw.pollBlocklist()

	m := rw.blocklist.Metas(testTenantID)
	assert.Equal(t, 0, len(m))
}

func checkBlocklists(t *testing.T, expectedID uuid.UUID, expectedB int, expectedCB int, rw *readerWriter) {
	rw.pollBlocklist()

	blocklist := rw.blocklist.Metas(testTenantID)
	require.Len(t, blocklist, expectedB)
	if expectedB > 0 && expectedID != uuid.Nil {
		assert.Equal(t, expectedID, blocklist[0].BlockID)
	}

	// confirm blocklists are in starttime ascending order
	lastTime := time.Time{}
	for _, b := range blocklist {
		assert.True(t, lastTime.Before(b.StartTime) || lastTime.Equal(b.StartTime))
		lastTime = b.StartTime
	}

	compactedBlocklist := rw.blocklist.CompactedMetas(testTenantID)
	assert.Len(t, compactedBlocklist, expectedCB)
	if expectedCB > 0 && expectedID != uuid.Nil {
		assert.Equal(t, expectedID, compactedBlocklist[0].BlockID)
	}

	lastTime = time.Time{}
	for _, b := range compactedBlocklist {
		assert.True(t, lastTime.Before(b.StartTime) || lastTime.Equal(b.StartTime))
		lastTime = b.StartTime
	}
}

func TestIncludeBlock(t *testing.T) {
	tests := []struct {
		name       string
		searchID   common.ID
		blockStart uuid.UUID
		blockEnd   uuid.UUID
		start      int64
		end        int64
		meta       *backend.BlockMeta
		expected   bool
	}{
		// includes
		{
			name:       "include - duh",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse(BlockIDMin),
			blockEnd:   uuid.MustParse(BlockIDMax),
			meta: &backend.BlockMeta{
				BlockID: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
				MinID:   []byte{0x00},
				MaxID:   []byte{0x10},
			},
			start:    0,
			end:      0,
			expected: true,
		},
		{
			name:       "include - min id range",
			searchID:   []byte{0x00},
			blockStart: uuid.MustParse(BlockIDMin),
			blockEnd:   uuid.MustParse(BlockIDMax),
			meta: &backend.BlockMeta{
				BlockID: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
				MinID:   []byte{0x00},
				MaxID:   []byte{0x10},
			},
			start:    0,
			end:      0,
			expected: true,
		},
		{
			name:       "include - max id range",
			searchID:   []byte{0x10},
			blockStart: uuid.MustParse(BlockIDMin),
			blockEnd:   uuid.MustParse(BlockIDMax),
			meta: &backend.BlockMeta{
				BlockID: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
				MinID:   []byte{0x00},
				MaxID:   []byte{0x10},
			},
			start:    0,
			end:      0,
			expected: true,
		},
		{
			name:       "include - min block range",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
			blockEnd:   uuid.MustParse(BlockIDMax),
			meta: &backend.BlockMeta{
				BlockID: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
				MinID:   []byte{0x00},
				MaxID:   []byte{0x10},
			},
			start:    0,
			end:      0,
			expected: true,
		},
		{
			name:       "include - max block range",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse(BlockIDMin),
			blockEnd:   uuid.MustParse("50000000-0000-0000-0000-000000000000"),
			meta: &backend.BlockMeta{
				BlockID:   uuid.MustParse("50000000-0000-0000-0000-000000000000"),
				MinID:     []byte{0x00},
				MaxID:     []byte{0x10},
				StartTime: time.Unix(10000, 0),
				EndTime:   time.Unix(20000, 0),
			},
			start:    10000,
			end:      20000,
			expected: true,
		},
		{
			name:       "include - max block range",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse(BlockIDMin),
			blockEnd:   uuid.MustParse("50000000-0000-0000-0000-000000000000"),
			meta: &backend.BlockMeta{
				BlockID:   uuid.MustParse("50000000-0000-0000-0000-000000000000"),
				MinID:     []byte{0x00},
				MaxID:     []byte{0x10},
				StartTime: time.Unix(1650285326, 0),
				EndTime:   time.Unix(1650288990, 0),
			},
			start:    10000,
			end:      20000,
			expected: false,
		},
		{
			name:       "include - exact hit",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
			blockEnd:   uuid.MustParse("50000000-0000-0000-0000-000000000000"),
			meta: &backend.BlockMeta{
				BlockID: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
				MinID:   []byte{0x05},
				MaxID:   []byte{0x05},
			},
			start:    0,
			end:      0,
			expected: true,
		},
		// excludes
		{
			name:       "exclude - duh",
			searchID:   []byte{0x20},
			blockStart: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
			blockEnd:   uuid.MustParse("51000000-0000-0000-0000-000000000000"),
			meta: &backend.BlockMeta{
				BlockID: uuid.MustParse("52000000-0000-0000-0000-000000000000"),
				MinID:   []byte{0x00},
				MaxID:   []byte{0x10},
			},
		},
		// todo: restore when this is fixed: https://github.com/intergral/deep/issues/1903
		// {
		// 	name:       "exclude - min id range",
		// 	searchID:   []byte{0x00},
		// 	blockStart: uuid.MustParse(BlockIDMin),
		// 	blockEnd:   uuid.MustParse(BlockIDMax),
		// 	meta: &backend.BlockMeta{
		// 		BlockID: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
		// 		MinID:   []byte{0x01},
		// 		MaxID:   []byte{0x10},
		// 	},
		// },
		// {
		// 	name:       "exclude - max id range",
		// 	searchID:   []byte{0x11},
		// 	blockStart: uuid.MustParse(BlockIDMin),
		// 	blockEnd:   uuid.MustParse(BlockIDMax),
		// 	meta: &backend.BlockMeta{
		// 		BlockID: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
		// 		MinID:   []byte{0x01},
		// 		MaxID:   []byte{0x10},
		// 	},
		// },
		{
			name:       "exclude - min block range",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
			blockEnd:   uuid.MustParse("51000000-0000-0000-0000-000000000000"),
			meta: &backend.BlockMeta{
				BlockID: uuid.MustParse("4FFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"),
				MinID:   []byte{0x00},
				MaxID:   []byte{0x10},
			},
		},
		{
			name:       "exclude - max block range",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
			blockEnd:   uuid.MustParse("51000000-0000-0000-0000-000000000000"),
			meta: &backend.BlockMeta{
				BlockID: uuid.MustParse("51000000-0000-0000-0000-000000000001"),
				MinID:   []byte{0x00},
				MaxID:   []byte{0x10},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := tc.blockStart.MarshalBinary()
			require.NoError(t, err)
			e, err := tc.blockEnd.MarshalBinary()
			require.NoError(t, err)

			assert.Equal(t, tc.expected, includeBlock(tc.meta, tc.searchID, s, e, tc.start, tc.end))
		})
	}
}

func TestIncludeCompactedBlock(t *testing.T) {
	blocklistPoll := 5 * time.Minute
	tests := []struct {
		name       string
		searchID   common.ID
		blockStart uuid.UUID
		blockEnd   uuid.UUID
		meta       *backend.CompactedBlockMeta
		start      int64
		end        int64
		expected   bool
	}{
		{
			name:       "include recent",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse(BlockIDMin),
			blockEnd:   uuid.MustParse(BlockIDMax),
			start:      0,
			end:        0,
			meta: &backend.CompactedBlockMeta{
				BlockMeta: backend.BlockMeta{
					BlockID: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
					MinID:   []byte{0x00},
					MaxID:   []byte{0x10},
				},
				CompactedTime: time.Now().Add(-(1 * blocklistPoll)),
			},
			expected: true,
		},
		{
			name:       "skip old",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse(BlockIDMin),
			blockEnd:   uuid.MustParse(BlockIDMax),
			start:      0,
			end:        0,
			meta: &backend.CompactedBlockMeta{
				BlockMeta: backend.BlockMeta{
					BlockID: uuid.MustParse("50000000-0000-0000-0000-000000000000"),
					MinID:   []byte{0x00},
					MaxID:   []byte{0x10},
				},
				CompactedTime: time.Now().Add(-(3 * blocklistPoll)),
			},
			expected: false,
		},
		{
			name:       "skip recent but out of range",
			searchID:   []byte{0x05},
			blockStart: uuid.MustParse("40000000-0000-0000-0000-000000000000"),
			blockEnd:   uuid.MustParse("50000000-0000-0000-0000-000000000000"),
			start:      0,
			end:        0,
			meta: &backend.CompactedBlockMeta{
				BlockMeta: backend.BlockMeta{
					BlockID: uuid.MustParse("51000000-0000-0000-0000-000000000000"),
					MinID:   []byte{0x00},
					MaxID:   []byte{0x10},
				},
				CompactedTime: time.Now().Add(-(1 * blocklistPoll)),
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := tc.blockStart.MarshalBinary()
			require.NoError(t, err)
			e, err := tc.blockEnd.MarshalBinary()
			require.NoError(t, err)

			assert.Equal(t, tc.expected, includeCompactedBlock(tc.meta, tc.searchID, s, e, blocklistPoll, tc.start, tc.end))
		})
	}
}

func TestSearchCompactedBlocks(t *testing.T) {
	r, w, c, _ := testConfig(t, backend.EncLZ4_256k, time.Hour)

	c.EnableCompaction(context.Background(), &CompactorConfig{
		ChunkSizeBytes:          10,
		MaxCompactionRange:      time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})

	wal := w.WAL()

	head, err := wal.NewBlock(uuid.New(), testTenantID, model.CurrentEncoding)
	assert.NoError(t, err)

	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	// write
	numMsgs := 10
	reqs := make([]*deeptp.Snapshot, 0, numMsgs)
	ids := make([][]byte, 0, numMsgs)
	for i := 0; i < numMsgs; i++ {
		id := test.ValidSnapshotID(nil)
		req := test.GenerateSnapshot(i, &test.GenerateOptions{Id: id})
		writeSnapshotToWal(t, head, dec, id, req, 0)
		reqs = append(reqs, req)
		ids = append(ids, id)
	}

	ctx := context.Background()
	complete, err := w.CompleteBlock(ctx, head)
	require.NoError(t, err)

	blockID := complete.BlockMeta().BlockID.String()

	rw := r.(*readerWriter)

	// poll
	rw.pollBlocklist()

	// read
	for i, id := range ids {
		bFound, failedBlocks, err := r.FindSnapshot(ctx, testTenantID, id, blockID, blockID, 0, 0)
		require.NoError(t, err)
		require.Nil(t, failedBlocks)
		require.True(t, proto.Equal(bFound, reqs[i]))
	}

	// compact
	var blockMetas []*backend.BlockMeta
	blockMetas = append(blockMetas, complete.BlockMeta())
	require.NoError(t, rw.compact(ctx, blockMetas, testTenantID))

	// poll
	rw.pollBlocklist()

	// make sure the block is compacted
	compactedBlocks := rw.blocklist.CompactedMetas(testTenantID)
	require.Len(t, compactedBlocks, 1)
	require.Equal(t, compactedBlocks[0].BlockID.String(), blockID)
	blocks := rw.blocklist.Metas(testTenantID)
	require.Len(t, blocks, 1)
	require.NotEqual(t, blocks[0].BlockID.String(), blockID)

	// find should succeed with old block range
	for i, id := range ids {
		bFound, failedBlocks, err := r.FindSnapshot(ctx, testTenantID, id, blockID, blockID, 0, 0)
		require.NoError(t, err)
		require.Nil(t, failedBlocks)
		require.True(t, proto.Equal(bFound, reqs[i]))
	}
}

func TestCompleteBlock(t *testing.T) {
	for _, from := range encoding.AllEncodings() {
		for _, to := range encoding.AllEncodings() {
			t.Run(fmt.Sprintf("%s->%s", from.Version(), to.Version()), func(t *testing.T) {
				testCompleteBlock(t, from.Version(), to.Version())
			})
		}
	}
}

func testCompleteBlock(t *testing.T, from, to string) {
	_, w, _, _ := testConfig(t, backend.EncLZ4_256k, time.Minute, func(c *Config) {
		c.Block.Version = from // deeprarily set config to from while we create the wal, so it makes blocks in the "from" format
	})

	wal := w.WAL()
	rw := w.(*readerWriter)
	rw.cfg.Block.Version = to // now set it back so we cut blocks in the "to" format

	blockID := uuid.New()

	block, err := wal.NewBlock(blockID, testTenantID, model.CurrentEncoding)
	require.NoError(t, err, "unexpected error creating block")
	require.Equal(t, block.BlockMeta().Version, from)

	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	numMsgs := 100
	reqs := make([]*deeptp.Snapshot, 0, numMsgs)
	ids := make([][]byte, 0, numMsgs)
	for i := 0; i < numMsgs; i++ {
		id := test.ValidSnapshotID(nil)
		req := test.GenerateSnapshot(i, &test.GenerateOptions{Id: id})
		writeSnapshotToWal(t, block, dec, id, req, 0)
		reqs = append(reqs, req)
		ids = append(ids, id)
	}
	require.NoError(t, block.Flush())

	complete, err := w.CompleteBlock(context.Background(), block)
	require.NoError(t, err, "unexpected error completing block")
	require.Equal(t, complete.BlockMeta().Version, to)

	for i, id := range ids {
		found, err := complete.FindSnapshotByID(context.TODO(), id, common.DefaultSearchOptions())
		require.NoError(t, err)
		require.NotNil(t, found)
		require.True(t, proto.Equal(found, reqs[i]))
	}
}

func TestCompleteBlockHonorsStartStopTimes(t *testing.T) {
	testEncodings := []string{vparquet.VersionString}
	for _, enc := range testEncodings {
		t.Run(enc, func(t *testing.T) {
			testCompleteBlockHonorsStartStopTimes(t, enc)
		})
	}
}

func testCompleteBlockHonorsStartStopTimes(t *testing.T, targetBlockVersion string) {
	tempDir := t.TempDir()

	_, w, _, _, _, err := New(&Config{
		Backend: "local",
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             targetBlockVersion,
			Encoding:            backend.EncNone,
		},
		WAL: &wal.Config{
			IngestionSlack: time.Minute,
			Filepath:       path.Join(tempDir, "wal"),
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	require.NoError(t, err)

	dec := model.MustNewSegmentDecoder(model.CurrentEncoding)

	wal := w.WAL()

	oneHourAgo := time.Now().Add(-1 * time.Hour).Unix()

	block, err := wal.NewBlock(uuid.New(), testTenantID, model.CurrentEncoding)
	require.NoError(t, err, "unexpected error creating block")

	// Write a snapshot from 1 hour ago.
	// The wal slack time will adjust it to 1 minute ago
	req := test.GenerateSnapshot(10, nil)
	writeSnapshotToWal(t, block, dec, req.ID, req, uint32(oneHourAgo))

	complete, err := w.CompleteBlock(context.Background(), block)
	require.NoError(t, err, "unexpected error completing block")

	// Verify the block time was constrained to the slack time.
	require.Equal(t, oneHourAgo, complete.BlockMeta().StartTime.Unix())
	require.Equal(t, oneHourAgo, complete.BlockMeta().EndTime.Unix())
}

func TestShouldCache(t *testing.T) {
	tempDir := t.TempDir()

	r, _, _, _, _, err := New(&Config{
		Backend: "local",
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             .01,
			BloomShardSizeBytes: 100_000,
			Version:             encoding.DefaultEncoding().Version(),
			Encoding:            backend.EncLZ4_256k,
		},
		WAL: &wal.Config{
			Filepath: path.Join(tempDir, "wal"),
		},
		BlocklistPoll:           0,
		CacheMaxBlockAge:        time.Hour,
		CacheMinCompactionLevel: 1,
	}, log.NewNopLogger())
	require.NoError(t, err)

	rw := r.(*readerWriter)

	testCases := []struct {
		name            string
		compactionLevel uint8
		startTime       time.Time
		cache           bool
	}{
		{
			name:            "both pass",
			compactionLevel: 1,
			startTime:       time.Now(),
			cache:           true,
		},
		{
			name:            "startTime fail",
			compactionLevel: 2,
			startTime:       time.Now().Add(-2 * time.Hour),
			cache:           false,
		},
		{
			name:            "compactionLevel fail",
			compactionLevel: 0,
			startTime:       time.Now(),
			cache:           false,
		},
		{
			name:            "both fail",
			compactionLevel: 0,
			startTime:       time.Now(),
			cache:           false,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.cache, rw.shouldCache(&backend.BlockMeta{CompactionLevel: tt.compactionLevel, StartTime: tt.startTime}, time.Now()))
		})
	}
}

func writeSnapshotToWal(t require.TestingT, b common.WALBlock, dec model.SegmentDecoder, id common.ID, snapshot *deeptp.Snapshot, start uint32) {
	b1, err := dec.PrepareForWrite(snapshot, 0)
	require.NoError(t, err)

	b2, err := dec.ToObject(b1)
	require.NoError(t, err)

	err = b.Append(id, b2, start)
	require.NoError(t, err, "unexpected error writing req")
}
