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
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"testing"

	"github.com/intergral/deep/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotStartEndTime(t *testing.T) {
	s := model.MustNewSegmentDecoder(model.CurrentEncoding)

	tr := newLiveSnapshot(nil, 0)

	// initial push
	buff, err := s.PrepareForWrite(&deep_tp.Snapshot{}, 10)
	require.NoError(t, err)
	err = tr.Push(context.Background(), "test", buff)
	require.NoError(t, err)

	assert.Equal(t, uint32(10), tr.start)

	// overwrite start
	buff, err = s.PrepareForWrite(&deep_tp.Snapshot{}, 5)
	require.NoError(t, err)
	err = tr.Push(context.Background(), "test", buff)
	require.NoError(t, err)

	assert.Equal(t, uint32(5), tr.start)
}
