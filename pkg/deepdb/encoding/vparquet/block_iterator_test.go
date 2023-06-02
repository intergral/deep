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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
)

func TestRawIteratorReadsAllRows(t *testing.T) {
	rawR, _, _, err := local.New(&local.Config{
		Path: "./test-data",
	})
	require.NoError(t, err)

	r := backend.NewReader(rawR)
	ctx := context.Background()

	blocks, err := r.Blocks(ctx, "single-tenant")
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	meta, err := r.BlockMeta(ctx, blocks[0], "single-tenant")
	require.NoError(t, err)

	b := newBackendBlock(meta, r)

	iter, err := b.RawIterator(context.Background(), newRowPool(10))
	require.NoError(t, err)
	defer iter.Close()

	actualCount := 0
	for {
		_, tr, err := iter.Next(context.Background())
		if tr == nil {
			break
		}
		actualCount++
		require.NoError(t, err)
	}

	require.Equal(t, meta.TotalObjects, actualCount)
}
