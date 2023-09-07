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
	"encoding/binary"
	"testing"
	"time"

	"github.com/segmentio/parquet-go"

	"github.com/go-kit/log"
	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	deep_io "github.com/intergral/deep/pkg/io"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/require"
)

func BenchmarkCompactor(b *testing.B) {
	b.Run("Small", func(b *testing.B) {
		benchmarkCompactor(b, 1000, 100, 100) // 10M snapshot
	})
	b.Run("Medium", func(b *testing.B) {
		benchmarkCompactor(b, 100, 100, 1000) // 10M snapshot
	})
	b.Run("Large", func(b *testing.B) {
		benchmarkCompactor(b, 10, 1000, 1000) // 10M snapshot
	})
}

func benchmarkCompactor(b *testing.B, snapshotCount, batchCount, count int) {
	rawR, rawW, _, err := local.New(&local.Config{
		Path: b.TempDir(),
	})
	require.NoError(b, err)

	r := backend.NewReader(rawR)
	w := backend.NewWriter(rawW)
	ctx := context.Background()
	l := log.NewNopLogger()

	cfg := &common.BlockConfig{
		BloomFP:             0.01,
		BloomShardSizeBytes: 100 * 1024,
		RowGroupSizeBytes:   20_000_000,
	}

	meta := createTestBlock(b, ctx, cfg, r, w, snapshotCount, batchCount, count)

	inputs := []*backend.BlockMeta{meta}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := NewCompactor(common.CompactionOptions{
			BlockConfig:         *cfg,
			OutputBlocks:        1,
			FlushSizeBytes:      30_000_000,
			MaxBytesPerSnapshot: 50_000_000,
		})

		_, err = c.Compact(ctx, l, r, func(*backend.BlockMeta, time.Time) backend.Writer { return w }, inputs)
		require.NoError(b, err)
	}
}

func BenchmarkCompactorDupes(b *testing.B) {
	rawR, rawW, _, err := local.New(&local.Config{
		Path: b.TempDir(),
	})
	require.NoError(b, err)

	r := backend.NewReader(rawR)
	w := backend.NewWriter(rawW)
	ctx := context.Background()
	l := log.NewNopLogger()

	cfg := &common.BlockConfig{
		BloomFP:             0.01,
		BloomShardSizeBytes: 100 * 1024,
		RowGroupSizeBytes:   20_000_000,
	}

	// 1M snapshots
	meta := createTestBlock(b, ctx, cfg, r, w, 10, 1000, 1000)
	inputs := []*backend.BlockMeta{meta, meta}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		c := NewCompactor(common.CompactionOptions{
			BlockConfig:         *cfg,
			OutputBlocks:        1,
			FlushSizeBytes:      30_000_000,
			MaxBytesPerSnapshot: 50_000_000,

			ObjectsWritten:     func(compactionLevel, objects int) {},
			SnapshotsDiscarded: func(snapshotID string, count int) {},
		})

		_, err = c.Compact(ctx, l, r, func(*backend.BlockMeta, time.Time) backend.Writer { return w }, inputs)
		require.NoError(b, err)
	}
}

// createTestBlock with the number of given snapshots and the needed sizes.
// Snapshot IDs are guaranteed to be monotonically increasing so that
// the block will be iterated in order.
// nolint: revive
func createTestBlock(t testing.TB, ctx context.Context, cfg *common.BlockConfig, r backend.Reader, w backend.Writer, snapshotCount, batchCount, count int) *backend.BlockMeta {
	inMeta := &backend.BlockMeta{
		TenantID:     tenantID,
		BlockID:      uuid.New(),
		TotalObjects: snapshotCount,
	}

	sb := newStreamingBlock(ctx, cfg, inMeta, r, w, deep_io.NewBufferedWriter)

	for i := 0; i < snapshotCount; i++ {
		id := make([]byte, 16)
		binary.LittleEndian.PutUint64(id, uint64(i))

		tr := test.GenerateSnapshot(i, &test.GenerateOptions{Id: id})
		trp := snapshotToParquet(id, tr, nil)

		err := sb.Add(trp, 0)
		require.NoError(t, err)
		if sb.EstimatedBufferedBytes() > 20_000_000 {
			_, err := sb.Flush()
			require.NoError(t, err)
		}
	}

	_, err := sb.Complete()
	require.NoError(t, err)

	return sb.meta
}

func TestValueAlloc(t *testing.T) {
	_ = make([]parquet.Value, 1_000_000)
}
