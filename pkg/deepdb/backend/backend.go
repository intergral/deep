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

package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
)

var (
	ErrDoesNotExist  = fmt.Errorf("does not exist")
	ErrEmptyTenantID = fmt.Errorf("empty tenant id")
	ErrEmptyBlockID  = fmt.Errorf("empty block id")
	ErrBadSeedFile   = fmt.Errorf("bad seed file")
)

// AppendTracker is an empty interface usable by the backend to track a long-running append operation
type AppendTracker interface{}

// Writer is a collection of methods to write data to deepdb backends
type Writer interface {
	// Write is for in memory data. shouldCache specifies whether caching should be attempted.
	Write(ctx context.Context, name string, blockID uuid.UUID, tenantID string, buffer []byte, shouldCache bool) error
	// StreamWriter is for larger data payloads streamed through an io.Reader.  It is expected this will _not_ be cached.
	StreamWriter(ctx context.Context, name string, blockID uuid.UUID, tenantID string, data io.Reader, size int64) error
	// WriteBlockMeta writes a block meta to its blocks
	WriteBlockMeta(ctx context.Context, meta *BlockMeta) error
	// Append starts or continues an Append job. Pass nil to AppendTracker to start a job.
	Append(ctx context.Context, name string, blockID uuid.UUID, tenantID string, tracker AppendTracker, buffer []byte) (AppendTracker, error)
	// CloseAppend closes any resources associated with the AppendTracker
	CloseAppend(ctx context.Context, tracker AppendTracker) error
	// WriteTenantIndex writes the two meta slices as a tenant index
	WriteTenantIndex(ctx context.Context, tenantID string, meta []*BlockMeta, compactedMeta []*CompactedBlockMeta) error
	// WriteTracepointBlock writes the tracepoint block for the given tenantID
	WriteTracepointBlock(ctx context.Context, tenantID string, data *bytes.Reader, size int64) error
}

// Reader is a collection of methods to read data from deepdb backends
type Reader interface {
	// Read is for reading entire objects from the backend. There will be an attempt to retrieve this
	// from cache if shouldCache is true.
	Read(ctx context.Context, name string, blockID uuid.UUID, tenantID string, shouldCache bool) ([]byte, error)
	// StreamReader is for streaming entire objects from the backend.  It is expected this will _not_ be cached.
	StreamReader(ctx context.Context, name string, blockID uuid.UUID, tenantID string) (io.ReadCloser, int64, error)
	// ReadRange is for reading parts of large objects from the backend.
	// There will be an attempt to retrieve this from cache if shouldCache is true. Cache key will be tenantID:blockID:offset:bufferLength
	ReadRange(ctx context.Context, name string, blockID uuid.UUID, tenantID string, offset uint64, buffer []byte, shouldCache bool) error
	// Tenants returns a list of all tenants in a backend
	Tenants(ctx context.Context) ([]string, error)
	// Blocks returns a list of block UUIDs given a tenant
	Blocks(ctx context.Context, tenantID string) ([]uuid.UUID, error)
	// BlockMeta returns the block meta given a block and tenant id
	BlockMeta(ctx context.Context, blockID uuid.UUID, tenantID string) (*BlockMeta, error)
	// TenantIndex returns lists of all metas given a tenant
	TenantIndex(ctx context.Context, tenantID string) (*TenantIndex, error)
	// Shutdown shuts...down?
	Shutdown()
	// ReadTracepointBlock reads the tracepoint block for the given tenantID
	ReadTracepointBlock(ctx context.Context, tenantID string) (io.ReadCloser, int64, error)
}

// Compactor is a collection of methods to interact with compacted elements of a deepdb block
type Compactor interface {
	// MarkBlockCompacted marks a block as compacted. Call this after a block has been successfully compacted to a new block
	MarkBlockCompacted(blockID uuid.UUID, tenantID string) error
	// ClearBlock removes a block from the backend
	ClearBlock(blockID uuid.UUID, tenantID string) error
	// CompactedBlockMeta returns the compacted block meta given a block and tenant id
	CompactedBlockMeta(blockID uuid.UUID, tenantID string) (*CompactedBlockMeta, error)
}
