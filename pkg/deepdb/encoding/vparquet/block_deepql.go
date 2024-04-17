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
	"strings"

	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepql"
	"github.com/intergral/deep/pkg/parquetquery"
	"github.com/pkg/errors"
	"github.com/segmentio/parquet-go"
)

// Lookup table of all well-known attributes with dedicated columns
var wellKnownColumnLookups = map[string]struct {
	columnPath string            // path.to.column
	typ        deepql.StaticType // Data type
}{
	// Resource-level columns
	Id:                    {columnPathSnapshotIDText, deepql.TypeString},
	LabelServiceName:      {columnPathResourceServiceName, deepql.TypeString},
	LabelCluster:          {columnPathResourceCluster, deepql.TypeString},
	LabelNamespace:        {columnPathResourceNamespace, deepql.TypeString},
	LabelPod:              {columnPathResourcePod, deepql.TypeString},
	LabelContainer:        {columnPathResourceContainer, deepql.TypeString},
	LabelK8sClusterName:   {columnPathResourceK8sClusterName, deepql.TypeString},
	LabelK8sNamespaceName: {columnPathResourceK8sNamespaceName, deepql.TypeString},
	LabelK8sPodName:       {columnPathResourceK8sPodName, deepql.TypeString},
	LabelK8sContainerName: {columnPathResourceK8sContainerName, deepql.TypeString},

	// also map when they are prefixed with 'resource.'
	"resource." + LabelServiceName:      {columnPathResourceServiceName, deepql.TypeString},
	"resource." + LabelCluster:          {columnPathResourceCluster, deepql.TypeString},
	"resource." + LabelNamespace:        {columnPathResourceNamespace, deepql.TypeString},
	"resource." + LabelPod:              {columnPathResourcePod, deepql.TypeString},
	"resource." + LabelContainer:        {columnPathResourceContainer, deepql.TypeString},
	"resource." + LabelK8sClusterName:   {columnPathResourceK8sClusterName, deepql.TypeString},
	"resource." + LabelK8sNamespaceName: {columnPathResourceK8sNamespaceName, deepql.TypeString},
	"resource." + LabelK8sPodName:       {columnPathResourceK8sPodName, deepql.TypeString},
	"resource." + LabelK8sContainerName: {columnPathResourceK8sContainerName, deepql.TypeString},
}

const (
	columnPathSnapshotID        = "ID"
	columnPathSnapshotIDText    = "IDText"
	columnPathStartTimeUnixNano = "TsNanos"
	columnPathDurationNanos     = "DurationNanos"

	columnPathTracepointID   = "tp.ID"
	columnPathTracepointFile = "tp.Path"
	columnPathTracepointLine = "tp.LineNumber"

	columnPathAttrKey         = "attr.Key"
	columnPathAttrValueString = "attr.Value"
	columnPathAttrValueInt    = "attr.ValueInt"
	columnPathAttrValueDouble = "attr.ValueDouble"
	columnPathAttrValueBool   = "attr.ValueBool"

	columnPathResourceAttrKey         = "rs.Attrs.Key"
	columnPathResourceAttrValueString = "rs.Attrs.Value"
	columnPathResourceAttrValueInt    = "rs.Attrs.ValueInt"
	columnPathResourceAttrValueDouble = "rs.Attrs.ValueDouble"
	columnPathResourceAttrValueBool   = "rs.Attrs.ValueBool"

	columnPathResourceServiceName      = "rs.ServiceName"
	columnPathResourceCluster          = "rs.Cluster"
	columnPathResourceNamespace        = "rs.Namespace"
	columnPathResourcePod              = "rs.Pod"
	columnPathResourceContainer        = "rs.Container"
	columnPathResourceK8sClusterName   = "rs.K8sClusterName"
	columnPathResourceK8sNamespaceName = "rs.K8sNamespaceName"
	columnPathResourceK8sPodName       = "rs.K8sPodName"
	columnPathResourceK8sContainerName = "rs.K8sContainerName"
)

// Helper function to create an iterator, that abstracts away
// context like file and rowgroups.
type makeIterFn func(columnName string, predicate parquetquery.Predicate, selectAs string) parquetquery.Iterator

