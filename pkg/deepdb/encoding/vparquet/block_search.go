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
	"io"
	"math"
	"time"

	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/deepql"
	"github.com/segmentio/parquet-go"

	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	deepIO "github.com/intergral/deep/pkg/io"
	pq "github.com/intergral/deep/pkg/parquetquery"
	"github.com/intergral/deep/pkg/util"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

// These are reserved search parameters
const (
	LabelDuration = "duration"
)

// openForSearch consolidates all the logic for opening a parquet file
func (b *backendBlock) openForSearch(ctx context.Context, opts common.SearchOptions) (*parquet.File, *BackendReaderAt, error) {
	b.openMtx.Lock()
	defer b.openMtx.Unlock()

	backendReaderAt := NewBackendReaderAt(ctx, b.r, DataFileName, b.meta.BlockID, b.meta.TenantID)

	// no searches currently require bloom filters or the page index. so just add them statically
	o := []parquet.FileOption{
		parquet.SkipBloomFilters(true),
		parquet.SkipPageIndex(true),
		parquet.FileReadMode(parquet.ReadModeAsync),
	}

	// backend reader
	readerAt := io.ReaderAt(backendReaderAt)

	// buffering
	if opts.ReadBufferSize > 0 {
		//   only use buffered reader at if the block is small, otherwise it's far more effective to use larger
		//   buffers in the parquet sdk
		if opts.ReadBufferCount*opts.ReadBufferSize > int(b.meta.Size) {
			readerAt = deepIO.NewBufferedReaderAt(readerAt, int64(b.meta.Size), opts.ReadBufferSize, opts.ReadBufferCount)
		} else {
			o = append(o, parquet.ReadBufferSize(opts.ReadBufferSize))
		}
	}

	// optimized reader
	readerAt = newParquetOptimizedReaderAt(readerAt, int64(b.meta.Size), b.meta.FooterSize)

	// cached reader
	if opts.CacheControl.ColumnIndex || opts.CacheControl.Footer || opts.CacheControl.OffsetIndex {
		readerAt = newCachedReaderAt(readerAt, backendReaderAt, opts.CacheControl)
	}

	span, _ := opentracing.StartSpanFromContext(ctx, "parquet.OpenFile")
	span.SetTag("tenantID", b.meta.TenantID)
	defer span.Finish()
	pf, err := parquet.OpenFile(readerAt, int64(b.meta.Size), o...)

	if err == nil {
		b.pf = pf
		b.readerAt = backendReaderAt
	}

	return pf, backendReaderAt, err
}

func (b *backendBlock) Search(ctx context.Context, req *deeppb.SearchRequest, opts common.SearchOptions) (_ *deeppb.SearchResponse, err error) {
	span, derivedCtx := opentracing.StartSpanFromContext(ctx, "parquet.backendBlock.Search",
		opentracing.Tags{
			"blockID":   b.meta.BlockID,
			"tenantID":  b.meta.TenantID,
			"blockSize": b.meta.Size,
		})
	defer span.Finish()

	pf, rr, err := b.openForSearch(derivedCtx, opts)
	if err != nil {
		return nil, fmt.Errorf("unexpected error opening parquet file: %w", err)
	}
	defer func() { span.SetTag("inspectedBytes", rr.TotalBytesRead.Load()) }()

	// Get list of row groups to inspect. Ideally we use predicate pushdown
	// here to keep only row groups that can potentially satisfy the request
	// conditions, but don't have it figured out yet.
	rgs := rowGroupsFromFile(pf, opts)
	results, err := searchParquetFile(derivedCtx, pf, req, rgs)
	if err != nil {
		return nil, err
	}
	results.Metrics.InspectedBlocks++
	results.Metrics.InspectedBytes += rr.TotalBytesRead.Load()
	results.Metrics.InspectedSnapshots += uint32(b.meta.TotalObjects)

	return results, nil
}

