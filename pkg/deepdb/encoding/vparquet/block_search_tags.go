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
	"strings"

	"github.com/segmentio/parquet-go"

	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepql"
	pq "github.com/intergral/deep/pkg/parquetquery"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

func (b *backendBlock) SearchTags(ctx context.Context, cb common.TagCallback, opts common.SearchOptions) error {
	span, derivedCtx := opentracing.StartSpanFromContext(ctx, "parquet.backendBlock.SearchTags",
		opentracing.Tags{
			"blockID":   b.meta.BlockID,
			"tenantID":  b.meta.TenantID,
			"blockSize": b.meta.Size,
		})
	defer span.Finish()

	pf, rr, err := b.openForSearch(derivedCtx, opts)
	if err != nil {
		return fmt.Errorf("unexpected error opening parquet file: %w", err)
	}
	defer func() { span.SetTag("inspectedBytes", rr.TotalBytesRead.Load()) }()

	return searchTags(derivedCtx, cb, pf)
}

func searchTags(_ context.Context, cb common.TagCallback, pf *parquet.File) error {
	// find indexes of generic attribute columns
	resourceKeyIdx, _ := pq.GetColumnIndexByPath(pf, FieldResourceAttrKey)
	attributeKeyIdx, _ := pq.GetColumnIndexByPath(pf, FieldAttrKey)
	if resourceKeyIdx == -1 || attributeKeyIdx == -1 {
		return fmt.Errorf("resource or attributes col not found (%d, %d)", resourceKeyIdx, attributeKeyIdx)
	}
	standardAttrIdxs := []int{
		resourceKeyIdx,
		attributeKeyIdx,
	}

	// find indexes of all special columns
	specialAttrIdxs := map[int]string{}
	for lbl, col := range labelMappings {

		idx, _ := pq.GetColumnIndexByPath(pf, col)
		if idx == -1 {
			continue
		}

		specialAttrIdxs[idx] = lbl
	}

	// now search all row groups
	var err error
	rgs := pf.RowGroups()
	for _, rg := range rgs {
		// search all special attributes
		for idx, lbl := range specialAttrIdxs {
			cc := rg.ColumnChunks()[idx]
			err = func() error {
				pgs := cc.Pages()
				defer func(pgs parquet.Pages) {
					_ = pgs.Close()
				}(pgs)
				for {
					pg, err := pgs.ReadPage()
					if err == io.EOF || pg == nil {
						break
					}
					if err != nil {
						return err
					}

					stop := func(page parquet.Page) bool {
						defer parquet.Release(page)

						// if a special attribute has any non-null values, include it
						if page.NumNulls() < page.NumValues() {
							cb(lbl)
							delete(specialAttrIdxs, idx) // remove from map so we won't search again
							return true
						}
						return false
					}(pg)
					if stop {
						break
					}
				}
				return nil
			}()
			if err != nil {
				return err
			}
		}

		// search other attributes
		for _, idx := range standardAttrIdxs {
			cc := rg.ColumnChunks()[idx]
			err = func() error {
				pgs := cc.Pages()
				defer func(pgs parquet.Pages) {
					_ = pgs.Close()
				}(pgs)

				// normally we'd loop here calling read page for every page in the column chunk, but
				// there is only one dictionary per column chunk, so just read it from the first page
				// and be done.
				pg, err := pgs.ReadPage()
				if err == io.EOF || pg == nil {
					return nil
				}
				if err != nil {
					return err
				}

				func(page parquet.Page) {
					defer parquet.Release(page)

					dict := page.Dictionary()
					if dict == nil {
						return
					}

					for i := 0; i < dict.Len(); i++ {
						s := dict.Index(int32(i)).String()
						cb(s)
					}
				}(pg)

				return nil
			}()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (b *backendBlock) SearchTagValues(ctx context.Context, tag string, cb common.TagCallback, opts common.SearchOptions) error {
	// Wrap to v2-style
	cb2 := func(v deepql.Static) bool {
		cb(v.String())
		return false
	}

	return b.SearchTagValuesV2(ctx, tag, cb2, opts)
}

func (b *backendBlock) SearchTagValuesV2(ctx context.Context, tag string, cb common.TagCallbackV2, opts common.SearchOptions) error {
	span, derivedCtx := opentracing.StartSpanFromContext(ctx, "parquet.backendBlock.SearchTagValuesV2",
		opentracing.Tags{
			"blockID":   b.meta.BlockID,
			"tenantID":  b.meta.TenantID,
			"blockSize": b.meta.Size,
		})
	defer span.Finish()

	pf, rr, err := b.openForSearch(derivedCtx, opts)
	if err != nil {
		return fmt.Errorf("unexpected error opening parquet file: %w", err)
	}
	defer func() { span.SetTag("inspectedBytes", rr.TotalBytesRead.Load()) }()

	return searchTagValues(derivedCtx, tag, cb, pf)
}

func searchTagValues(ctx context.Context, tag string, cb common.TagCallbackV2, pf *parquet.File) error {
	if column, ok := wellKnownColumnLookups[tag]; ok {
		err := searchSpecialTagValues(ctx, column.columnPath, pf, cb)
		if err != nil {
			return fmt.Errorf("unexpected error searching special tags: %w", err)
		}
	}

	// Finally also search generic key/values
	err := searchStandardTagValues(ctx, tag, pf, cb)
	if err != nil {
		return fmt.Errorf("unexpected error searching standard tags: %w", err)
	}

	return nil
}

// searchStandardTagValues searches a parquet file for "standard" tags. i.e. tags that don't have unique
// columns and are contained in labelMappings
func searchStandardTagValues(ctx context.Context, tag string, pf *parquet.File, cb common.TagCallbackV2) error {
	rgs := pf.RowGroups()
	makeIter := makeIterFunc(ctx, rgs, pf)

	keyPred := pq.NewStringInPredicate([]string{tag})

	err := searchKeyValues(DefinitionLevelResourceAttrs,
		FieldResourceAttrKey,
		FieldResourceAttrVal,
		FieldResourceAttrValInt,
		FieldResourceAttrValDouble,
		FieldResourceAttrValBool,
		makeIter, keyPred, cb)
	if err != nil {
		return errors.Wrap(err, "search resource key values")
	}

	if !strings.HasPrefix(tag, "resource.") {
		err := searchKeyValues(DefinitionLevelSnapshotAttrs,
			FieldAttrKey,
			FieldAttrVal,
			FieldAttrValInt,
			FieldAttrValDouble,
			FieldAttrValBool,
			makeIter, keyPred, cb)

		if err != nil {
			return errors.Wrap(err, "search snapshot key values")
		}
	}

	return nil
}

func searchKeyValues(definitionLevel int, keyPath, stringPath, intPath, floatPath, boolPath string, makeIter makeIterFn, keyPred pq.Predicate, cb common.TagCallbackV2) error {
	skipNils := pq.NewSkipNilsPredicate()

	iter := pq.NewLeftJoinIterator(definitionLevel,
		// This is required
		[]pq.Iterator{makeIter(keyPath, keyPred, "")},
		[]pq.Iterator{
			// These are optional, and we find matching values of all types
			makeIter(stringPath, skipNils, "string"),
			makeIter(intPath, skipNils, "int"),
			makeIter(floatPath, skipNils, "float"),
			makeIter(boolPath, skipNils, "bool"),
		}, nil)
	defer iter.Close()

	for {
		match, err := iter.Next()
		if err != nil {
			return err
		}
		if match == nil {
			break
		}
		for _, e := range match.Entries {
			if callback(cb, e.Value) {
				// Stop
				return nil
			}
		}
	}

	return nil
}

// todo incorporate this into tag search

// searchSpecialTagValues searches a parquet file for all values for the provided column. It first attempts
// to only pull all values from the column's dictionary. If this fails it falls back to scanning the entire path.
func searchSpecialTagValues(ctx context.Context, column string, pf *parquet.File, cb common.TagCallbackV2) error {
	pred := newReportValuesPredicate(cb)
	rgs := pf.RowGroups()

	iter := makeIterFunc(ctx, rgs, pf)(column, pred, "")
	defer iter.Close()
	for {
		match, err := iter.Next()
		if err != nil {
			return errors.Wrap(err, "iter.Next failed")
		}
		if match == nil {
			break
		}
	}

	return nil
}
