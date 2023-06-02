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
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepql"
	"github.com/intergral/deep/pkg/parquetquery"
	"github.com/segmentio/parquet-go"
)

const (
	columnPathSnapshotID  = "ID"
	columnPathServiceName = "ServiceName"

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

// Lookup table of all well-known attributes with dedicated columns
var wellKnownColumnLookups = map[string]struct {
	columnPath string                // path.to.column
	level      deepql.AttributeScope // span or resource level
	typ        deepql.StaticType     // Data type
}{
	// Resource-level columns
	LabelServiceName:      {columnPathResourceServiceName, deepql.AttributeScopeResource, deepql.TypeString},
	LabelCluster:          {columnPathResourceCluster, deepql.AttributeScopeResource, deepql.TypeString},
	LabelNamespace:        {columnPathResourceNamespace, deepql.AttributeScopeResource, deepql.TypeString},
	LabelPod:              {columnPathResourcePod, deepql.AttributeScopeResource, deepql.TypeString},
	LabelContainer:        {columnPathResourceContainer, deepql.AttributeScopeResource, deepql.TypeString},
	LabelK8sClusterName:   {columnPathResourceK8sClusterName, deepql.AttributeScopeResource, deepql.TypeString},
	LabelK8sNamespaceName: {columnPathResourceK8sNamespaceName, deepql.AttributeScopeResource, deepql.TypeString},
	LabelK8sPodName:       {columnPathResourceK8sPodName, deepql.AttributeScopeResource, deepql.TypeString},
	LabelK8sContainerName: {columnPathResourceK8sContainerName, deepql.AttributeScopeResource, deepql.TypeString},
}

// Helper function to create an iterator, that abstracts away
// context like file and rowgroups.
type makeIterFn func(columnName string, predicate parquetquery.Predicate, selectAs string) parquetquery.Iterator

// fetch is the core logic for executing the given conditions against the parquet columns. The algorithm
// can be summarized as a hiearchy of iterators where we iterate related columns together and collect the results
// at each level into attributes, spans, and spansets.  Each condition (.foo=bar) is pushed down to the one or more
// matching columns using parquetquery.Predicates.  Results are collected The final return is an iterator where each result is 1 Spanset for each trace.
func fetch(ctx context.Context, req deepql.FetchSnapshotRequest, pf *parquet.File, opts common.SearchOptions) (*snapshotMetadataIterator, error) {
	return nil, nil
}
