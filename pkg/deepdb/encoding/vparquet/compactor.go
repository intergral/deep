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
	"runtime"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	deepIO "github.com/intergral/deep/pkg/io"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

func NewCompactor(opts common.CompactionOptions) *Compactor {
	return &Compactor{opts: opts}
}

type Compactor struct {
	opts common.CompactionOptions
}

func (c *Compactor) Compact(ctx context.Context, l log.Logger, r backend.Reader, writerCallback func(*backend.BlockMeta, time.Time) backend.Writer, inputs []*backend.BlockMeta) (newCompactedBlocks []*backend.BlockMeta, err error) {

	var (
		compactionLevel uint8
		totalRecords    int
		minBlockStart   time.Time
		maxBlockEnd     time.Time
		bookmarks       = make([]*bookmark[parquet.Row], 0, len(inputs))
		// MaxBytesPerSnapshot is the largest snapshot that can be expected, and assumes 1 byte per value on average (same as flushing).
		// Divide by 4 to presumably require 2 slice allocations if we ever see a snapshot this large
		pool = newRowPool(c.opts.MaxBytesPerSnapshot / 4)
	)
	for _, blockMeta := range inputs {
		totalRecords += blockMeta.TotalObjects

		if blockMeta.CompactionLevel > compactionLevel {
			compactionLevel = blockMeta.CompactionLevel
		}

		if blockMeta.StartTime.Before(minBlockStart) || minBlockStart.IsZero() {
			minBlockStart = blockMeta.StartTime
		}
		if blockMeta.EndTime.After(maxBlockEnd) {
			maxBlockEnd = blockMeta.EndTime
		}

		block := newBackendBlock(blockMeta, r)

		span, derivedCtx := opentracing.StartSpanFromContext(ctx, "vparquet.compactor.iterator")
		defer span.Finish()

		iter, err := block.RawIterator(derivedCtx, pool)
		if err != nil {
			return nil, err
		}

		bookmarks = append(bookmarks, newBookmark[parquet.Row](iter))
	}

	var (
		nextCompactionLevel = compactionLevel + 1
	)

	var (
		m               = newMultiblockIterator(bookmarks)
		recordsPerBlock = totalRecords / int(c.opts.OutputBlocks)
		currentBlock    *streamingBlock
	)
	defer m.Close()

	for {
		lowestID, lowestObject, err := m.Next(ctx)
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, errors.Wrap(err, "error iterating input blocks")
		}

		// make a new block if necessary
		if currentBlock == nil {
			// Start with a copy and then customize
			newMeta := &backend.BlockMeta{
				BlockID:         uuid.New(),
				TenantID:        inputs[0].TenantID,
				CompactionLevel: nextCompactionLevel,
				TotalObjects:    recordsPerBlock, // Just an estimate
			}
			w := writerCallback(newMeta, time.Now())

			currentBlock = newStreamingBlock(ctx, &c.opts.BlockConfig, newMeta, r, w, deepIO.NewBufferedWriter)
			currentBlock.meta.CompactionLevel = nextCompactionLevel
			newCompactedBlocks = append(newCompactedBlocks, currentBlock.meta)
		}

		// Flush existing block data if the next snapshot can't fit
		if currentBlock.EstimatedBufferedBytes() > 0 && currentBlock.EstimatedBufferedBytes()+estimateMarshalledSizeFromParquetRow(lowestObject) > c.opts.BlockConfig.RowGroupSizeBytes {
			runtime.GC()
			err = c.appendBlock(ctx, currentBlock, l)
			if err != nil {
				return nil, errors.Wrap(err, "error writing partial block")
			}
		}

		// Write snapshot.
		// Note - not specifying snapshot start/end here, we set the overall block start/stop
		// times from the input metas.
		err = currentBlock.AddRaw(lowestID, lowestObject, 0)
		if err != nil {
			return nil, err
		}

		// Flush again if block is already full.
		if currentBlock.EstimatedBufferedBytes() > c.opts.BlockConfig.RowGroupSizeBytes {
			runtime.GC()
			err = c.appendBlock(ctx, currentBlock, l)
			if err != nil {
				return nil, errors.Wrap(err, "error writing partial block")
			}
		}

		pool.Put(lowestObject)

		// ship block to backend if done
		if currentBlock.meta.TotalObjects >= recordsPerBlock {
			currentBlockPtrCopy := currentBlock
			currentBlockPtrCopy.meta.StartTime = minBlockStart
			currentBlockPtrCopy.meta.EndTime = maxBlockEnd
			err := c.finishBlock(ctx, currentBlockPtrCopy, l)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("error shipping block to backend, blockID %s", currentBlockPtrCopy.meta.BlockID.String()))
			}
			currentBlock = nil
		}
	}

	// ship final block to backend
	if currentBlock != nil {
		currentBlock.meta.StartTime = minBlockStart
		currentBlock.meta.EndTime = maxBlockEnd
		err := c.finishBlock(ctx, currentBlock, l)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("error shipping block to backend, blockID %s", currentBlock.meta.BlockID.String()))
		}
	}

	return newCompactedBlocks, nil
}

func (c *Compactor) appendBlock(ctx context.Context, block *streamingBlock, l log.Logger) error {
	span, _ := opentracing.StartSpanFromContext(ctx, "vparquet.compactor.appendBlock")
	defer span.Finish()

	var (
		objs            = block.CurrentBufferedObjects()
		vals            = block.EstimatedBufferedBytes()
		compactionLevel = int(block.meta.CompactionLevel - 1)
	)

	if c.opts.ObjectsWritten != nil {
		c.opts.ObjectsWritten(compactionLevel, objs)
	}

	bytesFlushed, err := block.Flush()
	if err != nil {
		return err
	}

	if c.opts.BytesWritten != nil {
		c.opts.BytesWritten(compactionLevel, bytesFlushed)
	}

	level.Info(l).Log("msg", "flushed to block", "bytes", bytesFlushed, "objects", objs, "values", vals)

	return nil
}

func (c *Compactor) finishBlock(ctx context.Context, block *streamingBlock, l log.Logger) error {
	span, _ := opentracing.StartSpanFromContext(ctx, "vparquet.compactor.finishBlock")
	defer span.Finish()

	bytesFlushed, err := block.Complete()
	if err != nil {
		return errors.Wrap(err, "error completing block")
	}

	level.Info(l).Log("msg", "wrote compacted block", "meta", fmt.Sprintf("%+v", block.meta))
	compactionLevel := int(block.meta.CompactionLevel) - 1
	if c.opts.BytesWritten != nil {
		c.opts.BytesWritten(compactionLevel, bytesFlushed)
	}
	return nil
}

type rowPool struct {
	pool sync.Pool
}

func newRowPool(defaultRowSize int) *rowPool {
	return &rowPool{
		pool: sync.Pool{
			New: func() any {
				return make(parquet.Row, 0, defaultRowSize)
			},
		},
	}
}

func (r *rowPool) Get() parquet.Row {
	return r.pool.Get().(parquet.Row)
}

func (r *rowPool) Put(row parquet.Row) {
	// Clear before putting into the pool.
	// This is important so that pool entries don't hang
	// onto the underlying buffers.
	for i := range row {
		row[i] = parquet.Value{}
	}
	r.pool.Put(row[:0]) //nolint:all //SA6002
}

// estimateMarshalledSizeFromParquetRow estimates the byte size as marshalled into parquet.
// this is a very rough estimate and is generally 66%-100% of actual size.
func estimateMarshalledSizeFromParquetRow(row parquet.Row) (size int) {
	return len(row)
}
