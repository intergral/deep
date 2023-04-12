package common

import (
	"context"
	"github.com/intergral/deep/pkg/deeppb"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"time"

	"github.com/go-kit/log"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepql"
	"github.com/intergral/deep/pkg/model"
)

type Finder interface {
	FindSnapshotByID(ctx context.Context, id ID, opts SearchOptions) (*deep_tp.Snapshot, error)
}

type TagCallback func(t string)

type TagCallbackV2 func(deepql.Static) (stop bool)

type Searcher interface {
	Search(ctx context.Context, req *deeppb.SearchRequest, opts SearchOptions) (*deeppb.SearchResponse, error)
	SearchTags(ctx context.Context, cb TagCallback, opts SearchOptions) error
	SearchTagValues(ctx context.Context, tag string, cb TagCallback, opts SearchOptions) error
	SearchTagValuesV2(ctx context.Context, tag deepql.Attribute, cb TagCallbackV2, opts SearchOptions) error

	Fetch(context.Context, deepql.FetchSnapshotRequest, SearchOptions) (deepql.FetchSnapshotResponse, error)
}

type CacheControl struct {
	Footer      bool
	ColumnIndex bool
	OffsetIndex bool
}

type SearchOptions struct {
	ChunkSizeBytes     uint32 // Buffer size to read from backend storage.
	StartPage          int    // Controls searching only a subset of the block. Which page to begin searching at.
	TotalPages         int    // Controls searching only a subset of the block. How many pages to search.
	MaxBytes           int    // Max allowable trace size in bytes. Traces exceeding this are not searched.
	PrefetchTraceCount int    // How many traces to prefetch async.
	ReadBufferCount    int
	ReadBufferSize     int
	CacheControl       CacheControl
}

// DefaultSearchOptions() is used in a lot of places such as local ingester searches. It is important
// in these cases to set a reasonable read buffer size and count to prevent constant tiny readranges
// against the local backend.
// TODO: Note that there is another method of creating "default search options" that looks like this:
// deepdb.SearchConfig{}.ApplyToOptions(&searchOpts). we should consolidate these.
func DefaultSearchOptions() SearchOptions {
	return SearchOptions{
		ReadBufferCount: 32,
		ReadBufferSize:  1024 * 1024,
		ChunkSizeBytes:  4 * 1024 * 1024,
	}
}

type Compactor interface {
	Compact(ctx context.Context, l log.Logger, r backend.Reader, writerCallback func(*backend.BlockMeta, time.Time) backend.Writer, inputs []*backend.BlockMeta) ([]*backend.BlockMeta, error)
}

type CompactionOptions struct {
	ChunkSizeBytes     uint32
	FlushSizeBytes     uint32
	IteratorBufferSize int // How many traces to prefetch async.
	MaxBytesPerTrace   int
	OutputBlocks       uint8
	BlockConfig        BlockConfig
	Combiner           model.ObjectCombiner

	ObjectsCombined func(compactionLevel, objects int)
	ObjectsWritten  func(compactionLevel, objects int)
	BytesWritten    func(compactionLevel, bytes int)
	SpansDiscarded  func(traceID string, spans int)
}

type Iterator interface {
	Next(ctx context.Context) (ID, *deep_tp.Snapshot, error)
	Close()
}

type BackendBlock interface {
	Finder
	Searcher

	BlockMeta() *backend.BlockMeta
}

type WALBlock interface {
	BackendBlock

	// Append the given trace to the block. Must be safe for concurrent use with read operations.
	Append(id ID, b []byte, start uint32) error

	// Flush any unbuffered data to disk.  Must be safe for concurrent use with read operations.
	Flush() error

	DataLength() uint64
	Iterator() (Iterator, error)
	Clear() error
}
