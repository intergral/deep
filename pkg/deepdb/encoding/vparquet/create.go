package vparquet

import (
	"context"
	"encoding/binary"
	"github.com/segmentio/parquet-go"
	"io"

	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	deep_io "github.com/intergral/deep/pkg/io"
	"github.com/pkg/errors"
)

type backendWriter struct {
	ctx      context.Context
	w        backend.Writer
	name     string
	blockID  uuid.UUID
	tenantID string
	tracker  backend.AppendTracker
}

var _ io.WriteCloser = (*backendWriter)(nil)

func (b *backendWriter) Write(p []byte) (n int, err error) {
	b.tracker, err = b.w.Append(b.ctx, b.name, b.blockID, b.tenantID, b.tracker, p)
	return len(p), err
}

func (b *backendWriter) Close() error {
	return b.w.CloseAppend(b.ctx, b.tracker)
}

func CreateBlock(ctx context.Context, cfg *common.BlockConfig, meta *backend.BlockMeta, i common.Iterator, r backend.Reader, to backend.Writer) (*backend.BlockMeta, error) {
	s := newStreamingBlock(ctx, cfg, meta, r, to, deep_io.NewBufferedWriter)

	var next func(context.Context) (common.ID, parquet.Row, error)

	if ii, ok := i.(*commonIterator); ok {
		// Use interal iterator and avoid translation to/from proto
		next = ii.NextRow
	} else {
		// Need to convert from proto->parquet obj
		trp := &Snapshot{}
		sch := parquet.SchemaOf(trp)
		next = func(context.Context) (common.ID, parquet.Row, error) {
			id, tr, err := i.Next(ctx)
			if err == io.EOF || tr == nil {
				return id, nil, err
			}

			// Copy ID to allow it to escape the iterator.
			id = append([]byte(nil), id...)

			trp = snapshotToParquet(id, tr, trp)

			row := sch.Deconstruct(completeBlockRowPool.Get(), trp)

			return id, row, nil
		}
	}

	for {
		id, row, err := next(ctx)
		if err == io.EOF || row == nil {
			break
		}

		err = s.AddRaw(id, row, 0) // start and end time of the wal meta are used.
		if err != nil {
			return nil, err
		}
		completeBlockRowPool.Put(row)

		if s.EstimatedBufferedBytes() > cfg.RowGroupSizeBytes {
			_, err = s.Flush()
			if err != nil {
				return nil, err
			}
		}
	}

	_, err := s.Complete()
	if err != nil {
		return nil, err
	}

	return s.meta, nil
}

type streamingBlock struct {
	ctx   context.Context
	bloom *common.ShardedBloomFilter
	meta  *backend.BlockMeta
	bw    deep_io.BufferedWriteFlusher
	pw    *parquet.GenericWriter[*Snapshot]
	w     *backendWriter
	r     backend.Reader
	to    backend.Writer

	currentBufferedTraces int
	currentBufferedBytes  int
}

func newStreamingBlock(ctx context.Context, cfg *common.BlockConfig, meta *backend.BlockMeta, r backend.Reader, to backend.Writer, createBufferedWriter func(w io.Writer) deep_io.BufferedWriteFlusher) *streamingBlock {
	newMeta := backend.NewBlockMeta(meta.TenantID, meta.BlockID, VersionString, backend.EncNone, "")
	newMeta.StartTime = meta.StartTime
	newMeta.EndTime = meta.EndTime

	// TotalObjects is used here an an estimated count for the bloom filter.
	// The real number of objects is tracked below.
	bloom := common.NewBloom(cfg.BloomFP, uint(cfg.BloomShardSizeBytes), uint(meta.TotalObjects))

	w := &backendWriter{ctx, to, DataFileName, meta.BlockID, meta.TenantID, nil}
	bw := createBufferedWriter(w)
	pw := parquet.NewGenericWriter[*Snapshot](bw)

	return &streamingBlock{
		ctx:   ctx,
		meta:  newMeta,
		bloom: bloom,
		bw:    bw,
		pw:    pw,
		w:     w,
		r:     r,
		to:    to,
	}
}

