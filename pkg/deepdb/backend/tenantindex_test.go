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
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIndexMarshalUnmarshal(t *testing.T) {
	tests := []struct {
		idx *TenantIndex
	}{
		{
			idx: &TenantIndex{},
		},
		{
			idx: &TenantIndex{
				Meta: []*BlockMeta{
					NewBlockMeta("test", uuid.New(), "v1", EncGZIP, "adsf"),
					NewBlockMeta("test", uuid.New(), "v2", EncNone, "adsf"),
					NewBlockMeta("test", uuid.New(), "v3", EncLZ4_4M, "adsf"),
				},
			},
		},
		{
			idx: &TenantIndex{
				CompactedMeta: []*CompactedBlockMeta{
					{
						BlockMeta:     *NewBlockMeta("test", uuid.New(), "v1", EncGZIP, "adsf"),
						CompactedTime: time.Now(),
					},
					{
						BlockMeta:     *NewBlockMeta("test", uuid.New(), "v1", EncZstd, "adsf"),
						CompactedTime: time.Now(),
					},
					{
						BlockMeta:     *NewBlockMeta("test", uuid.New(), "v1", EncSnappy, "adsf"),
						CompactedTime: time.Now(),
					},
				},
			},
		},
		{
			idx: &TenantIndex{
				Meta: []*BlockMeta{
					NewBlockMeta("test", uuid.New(), "v1", EncGZIP, "adsf"),
					NewBlockMeta("test", uuid.New(), "v2", EncNone, "adsf"),
					NewBlockMeta("test", uuid.New(), "v3", EncLZ4_4M, "adsf"),
				},
				CompactedMeta: []*CompactedBlockMeta{
					{
						BlockMeta:     *NewBlockMeta("test", uuid.New(), "v1", EncGZIP, "adsf"),
						CompactedTime: time.Now(),
					},
					{
						BlockMeta:     *NewBlockMeta("test", uuid.New(), "v1", EncZstd, "adsf"),
						CompactedTime: time.Now(),
					},
					{
						BlockMeta:     *NewBlockMeta("test", uuid.New(), "v1", EncSnappy, "adsf"),
						CompactedTime: time.Now(),
					},
				},
			},
		},
	}

	for _, tc := range tests {
		buff, err := tc.idx.marshal()
		require.NoError(t, err)

		actual := &TenantIndex{}
		err = actual.unmarshal(buff)
		require.NoError(t, err)

		// cmp.Equal used due to time marshalling: https://github.com/stretchr/testify/issues/502
		assert.True(t, cmp.Equal(tc.idx, actual))
	}
}

func TestIndexUnmarshalErrors(t *testing.T) {
	test := &TenantIndex{}
	err := test.unmarshal([]byte("bad data"))
	assert.Error(t, err)
}
