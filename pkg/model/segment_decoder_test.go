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

package model

import (
	"math/rand"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/intergral/deep/pkg/model/decoder"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/require"
)

func TestSegmentDecoderToObjectDecoder(t *testing.T) {
	for _, e := range AllEncodings {
		t.Run(e, func(t *testing.T) {
			objectDecoder, err := NewObjectDecoder(e)
			require.NoError(t, err)

			segmentDecoder, err := NewSegmentDecoder(e)
			require.NoError(t, err)

			// random snapshot
			snapshot := test.GenerateSnapshot(100, nil)

			segment, err := segmentDecoder.PrepareForWrite(snapshot, 0)
			require.NoError(t, err)

			// segment prepareforread
			actual, err := segmentDecoder.PrepareForRead(segment)
			require.NoError(t, err)
			require.True(t, proto.Equal(snapshot, actual))

			// convert to object
			object, err := segmentDecoder.ToObject(segment)
			require.NoError(t, err)

			actual, err = objectDecoder.PrepareForRead(object)
			require.NoError(t, err)
			require.True(t, proto.Equal(snapshot, actual))
		})
	}
}

func TestSegmentDecoderToObjectDecoderRange(t *testing.T) {
	for _, e := range AllEncodings {
		t.Run(e, func(t *testing.T) {
			start := rand.Uint32()

			objectDecoder, err := NewObjectDecoder(e)
			require.NoError(t, err)

			segmentDecoder, err := NewSegmentDecoder(e)
			require.NoError(t, err)

			// random snapshot
			snapshot := test.GenerateSnapshot(100, nil)

			segment, err := segmentDecoder.PrepareForWrite(snapshot, start)
			require.NoError(t, err)

			// convert to object
			object, err := segmentDecoder.ToObject(segment)
			require.NoError(t, err)

			// test range
			actualStart, err := objectDecoder.FastRange(object)
			if err == decoder.ErrUnsupported {
				return
			}

			require.NoError(t, err)
			require.Equal(t, start, actualStart)
		})
	}
}

func TestSegmentDecoderFastRange(t *testing.T) {
	for _, e := range AllEncodings {
		t.Run(e, func(t *testing.T) {
			start := rand.Uint32()

			segmentDecoder, err := NewSegmentDecoder(e)
			require.NoError(t, err)

			// random snapshot
			snapshot := test.GenerateSnapshot(100, nil)

			segment, err := segmentDecoder.PrepareForWrite(snapshot, start)
			require.NoError(t, err)

			// test range
			actualStart, err := segmentDecoder.FastRange(segment)
			if err == decoder.ErrUnsupported {
				return
			}

			require.NoError(t, err)
			require.Equal(t, start, actualStart)
		})
	}
}
