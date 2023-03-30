package vparquet

import (
	"context"
	"github.com/segmentio/parquet-go"

	"github.com/intergral/deep/pkg/deepdb/encoding/common"
)

type TraceIterator interface {
	NextTrace(context.Context) (common.ID, *Trace, error)
	Close()
}

type RawIterator interface {
	Next(context.Context) (common.ID, parquet.Row, error)
	Close()
}
