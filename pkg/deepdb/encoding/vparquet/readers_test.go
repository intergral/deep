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
	"io"
	"testing"

	"github.com/segmentio/parquet-go"

	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
)

var tenantID = "single-tenant"

type dummyReader struct {
	r           io.ReaderAt
	footer      bool
	columnIndex bool
	offsetIndex bool
}

func (d *dummyReader) ReadAt(p []byte, off int64) (int, error) { return d.r.ReadAt(p, off) }

func (d *dummyReader) SetFooterSection(_ int64, _ int64)      { d.footer = true }
func (d *dummyReader) SetColumnIndexSection(_ int64, _ int64) { d.columnIndex = true }
func (d *dummyReader) SetOffsetIndexSection(_ int64, _ int64) { d.offsetIndex = true }

// TestParquetGoSetsMetadataSections tests if the special metadata sections are set correctly for caching.
// It is the best way right now to ensure that the interface used by the underlying parquet-go library does not drift.
// If this test starts failing at some point, we should update the interface used by `parquetOptimizedReaderAt` to match
// the specification in parquet-go
func TestParquetGoSetsMetadataSections(t *testing.T) {
	rawR, _, _, err := local.New(&local.Config{
		Path: "./test-data",
	})
	require.NoError(t, err)

	r := backend.NewReader(rawR)
	ctx := context.Background()

	blocks, err := r.Blocks(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	meta, err := r.BlockMeta(ctx, blocks[0], tenantID)
	require.NoError(t, err)

	br := NewBackendReaderAt(ctx, r, DataFileName, meta.BlockID, tenantID)
	dr := &dummyReader{r: br}
	_, err = parquet.OpenFile(dr, int64(meta.Size))
	require.NoError(t, err)

	require.True(t, dr.footer)
	require.True(t, dr.columnIndex)
	require.True(t, dr.offsetIndex)
}

func TestParquetReaderAt(t *testing.T) {
	rr := &recordingReaderAt{}
	pr := newParquetOptimizedReaderAt(rr, 1000, 100)

	expectedReads := []read{}

	// magic number doesn't pass through
	_, err := pr.ReadAt(make([]byte, 4), 0)
	require.NoError(t, err)

	// footer size doesn't pass through
	_, err = pr.ReadAt(make([]byte, 8), 992)
	require.NoError(t, err)

	// other calls pass through
	_, err = pr.ReadAt(make([]byte, 13), 25)
	require.NoError(t, err)
	expectedReads = append(expectedReads, read{13, 25})

	_, err = pr.ReadAt(make([]byte, 97), 118)
	require.NoError(t, err)
	expectedReads = append(expectedReads, read{97, 118})

	_, err = pr.ReadAt(make([]byte, 59), 421)
	require.NoError(t, err)
	expectedReads = append(expectedReads, read{59, 421})

	require.Equal(t, expectedReads, rr.reads)
}

func TestCachingReaderAt(t *testing.T) {
	rawR, _, _, err := local.New(&local.Config{
		Path: "./test-data",
	})
	require.NoError(t, err)

	r := backend.NewReader(rawR)
	ctx := context.Background()

	blocks, err := r.Blocks(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	meta, err := r.BlockMeta(ctx, blocks[0], tenantID)
	require.NoError(t, err)

	br := NewBackendReaderAt(ctx, r, DataFileName, meta.BlockID, tenantID)
	rr := &recordingReaderAt{}

	cr := newCachedReaderAt(rr, br, common.CacheControl{Footer: true, ColumnIndex: true, OffsetIndex: true})

	// cached items should not hit rr
	cr.SetColumnIndexSection(1, 34)
	_, err = cr.ReadAt(make([]byte, 34), 1)
	require.NoError(t, err)

	cr.SetFooterSection(14, 20)
	_, err = cr.ReadAt(make([]byte, 20), 14)
	require.NoError(t, err)

	cr.SetOffsetIndexSection(13, 12)
	_, err = cr.ReadAt(make([]byte, 12), 13)
	require.NoError(t, err)

	// other calls hit rr
	expectedReads := []read{}

	_, err = cr.ReadAt(make([]byte, 13), 25)
	require.NoError(t, err)
	expectedReads = append(expectedReads, read{13, 25})

	_, err = cr.ReadAt(make([]byte, 97), 118)
	require.NoError(t, err)
	expectedReads = append(expectedReads, read{97, 118})

	_, err = cr.ReadAt(make([]byte, 59), 421)
	require.NoError(t, err)
	expectedReads = append(expectedReads, read{59, 421})

	require.Equal(t, expectedReads, rr.reads)
}

type read struct {
	len int
	off int64
}
type recordingReaderAt struct {
	reads []read
}

func (r *recordingReaderAt) ReadAt(p []byte, off int64) (n int, err error) {
	r.reads = append(r.reads, read{len(p), off})

	return len(p), nil
}