func fetch(ctx context.Context, req deepql.FetchSnapshotRequest, pf *parquet.File, opts common.SearchOptions) (*snapshotMetadataIterator, error) {
	var (
		conditionIterators  []parquetquery.Iterator
		attributeConditions []deepql.Condition
		resourceConditions  []deepql.Condition
		columnPredicates    = map[string][]parquetquery.Predicate{}
		columnSelectAs      = map[string]string{}
	)

	rgs := rowGroupsFromFile(pf, opts)
	makeIter := makeIterFunc(ctx, rgs, pf)

	// add start time filter
	if req.StartTimeUnixNanos > 0 && req.EndTimeUnixNanos > 0 {
		conditionIterators = append(conditionIterators, makeIter(columnPathStartTimeUnixNano, parquetquery.NewIntBetweenPredicate(int64(req.StartTimeUnixNanos), int64(req.EndTimeUnixNanos)), columnPathStartTimeUnixNano))
	}

	if len(req.Conditions) == 0 {
		// we have no conditions so iterate the ids
		if len(conditionIterators) == 0 {
			return createSnapshotMetaIterator(makeIter, makeIter(columnPathSnapshotID, nil, ""))
		}
		// we are a time only request
		return createSnapshotMetaIterator(makeIter, conditionIterators[0])
	}

	addPredicate := func(columnPath string, p parquetquery.Predicate) {
		columnPredicates[columnPath] = append(columnPredicates[columnPath], p)
	}

	for _, condition := range req.Conditions {
		if condition.Attribute == "duration" {

			predicate, err := createIntPredicate(condition.Op, condition.Operands)
			if err != nil {
				return nil, err
			}
			conditionIterators = append(conditionIterators, makeIter(columnPathDurationNanos, predicate, columnPathDurationNanos))
		} else {
			if entry, ok := wellKnownColumnLookups[condition.Attribute]; ok {
				if condition.Op == deepql.OpNone {
					addPredicate(entry.columnPath, nil) // No filtering
					columnSelectAs[entry.columnPath] = condition.Attribute
					continue
				}

				// Compatible type?
				if entry.typ == operandType(condition.Operands) {
					pred, err := createPredicate(condition.Op, condition.Operands)
					if err != nil {
						return nil, errors.Wrap(err, "creating predicate")
					}
					addPredicate(entry.columnPath, pred)
					columnSelectAs[entry.columnPath] = condition.Attribute
					continue
				}
			} else {
				// if we are prefixed with 'resource.' then we should only search the resource attributes
				if strings.HasPrefix(condition.Attribute, "resource.") {
					// remove prefix and search
					condition.Attribute = strings.TrimPrefix(condition.Attribute, "resource.")
					resourceConditions = append(resourceConditions, condition)
				} else {
					// else add condition to both resource and attributes
					resourceConditions = append(resourceConditions, condition)
					attributeConditions = append(attributeConditions, condition)
				}
			}
		}
	}

	for columnPath, predicates := range columnPredicates {
		conditionIterators = append(conditionIterators, makeIter(columnPath, parquetquery.NewOrPredicate(predicates...), columnSelectAs[columnPath]))
	}
	if len(resourceConditions) > 0 || len(attributeConditions) > 0 {
		var iters []parquetquery.Iterator
		if len(resourceConditions) > 0 {
			resIter, err := createAttributeIterator(makeIter, resourceConditions, DefinitionLevelSnapshot, columnPathResourceAttrKey, columnPathResourceAttrValueString, columnPathResourceAttrValueInt, columnPathResourceAttrValueDouble, columnPathResourceAttrValueBool, true)
			if err != nil {
				return nil, errors.Wrap(err, "cannot make resource iterator")
			}
			iters = append(iters, resIter)
		}

		if len(attributeConditions) > 0 {
			attrIter, err := createAttributeIterator(makeIter, attributeConditions, DefinitionLevelSnapshot, columnPathAttrKey, columnPathAttrValueString, columnPathAttrValueInt, columnPathAttrValueDouble, columnPathAttrValueBool, true)
			if err != nil {
				return nil, errors.Wrap(err, "cannot make attribute iterator")
			}
			iters = append(iters, attrIter)
		}
		conditionIterators = append(conditionIterators, parquetquery.NewUnionIterator(DefinitionLevelSnapshot, iters, nil))
	}

	iterator := parquetquery.NewJoinIterator(DefinitionLevelSnapshot, conditionIterators, nil)

	return createSnapshotMetaIterator(makeIter, iterator)
}

