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

package common

import (
	"context"
	"github.com/intergral/deep/pkg/deeppb"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"time"

	"github.com/go-kit/log"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepql"
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
	ChunkSizeBytes        uint32 // Buffer size to read from backend storage.
	StartPage             int    // Controls searching only a subset of the block. Which page to begin searching at.
	TotalPages            int    // Controls searching only a subset of the block. How many pages to search.
	MaxBytes              int    // Max allowable snapshot size in bytes. Snapshots exceeding this are not searched.
	PrefetchSnapshotCount int    // How many snapshots to prefetch async.
	ReadBufferCount       int
	ReadBufferSize        int
	CacheControl          CacheControl
}

// DefaultSearchOptions is used in a lot of places such as local ingester searches. It is important
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
	ChunkSizeBytes      uint32
	FlushSizeBytes      uint32
	IteratorBufferSize  int // How many snapshots to prefetch async.
	MaxBytesPerSnapshot int
	OutputBlocks        uint8
	BlockConfig         BlockConfig

	ObjectsWritten     func(compactionLevel, objects int)
	BytesWritten       func(compactionLevel, bytes int)
	SnapshotsDiscarded func(snapshotID string, count int)
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

	// Append the given snapshot to the block. Must be safe for concurrent use with read operations.
	Append(id ID, b []byte, start uint32) error

	// Flush any unbuffered data to disk.  Must be safe for concurrent use with read operations.
	Flush() error

	DataLength() uint64
	Iterator() (Iterator, error)
	Clear() error
}
