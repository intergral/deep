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
	"math/rand"
	"path"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
	"github.com/intergral/deep/pkg/deepdb/wal"
	"github.com/intergral/deep/pkg/deeppb"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/model"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/require"
)

func TestSimpleDeepQL(t *testing.T) {
	runBlockQLTest(t, vparquet.VersionString, func(snapshot *deeptp.Snapshot, wantMeta *deeppb.SnapshotSearchMetadata, meta *backend.BlockMeta, reader Reader) {
		req := &deeppb.SearchRequest{Query: "{}"}
		res, err := reader.Search(context.Background(), meta, req, common.DefaultSearchOptions())
		require.NoError(t, err, "search request: %+v", req)
		require.Equal(t, wantMeta, actualForExpectedMeta(wantMeta, res), "search request: %v", req)
	})
}

type testFunc func(*deeptp.Snapshot, *deeppb.SnapshotSearchMetadata, *backend.BlockMeta, Reader)

func runBlockQLTest(t testing.TB, blockVersion string, runner testFunc) {
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

	wantID, wantTr, start, wantMeta := createTestData()

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
			tr = test.GenerateSnapshot(10, &test.GenerateOptions{Id: id})
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

	runner(wantTr, wantMeta, meta, rw)

	// todo: do some compaction and then call runner again
}

func createTestData() (id []byte,
	tr *deeptp.Snapshot,
	start uint32,
	expected *deeppb.SnapshotSearchMetadata,
) {
	snapshot := test.GenerateSnapshot(101, &test.GenerateOptions{ServiceName: "test-data"})

	return snapshot.ID, snapshot, uint32(snapshot.TsNanos / 1e9), &deeppb.SnapshotSearchMetadata{
		SnapshotID:        util.SnapshotIDToHexString(snapshot.ID),
		ServiceName:       "test-data",
		FilePath:          snapshot.Tracepoint.Path,
		LineNo:            snapshot.Tracepoint.LineNumber,
		StartTimeUnixNano: snapshot.TsNanos,
		DurationNano:      snapshot.DurationNanos,
	}
}