func makePipelineWithRowGroups(ctx context.Context, req *deeppb.SearchRequest, pf *parquet.File, rgs []parquet.RowGroup) pq.Iterator {
	makeIter := makeIterFunc(ctx, rgs, pf)

	// Wire up iterators
	var resourceIters []pq.Iterator
	var snapshotIters []pq.Iterator

	otherAttrConditions := map[string]string{}

	for k, v := range req.Tags {
		column := labelMappings[k]

		// if we don't have a column mapping then pass it forward to otherAttribute handling
		if column == "" {
			otherAttrConditions[k] = v
			continue
		}

		resourceIters = append(resourceIters, makeIter(column, pq.NewSubstringPredicate(v), ""))
	}

	// Generic attribute conditions?
	if len(otherAttrConditions) > 0 {
		// We are looking for one or more foo=bar attributes that aren't
		// projected to their own columns, they are in the generic Key/Value
		// columns at the resource or snapshot levels.  We want to search
		// both locations. But we also only want to read the columns once.

		keys := make([]string, 0, len(otherAttrConditions))
		vals := make([]string, 0, len(otherAttrConditions))
		for k, v := range otherAttrConditions {
			keys = append(keys, k)
			vals = append(vals, v)
		}

		keyPred := pq.NewStringInPredicate(keys)
		valPred := pq.NewStringInPredicate(vals)

		// This iterator combines the results from the resource
		// and snapshot searches, and checks if all conditions were satisfied
		// on each snapshot.  This is a single-pass over the attribute columns.
		j := pq.NewUnionIterator(DefinitionLevelSnapshot, []pq.Iterator{
			//	// This iterator finds all keys/values at the resource level
			pq.NewJoinIterator(DefinitionLevelResourceAttrs, []pq.Iterator{
				makeIter(FieldResourceAttrKey, keyPred, "keys"),
				makeIter(FieldResourceAttrVal, valPred, "values"),
			}, nil),
			//	// This iterator finds all keys/values at the snapshot level
			pq.NewJoinIterator(DefinitionLevelSnapshotAttrs, []pq.Iterator{
				makeIter(FieldAttrKey, keyPred, "keys"),
				makeIter(FieldAttrVal, valPred, "values"),
			}, nil),
		}, pq.NewKeyValueGroupPredicate(keys, vals))

		resourceIters = append(resourceIters, j)
	}

	// Multiple resource-level filters get joined and wrapped
	// up to snapshot-level. A single filter can be used as-is
	if len(resourceIters) == 1 {
		snapshotIters = append(snapshotIters, resourceIters[0])
	}
	if len(resourceIters) > 1 {
		snapshotIters = append(snapshotIters, pq.NewJoinIterator(DefinitionLevelSnapshot, resourceIters, nil))
	}

	// Duration filtering?
	if req.MinDurationMs > 0 || req.MaxDurationMs > 0 {
		min := int64(0)
		if req.MinDurationMs > 0 {
			min = (time.Millisecond * time.Duration(req.MinDurationMs)).Nanoseconds()
		}
		max := int64(math.MaxInt64)
		if req.MaxDurationMs > 0 {
			max = (time.Millisecond * time.Duration(req.MaxDurationMs)).Nanoseconds()
		}
		durFilter := pq.NewIntBetweenPredicate(min, max)
		snapshotIters = append(snapshotIters, makeIter("DurationNanos", durFilter, "DurationNanos"))
	}

	// Time range filtering?
	if req.Start > 0 && req.End > 0 {
		// Here's how we detect the snapshot overlaps the time window:

		// Snapshot start <= req.End
		startFilter := pq.NewIntBetweenPredicate(time.Unix(int64(req.Start), 0).UnixNano(), time.Unix(int64(req.End), 0).UnixNano())
		snapshotIters = append(snapshotIters, makeIter("TsNanos", startFilter, "TsNanos"))
	}

	switch len(snapshotIters) {

	case 0:
		// Empty request, in this case every snapshot matches, so we can
		// simply iterate any column.
		return makeIter("ID", nil, "")

	case 1:
		// There is only 1 iterator already, no need to wrap it up
		return snapshotIters[0]

	default:
		// Join all conditions
		return pq.NewJoinIterator(DefinitionLevelSnapshot, snapshotIters, nil)
	}
}

