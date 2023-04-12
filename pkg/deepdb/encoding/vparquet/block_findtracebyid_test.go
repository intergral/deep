package vparquet

import (
	"bytes"
	"context"
	"github.com/segmentio/parquet-go"
	"path"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	deep_io "github.com/intergral/deep/pkg/io"
	"github.com/intergral/deep/pkg/util/test"
)

func TestBackendBlockFindTraceByID(t *testing.T) {
	rawR, rawW, _, err := local.New(&local.Config{
		Path: t.TempDir(),
	})
	require.NoError(t, err)

	r := backend.NewReader(rawR)
	w := backend.NewWriter(rawW)
	ctx := context.Background()

	cfg := &common.BlockConfig{
		BloomFP:             0.01,
		BloomShardSizeBytes: 100 * 1024,
	}

	// Test data - sorted by trace ID
	// Find trace by ID uses the column and page bounds,
	// which by default only stores 16 bytes, which is the first
	// half of the trace ID (which is stored as 32 hex text)
	// Therefore it is important that the test data here has
	// full-length trace IDs.
	var snapshots []*Snapshot
	for i := 0; i < 16; i++ {
		bar := "bar"
		snapshots = append(snapshots, &Snapshot{
			ID:       test.ValidSnapshotID(nil),
			Resource: Resource{ServiceName: "s"},
			Attributes: []Attribute{
				{Key: "foo", Value: &bar},
			},
		})
	}

	// Sort
	sort.Slice(snapshots, func(i, j int) bool {
		return bytes.Compare(snapshots[i].ID, snapshots[j].ID) == -1
	})

	meta := backend.NewBlockMeta("fake", uuid.New(), VersionString, backend.EncNone, "")
	meta.TotalObjects = len(snapshots)
	s := newStreamingBlock(ctx, cfg, meta, r, w, deep_io.NewBufferedWriter)

	// Write test data, occasionally flushing (cutting new row group)
	rowGroupSize := 5
	for _, snap := range snapshots {
		err := s.Add(snap, 0)
		require.NoError(t, err)
		if s.CurrentBufferedObjects() >= rowGroupSize {
			_, err = s.Flush()
			require.NoError(t, err)
		}
	}
	_, err = s.Complete()
	require.NoError(t, err)

	b := newBackendBlock(s.meta, r)

	// Now find and verify all test traces
	for _, snap := range snapshots {
		wantProto := parquetToDeepSnapshot(snap)

		gotProto, err := b.FindSnapshotByID(ctx, snap.ID, common.DefaultSearchOptions())
		require.NoError(t, err)
		require.Equal(t, wantProto.ID, gotProto.ID)
		require.Equal(t, wantProto.Resource[0].Value.GetStringValue(), "s")
		require.Equal(t, wantProto.Attributes[0].Value.GetStringValue(), "bar")
	}
}

func TestBackendBlockFindTraceByID_TestData(t *testing.T) {
	rawR, _, _, err := local.New(&local.Config{
		Path: "./test-data",
	})
	require.NoError(t, err)

	r := backend.NewReader(rawR)
	ctx := context.Background()

	blocks, err := r.Blocks(ctx, "single-tenant")
	require.NoError(t, err)
	assert.Len(t, blocks, 1)

	meta, err := r.BlockMeta(ctx, blocks[0], "single-tenant")
	require.NoError(t, err)

	b := newBackendBlock(meta, r)

	iter, err := b.RawIterator(context.Background(), newRowPool(10))
	require.NoError(t, err)

	sch := parquet.SchemaOf(new(Snapshot))
	for {
		_, row, err := iter.Next(context.Background())
		require.NoError(t, err)

		if row == nil {
			break
		}

		tr := &Snapshot{}
		err = sch.Reconstruct(tr, row)
		require.NoError(t, err)

		protoTr, err := b.FindSnapshotByID(ctx, tr.ID, common.DefaultSearchOptions())
		require.NoError(t, err)
		require.NotNil(t, protoTr)
	}
}

func BenchmarkFindTraceByID(b *testing.B) {
	ctx := context.TODO()
	tenantID := "1"
	blockID := uuid.MustParse("3685ee3d-cbbf-4f36-bf28-93447a19dea6")
	// blockID := uuid.MustParse("1a2d50d7-f10e-41f0-850d-158b19ead23d")

	r, _, _, err := local.New(&local.Config{
		Path: path.Join("/Users/marty/src/tmp/"),
	})
	require.NoError(b, err)

	rr := backend.NewReader(r)

	meta, err := rr.BlockMeta(ctx, blockID, tenantID)
	require.NoError(b, err)

	traceID := meta.MinID
	// traceID, err := util.HexStringToTraceID("1a029f7ace79c7f2")
	// require.NoError(b, err)

	block := newBackendBlock(meta, rr)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tr, err := block.FindSnapshotByID(ctx, traceID, common.DefaultSearchOptions())
		require.NoError(b, err)
		require.NotNil(b, tr)
	}
}
