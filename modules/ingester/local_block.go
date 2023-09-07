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

package ingester

import (
	"context"
	"time"

	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"

	"go.uber.org/atomic"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

const nameFlushed = "flushed"

// localBlock is a block stored in a local storage.  It can be searched and flushed to a remote backend, and
// permanently tracks the flushed time with a special file in the block
type localBlock struct {
	common.BackendBlock
	reader backend.Reader
	writer backend.Writer

	flushedTime atomic.Int64 // protecting flushedTime b/c it's accessed from the store on flush and from the ingester tenantBlockManager checking flush time
}

var _ common.Finder = (*localBlock)(nil)

// newLocalBlock creates a local block
func newLocalBlock(ctx context.Context, existingBlock common.BackendBlock, l *local.Backend) *localBlock {
	c := &localBlock{
		BackendBlock: existingBlock,
		reader:       backend.NewReader(l),
		writer:       backend.NewWriter(l),
	}

	flushedBytes, err := c.reader.Read(ctx, nameFlushed, c.BlockMeta().BlockID, c.BlockMeta().TenantID, false)
	if err == nil {
		flushedTime := time.Time{}
		err = flushedTime.UnmarshalText(flushedBytes)
		if err == nil {
			c.flushedTime.Store(flushedTime.Unix())
		}
	}

	return c
}

// FindSnapshotByID will scan the local block for the id
func (c *localBlock) FindSnapshotByID(ctx context.Context, id common.ID, opts common.SearchOptions) (*deep_tp.Snapshot, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "localBlock.FindSnapshotByID")
	defer span.Finish()
	// local block is a local version of the backend lock so just pass to the backend block
	return c.BackendBlock.FindSnapshotByID(ctx, id, opts)
}

// FlushedTime returns the time the block was flushed.  Will return 0
//
//	if the block was never flushed
func (c *localBlock) FlushedTime() time.Time {
	unixTime := c.flushedTime.Load()
	if unixTime == 0 {
		return time.Time{} // return 0 time.  0 unix time is jan 1, 1970
	}
	return time.Unix(unixTime, 0)
}

func (c *localBlock) SetFlushed(ctx context.Context) error {
	flushedTime := time.Now()
	flushedBytes, err := flushedTime.MarshalText()
	if err != nil {
		return errors.Wrap(err, "error marshalling flush time to text")
	}

	err = c.writer.Write(ctx, nameFlushed, c.BlockMeta().BlockID, c.BlockMeta().TenantID, flushedBytes, false)
	if err != nil {
		return errors.Wrap(err, "error writing ingester block flushed file")
	}

	c.flushedTime.Store(flushedTime.Unix())
	return nil
}

func (c *localBlock) Write(ctx context.Context, w backend.Writer) error {
	err := encoding.CopyBlock(ctx, c.BlockMeta(), c.reader, w)
	if err != nil {
		return errors.Wrap(err, "error copying block from local to remote backend")
	}

	err = c.SetFlushed(ctx)
	return err
}
