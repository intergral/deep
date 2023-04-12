package vparquet

import (
	"context"
	"fmt"
	"github.com/segmentio/parquet-go"
	"io"

	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepql"
	pq "github.com/intergral/deep/pkg/parquetquery"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

var translateTagToAttribute = map[string]deepql.Attribute{
	// Preserve behavior of v1 tag lookups which directed some attributes
	// to dedicated columns.
	LabelServiceName:      deepql.NewScopedAttribute(deepql.AttributeScopeResource, false, LabelServiceName),
	LabelCluster:          deepql.NewScopedAttribute(deepql.AttributeScopeResource, false, LabelCluster),
	LabelNamespace:        deepql.NewScopedAttribute(deepql.AttributeScopeResource, false, LabelNamespace),
	LabelPod:              deepql.NewScopedAttribute(deepql.AttributeScopeResource, false, LabelPod),
	LabelContainer:        deepql.NewScopedAttribute(deepql.AttributeScopeResource, false, LabelContainer),
	LabelK8sNamespaceName: deepql.NewScopedAttribute(deepql.AttributeScopeResource, false, LabelK8sNamespaceName),
	LabelK8sClusterName:   deepql.NewScopedAttribute(deepql.AttributeScopeResource, false, LabelK8sClusterName),
	LabelK8sPodName:       deepql.NewScopedAttribute(deepql.AttributeScopeResource, false, LabelK8sPodName),
	LabelK8sContainerName: deepql.NewScopedAttribute(deepql.AttributeScopeResource, false, LabelK8sContainerName),
}

var nondeepqlAttributes = map[string]string{
	LabelRootServiceName: columnPathServiceName,
}

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
	spanKeyIdx, _ := pq.GetColumnIndexByPath(pf, FieldAttrKey)
	if resourceKeyIdx == -1 || spanKeyIdx == -1 {
		return fmt.Errorf("resource or span attributes col not found (%d, %d)", resourceKeyIdx, spanKeyIdx)
	}
	standardAttrIdxs := []int{
		resourceKeyIdx,
		spanKeyIdx,
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
				defer pgs.Close()
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
				defer pgs.Close()

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

	att, ok := translateTagToAttribute[tag]
	if !ok {
		att = deepql.NewAttribute(tag)
	}

	// Wrap to v2-style
	cb2 := func(v deepql.Static) bool {
		cb(v.EncodeToString(false))
		return false
	}

	return b.SearchTagValuesV2(ctx, att, cb2, opts)
}

func (b *backendBlock) SearchTagValuesV2(ctx context.Context, tag deepql.Attribute, cb common.TagCallbackV2, opts common.SearchOptions) error {
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

func searchTagValues(ctx context.Context, tag deepql.Attribute, cb common.TagCallbackV2, pf *parquet.File) error {
	// Special handling for intrinsics
	if tag.Intrinsic != deepql.IntrinsicNone {
		//lookup := intrinsicColumnLookups[tag.Intrinsic]
		//if lookup.columnPath != "" {
		//	err := searchSpecialTagValues(ctx, lookup.columnPath, pf, cb)
		//	if err != nil {
		//		return fmt.Errorf("unexpected error searching special tags: %w", err)
		//	}
		//}
		return nil
	}

	// Special handling for weird non-deepql things
	if columnPath := nondeepqlAttributes[tag.Name]; columnPath != "" {
		err := searchSpecialTagValues(ctx, columnPath, pf, cb)
		if err != nil {
			return fmt.Errorf("unexpected error searching special tags: %s %w", columnPath, err)
		}
		return nil
	}

	// Search dedicated attribute column if one exists and is a compatible scope.
	//column := wellKnownColumnLookups[tag.Name]
	//if column.columnPath != "" && (tag.Scope == column.level || tag.Scope == deepql.AttributeScopeNone) {
	//	err := searchSpecialTagValues(ctx, column.columnPath, pf, cb)
	//	if err != nil {
	//		return fmt.Errorf("unexpected error searching special tags: %w", err)
	//	}
	//}

	// Finally also search generic key/values
	err := searchStandardTagValues(ctx, tag, pf, cb)
	if err != nil {
		return fmt.Errorf("unexpected error searching standard tags: %w", err)
	}

	return nil
}

// searchStandardTagValues searches a parquet file for "standard" tags. i.e. tags that don't have unique
// columns and are contained in labelMappings
func searchStandardTagValues(ctx context.Context, tag deepql.Attribute, pf *parquet.File, cb common.TagCallbackV2) error {
	rgs := pf.RowGroups()
	makeIter := makeIterFunc(ctx, rgs, pf)

	keyPred := pq.NewStringInPredicate([]string{tag.Name})

	if tag.Scope == deepql.AttributeScopeNone || tag.Scope == deepql.AttributeScopeResource {
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
	}

	if tag.Scope == deepql.AttributeScopeNone || tag.Scope == deepql.AttributeScopeSnapshot {
		err := searchKeyValues(DefinitionLevelResourceSpansILSSpanAttrs,
			FieldAttrKey,
			FieldAttrVal,
			FieldAttrValInt,
			FieldAttrValDouble,
			FieldAttrValBool,
			makeIter, keyPred, cb)
		if err != nil {
			return errors.Wrap(err, "search span key values")
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
			// These are optional and we find matching values of all types
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
