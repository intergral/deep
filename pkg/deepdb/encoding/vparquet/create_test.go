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
	"io"
	"testing"
	"time"

	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"

	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/require"
)

func TestCreateBlockHonorsSnapshotStartEndTimesFromWalMeta(t *testing.T) {
	ctx := context.Background()

	rawR, rawW, _, err := local.New(&local.Config{
		Path: t.TempDir(),
	})
	require.NoError(t, err)

	r := backend.NewReader(rawR)
	w := backend.NewWriter(rawW)

	iter := newTestIterator()

	iter.Add(test.GenerateSnapshot(1, nil), 100, 401)
	iter.Add(test.GenerateSnapshot(2, nil), 101, 402)
	iter.Add(test.GenerateSnapshot(3, nil), 102, 403)

	cfg := &common.BlockConfig{
		BloomFP:             0.01,
		BloomShardSizeBytes: 100 * 1024,
	}

	meta := backend.NewBlockMeta("fake", uuid.New(), VersionString, backend.EncNone, "")
	meta.TotalObjects = 1
	meta.StartTime = time.Unix(300, 0)
	meta.EndTime = time.Unix(305, 0)

	outMeta, err := CreateBlock(ctx, cfg, meta, iter, r, w)
	require.NoError(t, err)
	require.Equal(t, 300, int(outMeta.StartTime.Unix()))
	require.Equal(t, 305, int(outMeta.EndTime.Unix()))
}

type testIterator struct {
	snapshots []*deep_tp.Snapshot
}

var _ common.Iterator = (*testIterator)(nil)

func newTestIterator() *testIterator {
	return &testIterator{}
}

func (i *testIterator) Add(tr *deep_tp.Snapshot, start, end uint32) {
	i.snapshots = append(i.snapshots, tr)
}

func (i *testIterator) Next(ctx context.Context) (common.ID, *deep_tp.Snapshot, error) {
	if len(i.snapshots) == 0 {
		return nil, nil, io.EOF
	}
	tr := i.snapshots[0]
	i.snapshots = i.snapshots[1:]
	return nil, tr, nil
}

func (i *testIterator) Close() {
}