func (b *streamingBlock) Add(tr *Snapshot, start uint32) error {
	_, err := b.pw.Write([]*Snapshot{tr})
	if err != nil {
		return err
	}
	id := tr.Id

	b.bloom.Add(id)
	b.meta.ObjectAdded(id, start)
	b.currentBufferedTraces++
	b.currentBufferedBytes += estimateMarshalledSizeFromSnapshot(tr)

	return nil
}

func (b *streamingBlock) AddRaw(id []byte, row parquet.Row, start uint32) error {
	_, err := b.pw.WriteRows([]parquet.Row{row})
	if err != nil {
		return err
	}

	b.bloom.Add(id)
	b.meta.ObjectAdded(id, start)
	b.currentBufferedTraces++
	b.currentBufferedBytes += estimateMarshalledSizeFromParquetRow(row)

	return nil
}

func (b *streamingBlock) EstimatedBufferedBytes() int {
	return b.currentBufferedBytes
}

func (b *streamingBlock) CurrentBufferedObjects() int {
	return b.currentBufferedTraces
}

func (b *streamingBlock) Flush() (int, error) {
	// Flush row group
	err := b.pw.Flush()
	if err != nil {
		return 0, err
	}

	n := b.bw.Len()
	b.meta.Size += uint64(n)
	b.meta.TotalRecords++
	b.currentBufferedTraces = 0
	b.currentBufferedBytes = 0

	// Flush to underlying writer
	return n, b.bw.Flush()
}

func (b *streamingBlock) Complete() (int, error) {
	// Flush final row group
	b.meta.TotalRecords++
	err := b.pw.Flush()
	if err != nil {
		return 0, err
	}

	// Close parquet file. This writes the footer and metadata.
	err = b.pw.Close()
	if err != nil {
		return 0, err
	}

	// Now Flush and close out in-memory buffer
	n := b.bw.Len()
	b.meta.Size += uint64(n)
	err = b.bw.Flush()
	if err != nil {
		return 0, err
	}

	err = b.bw.Close()
	if err != nil {
		return 0, err
	}

	err = b.w.Close()
	if err != nil {
		return 0, err
	}

	// Read the footer size out of the parquet footer
	buf := make([]byte, 8)
	err = b.r.ReadRange(b.ctx, DataFileName, b.meta.BlockID, b.meta.TenantID, b.meta.Size-8, buf, false)
	if err != nil {
		return 0, errors.Wrap(err, "error reading parquet file footer")
	}
	if string(buf[4:8]) != "PAR1" {
		return 0, errors.New("Failed to confirm magic footer while writing a new parquet block")
	}
	b.meta.FooterSize = binary.LittleEndian.Uint32(buf[0:4])

	b.meta.BloomShardCount = uint16(b.bloom.GetShardCount())

	return n, writeBlockMeta(b.ctx, b.to, b.meta, b.bloom)
}

// estimateMarshalledSizeFromSnapshot attempts to estimate the size of trace in bytes. This is used to make choose
// when to cut a row group during block creation.
// TODO: This function regularly estimates lower values then estimateProtoSize() and the size
// of the actual proto. It's also quite inefficient. Perhaps just using static values per span or attribute
// would be a better choice?
func estimateMarshalledSizeFromSnapshot(tr *Snapshot) (size int) {
	size += 10 // 10 snapshot lvl fields

	size += estimateAttrSize(tr.Resource.Attrs)
	size += estimateAttrSize(tr.Attributes)
	size += 5                     // 5 tracepoint lvl fields
	size += 6 * len(tr.Watches)   // 6 watch result fields
	size += 11 * len(tr.Frames)   // frame fields
	size += 5 * len(tr.VarLookup) // var look up fields

	return
}

func estimateAttrSize(attrs []Attribute) (size int) {
	return len(attrs) * 7 // 7 attribute lvl fields
}
