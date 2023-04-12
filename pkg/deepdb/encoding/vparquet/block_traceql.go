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
)

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
