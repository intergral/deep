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
	"io"

	"github.com/pkg/errors"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
)

func CopyBlock(ctx context.Context, fromMeta, toMeta *backend.BlockMeta, from backend.Reader, to backend.Writer) error {
	// Copy streams, efficient but can't cache.
	copyStream := func(name string) error {
		reader, size, err := from.StreamReader(ctx, name, fromMeta.BlockID, fromMeta.TenantID)
		if err != nil {
			return errors.Wrapf(err, "error reading %s", name)
		}
		defer func(reader io.ReadCloser) {
			_ = reader.Close()
		}(reader)

		return to.StreamWriter(ctx, name, toMeta.BlockID, toMeta.TenantID, reader, size)
	}

	// Read entire object and attempt to cache
	copyFunc := func(name string) error {
		b, err := from.Read(ctx, name, fromMeta.BlockID, fromMeta.TenantID, true)
		if err != nil {
			return errors.Wrapf(err, "error reading %s", name)
		}

		return to.Write(ctx, name, toMeta.BlockID, toMeta.TenantID, b, true)
	}

	// Data
	err := copyStream(DataFileName)
	if err != nil {
		return err
	}

	// Bloom
	for i := 0; i < common.ValidateShardCount(int(fromMeta.BloomShardCount)); i++ {
		err = copyFunc(common.BloomName(i))
		if err != nil {
			return err
		}
	}

	// Meta
	err = to.WriteBlockMeta(ctx, toMeta)
	return err
}

func writeBlockMeta(ctx context.Context, w backend.Writer, meta *backend.BlockMeta, bloom *common.ShardedBloomFilter) error {
	// bloom
	blooms, err := bloom.Marshal()
	if err != nil {
		return err
	}
	for i, bloom := range blooms {
		nameBloom := common.BloomName(i)
		err := w.Write(ctx, nameBloom, meta.BlockID, meta.TenantID, bloom, true)
		if err != nil {
			return fmt.Errorf("unexpected error writing bloom-%d %w", i, err)
		}
	}

	// meta
	err = w.WriteBlockMeta(ctx, meta)
	if err != nil {
		return fmt.Errorf("unexpected error writing meta %w", err)
	}

	return nil
}
