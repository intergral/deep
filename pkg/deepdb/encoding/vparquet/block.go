package vparquet

import (
	"context"
	"fmt"
	"github.com/intergral/deep/pkg/deepql"
	"github.com/pkg/errors"
	"github.com/segmentio/parquet-go"
	"reflect"
	"sync"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
)

const (
	DataFileName = "data.parquet"
)

type backendBlock struct {
	meta *backend.BlockMeta
	r    backend.Reader

	openMtx  sync.Mutex
	pf       *parquet.File
	readerAt *BackendReaderAt
}

var _ common.BackendBlock = (*backendBlock)(nil)

func newBackendBlock(meta *backend.BlockMeta, r backend.Reader) *backendBlock {
	return &backendBlock{
		meta: meta,
		r:    r,
	}
}

func (b *backendBlock) BlockMeta() *backend.BlockMeta {
	return b.meta
}

// Fetch snapshots from the block for the given deepql FetchSpansRequest. The request is checked for
// internal consistencies:  operand count matches the operation, all operands in each condition are identical
// types, and the operand type is compatible with the operation.
func (b *backendBlock) Fetch(ctx context.Context, req deepql.FetchSnapshotRequest, opts common.SearchOptions) (deepql.FetchSnapshotResponse, error) {

	err := checkConditions(req.Conditions)
	if err != nil {
		return deepql.FetchSnapshotResponse{}, errors.Wrap(err, "conditions invalid")
	}

	pf, rr, err := b.openForSearch(ctx, opts)
	if err != nil {
		return deepql.FetchSnapshotResponse{}, err
	}

	iter, err := fetch(ctx, req, pf, opts)
	if err != nil {
		return deepql.FetchSnapshotResponse{}, errors.Wrap(err, "creating fetch iter")
	}

	return deepql.FetchSnapshotResponse{
		Results: iter,
		Bytes:   func() uint64 { return rr.TotalBytesRead.Load() },
	}, nil
}

func checkConditions(conditions []deepql.Condition) error {
	for _, cond := range conditions {
		opCount := len(cond.Operands)

		switch cond.Op {

		case deepql.OpNone:
			if opCount != 0 {
				return fmt.Errorf("operanion none must have 0 arguments. condition: %+v", cond)
			}

		case deepql.OpEqual, deepql.OpNotEqual,
			deepql.OpGreater, deepql.OpGreaterEqual,
			deepql.OpLess, deepql.OpLessEqual,
			deepql.OpRegex:
			if opCount != 1 {
				return fmt.Errorf("operation %v must have exactly 1 argument. condition: %+v", cond.Op, cond)
			}

		default:
			return fmt.Errorf("unknown operation. condition: %+v", cond)
		}

		// Verify all operands are of the same type
		if opCount == 0 {
			continue
		}

		for i := 1; i < opCount; i++ {
			if reflect.TypeOf(cond.Operands[0]) != reflect.TypeOf(cond.Operands[i]) {
				return fmt.Errorf("operands must be of the same type. condition: %+v", cond)
			}
		}
	}

	return nil
}
