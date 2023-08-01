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
	"github.com/intergral/deep/pkg/deepql"
	"github.com/intergral/deep/pkg/util/test"
	"path"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectAttributes(snapshots []*Snapshot) map[string]string {
	attrs := make(map[string]string, 5)

	attrValue := func(attr Attribute) string {
		if attr.Value != nil {
			return *attr.Value
		}
		if attr.ValueInt != nil {
			return strconv.FormatInt(*attr.ValueInt, 10)
		}
		return ""
	}

	for _, snapshot := range snapshots {
		for _, attribute := range snapshot.Attributes {
			attrs[attribute.Key] = attrValue(attribute)
		}
	}

	return attrs
}

func TestBackendBlockSearchTags(t *testing.T) {
	snapshots := make([]*Snapshot, 10)
	for i := 0; i < 10; i++ {
		snapshots[i] = fullyPopulatedTestSnapshot(test.ValidSnapshotID(nil))
		snapshots[i].Attributes = nil
	}
	block := makeBackendBlockWithSnapshots(t, snapshots)
	attributes := collectAttributes(snapshots)

	foundAttrs := map[string]struct{}{}

	cb := func(s string) {
		foundAttrs[s] = struct{}{}
	}

	ctx := context.Background()
	err := block.SearchTags(ctx, cb, common.DefaultSearchOptions())
	require.NoError(t, err)

	// test that all attrs are in found attrs
	for key, _ := range attributes {
		_, ok := foundAttrs[key]
		require.True(t, ok)
	}
}

func TestBackendBlockSearchTagValues(t *testing.T) {
	snapshots := make([]*Snapshot, 10)
	for i := 0; i < 10; i++ {
		snapshots[i] = fullyPopulatedTestSnapshot(test.ValidSnapshotID(nil))
		snapshots[i].Attributes = nil
	}
	block := makeBackendBlockWithSnapshots(t, snapshots)
	attributes := collectAttributes(snapshots)

	ctx := context.Background()
	for key, val := range attributes {
		wasCalled := false
		cb := func(s string) {
			wasCalled = true
			assert.Equal(t, val, s, key)
		}

		err := block.SearchTagValues(ctx, key, cb, common.DefaultSearchOptions())
		require.NoError(t, err)
		require.True(t, wasCalled, key)
	}
}

func TestBackendBlockSearchTagValuesV2(t *testing.T) {
	block := makeBackendBlockWithSnapshots(t, []*Snapshot{fullyPopulatedTestSnapshot(common.ID{0})})

	testCases := []struct {
		tag  deepql.Attribute
		vals []deepql.Static
	}{
		// Attr present at both resource and snapshot level
		{deepql.MustParseIdentifier(".foo"), []deepql.Static{
			deepql.NewStaticString("abc"),
			deepql.NewStaticString("def"),
		}},
	}

	ctx := context.Background()
	for _, tc := range testCases {

		var got []deepql.Static
		cb := func(v deepql.Static) bool {
			got = append(got, v)
			return false
		}

		err := block.SearchTagValuesV2(ctx, tc.tag, cb, common.DefaultSearchOptions())
		require.NoError(t, err, tc.tag)
		require.Equal(t, tc.vals, got, tc.tag)
	}
}

func BenchmarkBackendBlockSearchTags(b *testing.B) {
	ctx := context.TODO()
	tenantID := "1"
	blockID := uuid.MustParse("3685ee3d-cbbf-4f36-bf28-93447a19dea6")

	r, _, _, err := local.New(&local.Config{
		Path: path.Join("/Users/marty/src/tmp/"),
	})
	require.NoError(b, err)

	rr := backend.NewReader(r)
	meta, err := rr.BlockMeta(ctx, blockID, tenantID)
	require.NoError(b, err)

	block := newBackendBlock(meta, rr)
	opts := common.DefaultSearchOptions()
	d := util.NewDistinctStringCollector(1_000_000)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := block.SearchTags(ctx, d.Collect, opts)
		require.NoError(b, err)
	}
}

func BenchmarkBackendBlockSearchTagValues(b *testing.B) {
	testCases := []string{
		"foo",
		"http.url",
	}

	ctx := context.TODO()
	tenantID := "1"
	blockID := uuid.MustParse("3685ee3d-cbbf-4f36-bf28-93447a19dea6")

	r, _, _, err := local.New(&local.Config{
		Path: path.Join("/Users/marty/src/tmp/"),
	})
	require.NoError(b, err)

	rr := backend.NewReader(r)
	meta, err := rr.BlockMeta(ctx, blockID, tenantID)
	require.NoError(b, err)

	block := newBackendBlock(meta, rr)
	opts := common.DefaultSearchOptions()

	for _, tc := range testCases {
		b.Run(tc, func(b *testing.B) {
			d := util.NewDistinctStringCollector(1_000_000)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				err := block.SearchTagValues(ctx, tc, d.Collect, opts)
				require.NoError(b, err)
			}
		})
	}
}
