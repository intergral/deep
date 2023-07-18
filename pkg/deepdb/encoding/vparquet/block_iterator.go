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
	"fmt"
	"github.com/segmentio/parquet-go"
	"io"

	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	deepIO "github.com/intergral/deep/pkg/io"
	"github.com/intergral/deep/pkg/parquetquery"
	"github.com/pkg/errors"
)

func (b *backendBlock) open(ctx context.Context) (*parquet.File, *parquet.Reader, error) { //nolint:all //deprecated
	rr := NewBackendReaderAt(ctx, b.r, DataFileName, b.meta.BlockID, b.meta.TenantID)

	// 128 MB memory buffering
	br := deepIO.NewBufferedReaderAt(rr, int64(b.meta.Size), 2*1024*1024, 64)

	pf, err := parquet.OpenFile(br, int64(b.meta.Size), parquet.SkipBloomFilters(true), parquet.SkipPageIndex(true))
	if err != nil {
		return nil, nil, err
	}

	r := parquet.NewReader(pf, parquet.SchemaOf(&Snapshot{}))
	return pf, r, nil
}

func (b *backendBlock) RawIterator(ctx context.Context, pool *rowPool) (*rawIterator, error) {
	pf, r, err := b.open(ctx)
	if err != nil {
		return nil, err
	}

	traceIDIndex, _ := parquetquery.GetColumnIndexByPath(pf, SnapshotIDColumnName)
	if traceIDIndex < 0 {
		return nil, fmt.Errorf("cannot find snapshot ID column in '%s' in block '%s'", SnapshotIDColumnName, b.meta.BlockID.String())
	}

	return &rawIterator{b.meta.BlockID.String(), r, traceIDIndex, pool}, nil
}

type rawIterator struct {
	blockID      string
	r            *parquet.Reader //nolint:all //deprecated
	traceIDIndex int
	pool         *rowPool
}

var _ RawIterator = (*rawIterator)(nil)

func (i *rawIterator) getTraceID(r parquet.Row) common.ID {
	for _, v := range r {
		if v.Column() == i.traceIDIndex {
			return v.ByteArray()
		}
	}
	return nil
}

func (i *rawIterator) Next(context.Context) (common.ID, parquet.Row, error) {
	rows := []parquet.Row{i.pool.Get()}
	n, err := i.r.ReadRows(rows)
	if n > 0 {
		return i.getTraceID(rows[0]), rows[0], nil
	}

	if err == io.EOF {
		return nil, nil, nil
	}

	return nil, nil, errors.Wrap(err, fmt.Sprintf("error iterating through block %s", i.blockID))
}

func (i *rawIterator) peekNextID(_ context.Context) (common.ID, error) { // nolint:unused // this is required to satisfy the bookmarkIterator interface
	return nil, common.ErrUnsupported
}

func (i *rawIterator) Close() {
	_ = i.r.Close()
}