func searchParquetFile(ctx context.Context, pf *parquet.File, req *deeppb.SearchRequest, rgs []parquet.RowGroup) (*deeppb.SearchResponse, error) {
	// Search happens in 2 phases for an optimization.
	// Phase 1 is to iterate all columns involved in the request.
	// Only if there are any matches do we enter phase 2, which
	// is to load the display-related columns.

	// Find matches
	matchingRows, err := searchRaw(ctx, pf, req, rgs)
	if err != nil {
		return nil, err
	}
	if len(matchingRows) == 0 {
		return &deeppb.SearchResponse{Metrics: &deeppb.SearchMetrics{}}, nil
	}

	// We have some results, now load the display columns
	results, err := rawToResults(ctx, pf, rgs, matchingRows)
	if err != nil {
		return nil, err
	}

	return &deeppb.SearchResponse{
		Snapshots: results,
		Metrics:   &deeppb.SearchMetrics{},
	}, nil
}

func searchRaw(ctx context.Context, pf *parquet.File, req *deeppb.SearchRequest, rgs []parquet.RowGroup) ([]pq.RowNumber, error) {
	iter := makePipelineWithRowGroups(ctx, req, pf, rgs)
	if iter == nil {
		return nil, errors.New("make pipeline returned a nil iterator")
	}
	defer iter.Close()

	// Collect matches, row numbers only.
	var matchingRows []pq.RowNumber
	for {
		match, err := iter.Next()
		if err != nil {
			return nil, errors.Wrap(err, "searchRaw next failed")
		}
		if match == nil {
			break
		}
		matchingRows = append(matchingRows, match.RowNumber)
		if req.Limit > 0 && len(matchingRows) >= int(req.Limit) {
			break
		}
	}

	return matchingRows, nil
}

func rawToResults(ctx context.Context, pf *parquet.File, rgs []parquet.RowGroup, rowNumbers []pq.RowNumber) ([]*deeppb.SnapshotSearchMetadata, error) {
	makeIter := makeIterFunc(ctx, rgs, pf)

	results := []*deeppb.SnapshotSearchMetadata{}
	iter2 := pq.NewJoinIterator(DefinitionLevelSnapshot, []pq.Iterator{
		&rowNumberIterator{rowNumbers: rowNumbers},
		makeIter("ID", nil, "ID"),
		makeIter("rs.ServiceName", nil, "ServiceName"),
		makeIter("TsNanos", nil, "TsNanos"),
		makeIter("DurationNanos", nil, "DurationNanos"),
		makeIter("tp.Path", nil, "FilePath"),
		makeIter("tp.LineNumber", nil, "LineNo"),
	}, nil)
	defer iter2.Close()

	for {
		match, err := iter2.Next()
		if err != nil {
			return nil, errors.Wrap(err, "rawToResults next failed")
		}
		if match == nil {
			break
		}

		matchMap := match.ToMap()
		result := &deeppb.SnapshotSearchMetadata{
			SnapshotID:        util.SnapshotIDToHexString(matchMap["ID"][0].Bytes()),
			ServiceName:       matchMap["ServiceName"][0].String(),
			FilePath:          matchMap["FilePath"][0].String(),
			LineNo:            matchMap["LineNo"][0].Uint32(),
			StartTimeUnixNano: matchMap["TsNanos"][0].Uint64(),
			DurationNano:      matchMap["DurationNanos"][0].Uint64(),
		}
		results = append(results, result)
	}

	return results, nil
}

func makeIterFunc(ctx context.Context, rgs []parquet.RowGroup, pf *parquet.File) func(name string, predicate pq.Predicate, selectAs string) pq.Iterator {
	return func(name string, predicate pq.Predicate, selectAs string) pq.Iterator {
		index, _ := pq.GetColumnIndexByPath(pf, name)
		if index == -1 {
			// TODO - don't panic, error instead
			panic("column not found in parquet file:" + name)
		}
		return pq.NewColumnIterator(ctx, rgs, index, name, 1000, predicate, selectAs)
	}
}

