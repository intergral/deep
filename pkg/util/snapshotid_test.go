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

package util

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHexStringToSnapshotID(t *testing.T) {
	tc := []struct {
		id          string
		expected    []byte
		expectError error
	}{
		{
			id:       "12",
			expected: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12},
		},
		{
			id:       "1234567890abcdef", // 64 bit
			expected: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
		},
		{
			id:       "1234567890abcdef1234567890abcdef", // 128 bit
			expected: []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
		},
		{
			id:          "121234567890abcdef1234567890abcdef", // value too long
			expected:    []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
			expectError: errors.New("snapshot IDs can't be larger than 128 bits"),
		},
		{
			id:       "234567890abcdef", // odd length
			expected: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
		},
		{
			id:          "1234567890abcdef ", // trailing space
			expected:    nil,
			expectError: errors.New("snapshot IDs can only contain hex characters: invalid character ' ' at position 17"),
		},
	}

	for _, tt := range tc {
		t.Run(tt.id, func(t *testing.T) {
			actual, err := HexStringToSnapshotID(tt.id)

			if tt.expectError != nil {
				assert.Equal(t, tt.expectError, err)
				assert.Nil(t, actual)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestSnapshotIDToFromString(t *testing.T) {
	id, _ := HexStringToSnapshotID("123")

	hexString := SnapshotIDToHexString(id)

	assert.Equal(t, "123", hexString)
}

func TestSnapshotIDToHexString(t *testing.T) {
	tc := []struct {
		byteID     []byte
		snapshotID string
	}{
		{
			byteID:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12},
			snapshotID: "12",
		},
		{
			byteID:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
			snapshotID: "1234567890abcdef", // 64 bit
		},
		{
			byteID:     []byte{0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef, 0x12, 0x34, 0x56, 0x78, 0x90, 0xab, 0xcd, 0xef},
			snapshotID: "1234567890abcdef1234567890abcdef", // 128 bit
		},
		{
			byteID:     []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x12, 0xa0},
			snapshotID: "12a0", // trailing zero
		},
	}

	for _, tt := range tc {
		t.Run(tt.snapshotID, func(t *testing.T) {
			actual := SnapshotIDToHexString(tt.byteID)

			assert.Equal(t, tt.snapshotID, actual)
		})
	}
}

func TestEqualHexStringSnapshotIDs(t *testing.T) {
	a := "82f6471b46d25e23418a0a99d4c2cda"
	b := "082f6471b46d25e23418a0a99d4c2cda"

	v, err := EqualHexStringSnapshotIDs(a, b)
	assert.Nil(t, err)
	assert.True(t, v)
}

func TestPadSnapshotIDTo16Bytes(t *testing.T) {
	tc := []struct {
		name     string
		tid      []byte
		expected []byte
	}{
		{
			name:     "small",
			tid:      []byte{0x01, 0x02},
			expected: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02},
		},
		{
			name:     "exact",
			tid:      []byte{0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02},
			expected: []byte{0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02},
		},
		{ // least significant bits are preserved
			name:     "large",
			tid:      []byte{0x05, 0x05, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02},
			expected: []byte{0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02},
		},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, PadSnapshotIDTo16Bytes(tt.tid))
		})
	}
}
