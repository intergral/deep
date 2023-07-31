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
	"bytes"
	"context"
	"fmt"
	deepTP "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/segmentio/parquet-go"
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/willf/bloom"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/parquetquery"
	pq "github.com/intergral/deep/pkg/parquetquery"
	"github.com/intergral/deep/pkg/util"
)

const (
	SearchPrevious = -1
	SearchNext     = -2
	NotFound       = -3

	SnapshotIDColumnName = "ID"
)

func (b *backendBlock) checkBloom(ctx context.Context, id common.ID) (found bool, err error) {
	span, derivedCtx := opentracing.StartSpanFromContext(ctx, "parquet.backendBlock.checkBloom",
		opentracing.Tags{
			"blockID":  b.meta.BlockID,
			"tenantID": b.meta.TenantID,
		})
	defer span.Finish()

	shardKey := common.ShardKeyForSnapshotID(id, int(b.meta.BloomShardCount))
	nameBloom := common.BloomName(shardKey)
	span.SetTag("bloom", nameBloom)

	bloomBytes, err := b.r.Read(derivedCtx, nameBloom, b.meta.BlockID, b.meta.TenantID, true)
	if err != nil {
		return false, fmt.Errorf("error retrieving bloom %s (%s, %s): %w", nameBloom, b.meta.TenantID, b.meta.BlockID, err)
	}

	filter := &bloom.BloomFilter{}
	_, err = filter.ReadFrom(bytes.NewReader(bloomBytes))
	if err != nil {
		return false, fmt.Errorf("error parsing bloom (%s, %s): %w", b.meta.TenantID, b.meta.BlockID, err)
	}

	return filter.Test(id), nil
}