type rowNumberIterator struct {
	rowNumbers []pq.RowNumber
}

var _ pq.Iterator = (*rowNumberIterator)(nil)

func (r *rowNumberIterator) String() string {
	return "rowNumberIterator()"
}

func (r *rowNumberIterator) Next() (*pq.IteratorResult, error) {
	if len(r.rowNumbers) == 0 {
		return nil, nil
	}

	res := &pq.IteratorResult{RowNumber: r.rowNumbers[0]}
	r.rowNumbers = r.rowNumbers[1:]
	return res, nil
}

func (r *rowNumberIterator) SeekTo(to pq.RowNumber, definitionLevel int) (*pq.IteratorResult, error) {
	var at *pq.IteratorResult

	for at, _ = r.Next(); r != nil && at != nil && pq.CompareRowNumbers(definitionLevel, at.RowNumber, to) < 0; {
		at, _ = r.Next()
	}

	return at, nil
}

func (r *rowNumberIterator) Close() {}

// reportValuesPredicate is a "fake" predicate that uses existing iterator logic to find all values in a given column
type reportValuesPredicate struct {
	cb            common.TagCallbackV2
	inspectedDict bool
}

func newReportValuesPredicate(cb common.TagCallbackV2) *reportValuesPredicate {
	return &reportValuesPredicate{cb: cb}
}

func (r *reportValuesPredicate) String() string {
	return "reportValuesPredicate{}"
}

// KeepColumnChunk always returns true b/c we always have to dig deeper to find all values
func (r *reportValuesPredicate) KeepColumnChunk(parquet.ColumnChunk) bool {
	// Reinspect dictionary for each new column chunk
	r.inspectedDict = false
	return true
}

// KeepPage checks to see if the page has a dictionary. if it does then we can report the values contained in it
// and return false b/c we don't have to go to the actual columns to retrieve values. if there is no dict we return
// true so the iterator will call KeepValue on all values in the column
func (r *reportValuesPredicate) KeepPage(pg parquet.Page) bool {
	if r.inspectedDict {
		// Already inspected dictionary for this column chunk
		return false
	}

	if dict := pg.Dictionary(); dict != nil {
		for i := 0; i < dict.Len(); i++ {
			v := dict.Index(int32(i))
			if callback(r.cb, v) {
				break
			}
		}

		// Only inspect first dictionary per column chunk.
		r.inspectedDict = true
		return false
	}

	return true
}

// KeepValue is only called if this column does not have a dictionary. Just report everything to r.cb and
// return false so the iterator do any extra work.
func (r *reportValuesPredicate) KeepValue(v parquet.Value) bool {
	callback(r.cb, v)

	return false
}

func callback(cb common.TagCallbackV2, v parquet.Value) (stop bool) {
	switch v.Kind() {

	case parquet.Boolean:
		return cb(deepql.NewStaticBool(v.Boolean()))

	case parquet.Int32, parquet.Int64:
		return cb(deepql.NewStaticInt(int(v.Int64())))

	case parquet.Float, parquet.Double:
		return cb(deepql.NewStaticFloat(v.Double()))

	case parquet.ByteArray, parquet.FixedLenByteArray:
		return cb(deepql.NewStaticString(v.String()))

	default:
		// Skip nils or unsupported type
		return false
	}
}

func rowGroupsFromFile(pf *parquet.File, opts common.SearchOptions) []parquet.RowGroup {
	rgs := pf.RowGroups()
	if opts.TotalPages > 0 {
		// Read UP TO TotalPages.  The sharding calculations
		// are just estimates, so it may not line up with the
		// actual number of pages in this file.
		if opts.StartPage+opts.TotalPages > len(rgs) {
			opts.TotalPages = len(rgs) - opts.StartPage
		}
		rgs = rgs[opts.StartPage : opts.StartPage+opts.TotalPages]
	}

	return rgs
}