func createSnapshotMetaIterator(makeIter makeIterFn, snapshotIter parquetquery.Iterator) (*snapshotMetadataIterator, error) {
	snapshotIterators := []parquetquery.Iterator{
		snapshotIter,
		// Add static columns that are always return
		makeIter(columnPathSnapshotID, nil, columnPathSnapshotID),
		makeIter(columnPathStartTimeUnixNano, nil, columnPathStartTimeUnixNano),
		makeIter(columnPathDurationNanos, nil, columnPathDurationNanos),
		makeIter(columnPathResourceServiceName, nil, columnPathResourceServiceName),
		makeIter(columnPathTracepointFile, nil, columnPathTracepointFile),
		makeIter(columnPathTracepointLine, nil, columnPathTracepointLine),
	}

	return newSnapshotMetadataIterator(parquetquery.NewJoinIterator(DefinitionLevelSnapshot, snapshotIterators, nil)), nil
}

func operandType(operands deepql.Operands) deepql.StaticType {
	if len(operands) > 0 {
		return operands[0].Type
	}
	return deepql.TypeNil
}

func createAttributeIterator(makeIter makeIterFn, conditions []deepql.Condition,
	definitionLevel int,
	keyPath, strPath, intPath, floatPath, boolPath string,
	allConditions bool,
) (parquetquery.Iterator, error) {
	var ittrs []parquetquery.Iterator
	for _, condition := range conditions {
		attrName := condition.Attribute
		keyIter := makeIter(keyPath, parquetquery.NewStringInPredicate([]string{attrName}), "key")
		var valueIter parquetquery.Iterator
		switch condition.Operands[0].Type {
		case deepql.TypeString:
			valuePred, err := createStringPredicate(condition.Op, condition.Operands)
			if err != nil {
				return nil, errors.Wrap(err, "creating attribute predicate")
			}
			valueIter = makeIter(strPath, valuePred, "string")
		case deepql.TypeInt:
			valuePred, err := createIntPredicate(condition.Op, condition.Operands)
			if err != nil {
				return nil, errors.Wrap(err, "creating attribute predicate")
			}
			valueIter = makeIter(intPath, valuePred, "int")
		case deepql.TypeFloat:
			valuePred, err := createFloatPredicate(condition.Op, condition.Operands)
			if err != nil {
				return nil, errors.Wrap(err, "creating attribute predicate")
			}
			valueIter = makeIter(floatPath, valuePred, "float")
		case deepql.TypeBoolean:
			valuePred, err := createBoolPredicate(condition.Op, condition.Operands)
			if err != nil {
				return nil, errors.Wrap(err, "creating attribute predicate")
			}
			valueIter = makeIter(boolPath, valuePred, "bool")
		default:
			return nil, errors.Errorf("unknown operand type: %v", condition.Operands[0].Type)
		}

		ittrs = append(ittrs, parquetquery.NewLeftJoinIterator(definitionLevel, []parquetquery.Iterator{keyIter, valueIter}, nil, nil))
	}

	if allConditions {
		return parquetquery.NewLeftJoinIterator(definitionLevel, ittrs, nil, nil), nil
	}
	return parquetquery.NewUnionIterator(definitionLevel, ittrs, nil), nil
}

type attributeCollector struct{}

var _ parquetquery.GroupPredicate = (*attributeCollector)(nil)

func (c *attributeCollector) String() string {
	return "attributeCollector{}"
}

func (c *attributeCollector) KeepGroup(res *parquetquery.IteratorResult) bool {
	var key string
	var val deepql.Static

	for _, e := range res.Entries {
		// Ignore nulls, this leaves val as the remaining found value,
		// or nil if the key was found but no matching values
		if e.Value.Kind() < 0 {
			continue
		}

		switch e.Key {
		case "key":
			key = e.Value.String()
		case "string":
			val = deepql.NewStaticString(e.Value.String())
		case "int":
			val = deepql.NewStaticInt(int(e.Value.Int64()))
		case "float":
			val = deepql.NewStaticFloat(e.Value.Double())
		case "bool":
			val = deepql.NewStaticBool(e.Value.Boolean())
		}
	}

	res.Entries = res.Entries[:0]
	res.OtherEntries = res.OtherEntries[:0]
	res.AppendOtherValue(key, val)

	return true
}