// FindSnapshotByID scan the block for the snapshot with the given ID
func (b *backendBlock) FindSnapshotByID(ctx context.Context, snapshotID common.ID, opts common.SearchOptions) (_ *deepTP.Snapshot, err error) {
	span, derivedCtx := opentracing.StartSpanFromContext(ctx, "parquet.backendBlock.FindSnapshotByID",
		opentracing.Tags{
			"blockID":   b.meta.BlockID,
			"tenantID":  b.meta.TenantID,
			"blockSize": b.meta.Size,
		})
	defer span.Finish()

	// if the id is not in the bloom, then it is not in the block.
	// bloom can give false positives, so we still need to handle the case it is not in the block later
	found, err := b.checkBloom(derivedCtx, snapshotID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	pf, rr, err := b.openForSearch(derivedCtx, opts)
	if err != nil {
		return nil, fmt.Errorf("unexpected error opening parquet file: %w", err)
	}
	defer func() {
		span.SetTag("inspectedBytes", rr.TotalBytesRead.Load())
	}()

	return findSnapshotByID(derivedCtx, snapshotID, b.meta, pf)
}

// findSnapshotByID will scan a given block for the snapshot
func findSnapshotByID(ctx context.Context, snapshotID common.ID, meta *backend.BlockMeta, pf *parquet.File) (*deepTP.Snapshot, error) {
	// snapshotID column index
	colIndex, _ := pq.GetColumnIndexByPath(pf, SnapshotIDColumnName)
	if colIndex == -1 {
		return nil, fmt.Errorf("unable to get index for column: %s", SnapshotIDColumnName)
	}

	numRowGroups := len(pf.RowGroups())
	buf := make(parquet.Row, 1)

	// Cache of row group bounds
	rowGroupMins := make([]common.ID, numRowGroups+1)
	// todo: restore using meta min/max id once it works
	//    https://github.com/intergral/deep/issues/1903
	rowGroupMins[0] = bytes.Repeat([]byte{0}, 16)
	rowGroupMins[numRowGroups] = bytes.Repeat([]byte{255}, 16) // This is actually inclusive and the logic is special for the last row group below

	// Gets the minimum snapshot ID within the row group. Since the column is sorted
	// ascending we just read the first value from the first page.
	getRowGroupMin := func(rgIdx int) (common.ID, error) {
		min := rowGroupMins[rgIdx]
		if len(min) > 0 {
			// Already loaded
			return min, nil
		}

		pages := pf.RowGroups()[rgIdx].ColumnChunks()[colIndex].Pages()
		defer func(pages parquet.Pages) {
			_ = pages.Close()
		}(pages)

		page, err := pages.ReadPage()
		if err != nil {
			return nil, err
		}

		c, err := page.Values().ReadValues(buf)
		if err != nil && err != io.EOF {
			return nil, err
		}
		if c < 1 {
			return nil, fmt.Errorf("failed to read value from page: snapshotID: %s blockID:%v rowGroupIdx:%d", util.SnapshotIDToHexString(snapshotID), meta.BlockID, rgIdx)
		}

		min = buf[0].ByteArray()
		rowGroupMins[rgIdx] = min
		return min, nil
	}

	rowGroup, err := binarySearch(numRowGroups, func(rgIdx int) (int, error) {
		min, err := getRowGroupMin(rgIdx)
		if err != nil {
			return 0, err
		}

		if check := bytes.Compare(snapshotID, min); check <= 0 {
			// Snapshot is before or in this group
			return check, nil
		}

		max, err := getRowGroupMin(rgIdx + 1)
		if err != nil {
			return 0, err
		}

		// This is actually the min of the next group, so check is exclusive not inclusive like min
		// Except for the last group, it is inclusive
		check := bytes.Compare(snapshotID, max)
		if check > 0 || (check == 0 && rgIdx < (numRowGroups-1)) {
			// Snapshot is after this group
			return 1, nil
		}

		// Must be in this group
		return 0, nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "error binary searching row groups")
	}

	if rowGroup == -1 {
		// Not within the bounds of any row group
		return nil, nil
	}

	// Now iterate the matching row group
	iter := parquetquery.NewColumnIterator(ctx, pf.RowGroups()[rowGroup:rowGroup+1], colIndex, "", 1000, parquetquery.NewStringInPredicate([]string{string(snapshotID)}), "")
	defer iter.Close()

	res, err := iter.Next()
	if err != nil {
		return nil, err
	}
	if res == nil {
		// SnapshotID not found in this block
		return nil, nil
	}

	// The row number coming out of the iterator is relative,
	// so offset it using the num rows in all previous groups
	rowMatch := int64(0)
	for _, rg := range pf.RowGroups()[0:rowGroup] {
		rowMatch += rg.NumRows()
	}
	rowMatch += res.RowNumber[0]

	// seek to row and read
	r := parquet.NewReader(pf)
	err = r.SeekToRow(rowMatch)
	if err != nil {
		return nil, errors.Wrap(err, "seek to row")
	}

	tr := new(Snapshot)
	err = r.Read(tr)
	if err != nil {
		return nil, errors.Wrap(err, "error reading row from backend")
	}

	// convert to proto snapshot and return
	return parquetToDeepSnapshot(tr), nil
}

// binarySearch that finds exact matching entry. Returns non-zero index when found, or -1 when not found
// Inspired by sort.Search but makes uses of tri-state comparator to eliminate the last comparison when
// we want to find exact match, not insertion point.
func binarySearch(n int, compare func(int) (int, error)) (int, error) {
	i, j := 0, n
	for i < j {
		h := int(uint(i+j) >> 1) // avoid overflow when computing h
		c, err := compare(h)
		if err != nil {
			return -1, err
		}
		// i ≤ h < j
		switch c {
		case 0:
			// Found exact match
			return h, nil
		case -1:
			j = h
		case 1:
			i = h + 1
		}
	}

	// No match
	return -1, nil
}

/*func dumpParquetRow(sch parquet.Schema, row parquet.Row) {
	for i, r := range row {
		slicestr := ""
		if r.Kind() == parquet.ByteArray {
			slicestr = util.SnapshotIDToHexString(r.ByteArray())
		}
		fmt.Printf("row[%d] = c:%d (%s) r:%d d:%d v:%s (%s)\n",
			i,
			r.Column(),
			strings.Join(sch.Columns()[r.Column()], "."),
			r.RepetitionLevel(),
			r.DefinitionLevel(),
			r.String(),
			slicestr,
		)
	}
}*/
