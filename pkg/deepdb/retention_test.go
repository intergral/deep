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
	"path"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/wal"
	"github.com/intergral/deep/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetention(t *testing.T) {
	tempDir := t.TempDir()

	r, w, _, _, c, err := New(&Config{
		Backend: "local",
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             0.01,
			BloomShardSizeBytes: 100_000,
			Version:             encoding.DefaultEncoding().Version(),
			Encoding:            backend.EncLZ4_256k,
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
		MaxCompactionRange:      time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	r.EnablePolling(&mockJobSharder{})

	blockID := uuid.New()

	wal := w.WAL()
	assert.NoError(t, err)

	head, err := wal.NewBlock(blockID, testTenantID, model.CurrentEncoding)
	assert.NoError(t, err)

	complete, err := w.CompleteBlock(context.Background(), head)
	assert.NoError(t, err)
	blockID = complete.BlockMeta().BlockID

	rw := r.(*readerWriter)
	// poll
	checkBlocklists(t, blockID, 1, 0, rw)

	// retention should mark it compacted
	r.(*readerWriter).doRetention(ctx)
	checkBlocklists(t, blockID, 0, 1, rw)

	// retention again should clear it
	r.(*readerWriter).doRetention(ctx)
	checkBlocklists(t, blockID, 0, 0, rw)
}

func TestRetentionUpdatesBlocklistImmediately(t *testing.T) {
	// Test that retention updates the in-memory blocklist
	// immediately to reflect affected blocks and doesn't
	// wait for the next polling cycle.

	tempDir := t.TempDir()

	r, w, _, _, c, err := New(&Config{
		Backend: "local",
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             0.01,
			BloomShardSizeBytes: 100_000,
			Version:             encoding.DefaultEncoding().Version(),
			Encoding:            backend.EncLZ4_256k,
		},
		WAL: &wal.Config{
			Filepath: path.Join(tempDir, "wal"),
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	assert.NoError(t, err)

	r.EnablePolling(&mockJobSharder{})

	c.EnableCompaction(context.Background(), &CompactorConfig{
		ChunkSizeBytes:          10,
		MaxCompactionRange:      time.Hour,
		BlockRetention:          0,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, &mockOverrides{})

	wal := w.WAL()
	assert.NoError(t, err)

	blockID := uuid.New()

	head, err := wal.NewBlock(blockID, testTenantID, model.CurrentEncoding)
	assert.NoError(t, err)

	ctx := context.Background()
	complete, err := w.CompleteBlock(ctx, head)
	assert.NoError(t, err)
	blockID = complete.BlockMeta().BlockID

	// We have a block
	rw := r.(*readerWriter)
	rw.pollBlocklist()
	require.Equal(t, blockID, rw.blocklist.Metas(testTenantID)[0].BlockID)

	// Mark it compacted
	r.(*readerWriter).compactorCfg.BlockRetention = 0 // Immediately delete
	r.(*readerWriter).compactorCfg.CompactedBlockRetention = time.Hour
	r.(*readerWriter).doRetention(ctx)

	// Immediately compacted
	require.Empty(t, rw.blocklist.Metas(testTenantID))
	require.Equal(t, blockID, rw.blocklist.CompactedMetas(testTenantID)[0].BlockID)

	// Now delete it permanently
	r.(*readerWriter).compactorCfg.BlockRetention = time.Hour
	r.(*readerWriter).compactorCfg.CompactedBlockRetention = 0 // Immediately delete
	r.(*readerWriter).doRetention(ctx)

	require.Empty(t, rw.blocklist.Metas(testTenantID))
	require.Empty(t, rw.blocklist.CompactedMetas(testTenantID))
}

func TestBlockRetentionOverride(t *testing.T) {
	tempDir := t.TempDir()

	r, w, _, _, c, err := New(&Config{
		Backend: "local",
		Local: &local.Config{
			Path: path.Join(tempDir, "snapshots"),
		},
		Block: &common.BlockConfig{
			BloomFP:             0.01,
			BloomShardSizeBytes: 100_000,
			Version:             encoding.DefaultEncoding().Version(),
			Encoding:            backend.EncLZ4_256k,
		},
		WAL: &wal.Config{
			Filepath: path.Join(tempDir, "wal"),
		},
		BlocklistPoll: 0,
	}, log.NewNopLogger())
	require.NoError(t, err)

	overrides := &mockOverrides{}

	ctx := context.Background()
	c.EnableCompaction(ctx, &CompactorConfig{
		ChunkSizeBytes:          10,
		MaxCompactionRange:      time.Hour,
		BlockRetention:          time.Hour,
		CompactedBlockRetention: 0,
	}, &mockSharder{}, overrides)

	r.EnablePolling(&mockJobSharder{})

	cutTestBlocks(t, w, testTenantID, 10, 10)

	// The test snapshots are all 1 second long, so we have to sleep to put all the
	// data in the past
	time.Sleep(time.Second)

	rw := r.(*readerWriter)
	rw.pollBlocklist()
	require.Equal(t, 10, len(rw.blocklist.Metas(testTenantID)))

	// Retention = 1 hour, does nothing
	overrides.blockRetention = time.Hour
	r.(*readerWriter).doRetention(ctx)
	rw.pollBlocklist()
	require.Equal(t, 10, len(rw.blocklist.Metas(testTenantID)))

	// Retention = 1 minute, still does nothing
	overrides.blockRetention = time.Minute
	r.(*readerWriter).doRetention(ctx)
	rw.pollBlocklist()
	require.Equal(t, 10, len(rw.blocklist.Metas(testTenantID)))

	// Retention = 1ns, deletes everything
	overrides.blockRetention = time.Nanosecond
	r.(*readerWriter).doRetention(ctx)
	rw.pollBlocklist()
	require.Equal(t, 0, len(rw.blocklist.Metas(testTenantID)))
}
