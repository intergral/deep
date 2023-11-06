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
	"fmt"
	"io"
	"math/rand"
	"os"
	"testing"

	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/segmentio/parquet-go"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/util/test"
)

func TestProtoParquetRoundTrip(t *testing.T) {
	// This test round trips a proto Snapshot and checks that the transformation works as expected
	// Proto -> Parquet -> Proto

	snapshotIDA := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	expectedSnapshot := parquetToDeepSnapshot(fullyPopulatedTestSnapshot(snapshotIDA))

	parquetSnapshot := snapshotToParquet(snapshotIDA, expectedSnapshot, nil)
	actualSnapshot := parquetToDeepSnapshot(parquetSnapshot)
	assert.Equal(t, expectedSnapshot, actualSnapshot)

	assert.NotNil(t, expectedSnapshot.LogMsg)
	assert.Equal(t, expectedSnapshot.LogMsg, actualSnapshot.LogMsg)
}

func TestProtoToParquetEmptySnapshot(t *testing.T) {
	want := &Snapshot{
		ID:         make([]byte, 16),
		VarLookup:  map[string]Variable{},
		Frames:     make([]StackFrame, 0),
		Watches:    make([]WatchResult, 0),
		Attributes: make([]Attribute, 0),
	}

	got := snapshotToParquet(nil, &deeptp.Snapshot{}, nil)
	require.Equal(t, want, got)
}

func TestProtoParquetRando(t *testing.T) {
	trp := &Snapshot{}
	for i := 0; i < 100; i++ {
		batches := rand.Intn(15)
		id := test.ValidSnapshotID(nil)
		expectedSnapshot := test.GenerateSnapshot(batches, &test.GenerateOptions{Id: id})

		parqTr := snapshotToParquet(id, expectedSnapshot, trp)
		actualSnapshot := parquetToDeepSnapshot(parqTr)
		require.Equal(t, expectedSnapshot, actualSnapshot)
	}
}

func TestFieldsAreCleared(t *testing.T) {
	snapshotID := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	complexSnapshot := parquetToDeepSnapshot(fullyPopulatedTestSnapshot(snapshotID))
	simpleSnapshot := test.GenerateSnapshot(0, &test.GenerateOptions{Id: snapshotID})
	// first convert a Snapshot that sets all fields and then convert
	// a minimal Snapshot to make sure nothing bleeds through
	snapshot := &Snapshot{}
	_ = snapshotToParquet(snapshotID, complexSnapshot, snapshot)
	parqSnapshot := snapshotToParquet(snapshotID, simpleSnapshot, snapshot)
	actualSnapshot := parquetToDeepSnapshot(parqSnapshot)
	require.Equal(t, simpleSnapshot, actualSnapshot)
}

func TestParquetRowSizeEstimate(t *testing.T) {
	// use this test to parse actual Parquet files and compare the two methods of estimating row size
	s := []string{}

	for _, s := range s {
		estimateRowSize(t, s)
	}
}

func estimateRowSize(t *testing.T, name string) {
	f, err := os.OpenFile(name, os.O_RDONLY, 0o644)
	require.NoError(t, err)

	fi, err := f.Stat()
	require.NoError(t, err)

	pf, err := parquet.OpenFile(f, fi.Size())
	require.NoError(t, err)

	r := parquet.NewGenericReader[*Snapshot](pf)
	row := make([]*Snapshot, 1)

	totalProtoSize := int64(0)
	totalSnapshotSize := int64(0)
	for {
		_, err := r.Read(row)
		if err != nil {
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
		}

		tr := row[0]
		sch := parquet.SchemaOf(tr)
		row := sch.Deconstruct(nil, tr)

		totalProtoSize += int64(estimateMarshalledSizeFromParquetRow(row))
		totalSnapshotSize += int64(estimateMarshalledSizeFromSnapshot(tr))
	}

	fmt.Println(pf.Size(), ",", len(pf.RowGroups()), ",", totalProtoSize, ",", totalSnapshotSize)
}

func TestExtendReuseSlice(t *testing.T) {
	tcs := []struct {
		sz       int
		in       []int
		expected []int
	}{
		{
			sz:       0,
			in:       []int{1, 2, 3},
			expected: []int{},
		},
		{
			sz:       2,
			in:       []int{1, 2, 3},
			expected: []int{1, 2},
		},
		{
			sz:       5,
			in:       []int{1, 2, 3},
			expected: []int{1, 2, 3, 0, 0},
		},
	}

	for _, tc := range tcs {
		t.Run(fmt.Sprintf("%v", tc.sz), func(t *testing.T) {
			out := extendReuseSlice(tc.sz, tc.in)
			assert.Equal(t, tc.expected, out)
		})
	}
}

func BenchmarkExtendReuseSlice(b *testing.B) {
	in := []int{1, 2, 3}
	for i := 0; i < b.N; i++ {
		_ = extendReuseSlice(100, in)
	}
}
