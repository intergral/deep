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
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

func HexStringToSnapshotID(id string) ([]byte, error) {
	// The encoding/hex package does not handle non-hex characters.
	// Ensure the ID has only the proper characters
	for pos, idChar := range strings.Split(id, "") {
		if (idChar >= "a" && idChar <= "f") ||
			(idChar >= "A" && idChar <= "F") ||
			(idChar >= "0" && idChar <= "9") {
			continue
		} else {
			return nil, fmt.Errorf("snapshot IDs can only contain hex characters: invalid character '%s' at position %d", idChar, pos+1)
		}
	}

	// the encoding/hex package does not like odd length strings.
	// just append a bit here
	if len(id)%2 == 1 {
		id = "0" + id
	}

	byteID, err := hex.DecodeString(id)
	if err != nil {
		return nil, err
	}

	size := len(byteID)
	if size > 16 {
		return nil, errors.New("snapshot IDs can't be larger than 128 bits")
	}
	if size < 16 {
		byteID = append(make([]byte, 16-size), byteID...)
	}

	return byteID, nil
}

// SnapshotIDToHexString converts a snapshot ID to its string representation and removes any leading zeros.
func SnapshotIDToHexString(byteID []byte) string {
	id := hex.EncodeToString(byteID)
	// remove leading zeros
	id = strings.TrimLeft(id, "0")
	return id
}

// EqualHexStringSnapshotIDs compares two snapshot ID strings and compares the
// resulting bytes after padding.  Returns true unless there is a reason not to.
func EqualHexStringSnapshotIDs(a, b string) (bool, error) {
	aa, err := HexStringToSnapshotID(a)
	if err != nil {
		return false, err
	}
	bb, err := HexStringToSnapshotID(b)
	if err != nil {
		return false, err
	}

	return bytes.Equal(aa, bb), nil
}

func PadSnapshotIDTo16Bytes(snapshotID []byte) []byte {
	if len(snapshotID) > 16 {
		return snapshotID[len(snapshotID)-16:]
	}

	if len(snapshotID) == 16 {
		return snapshotID
	}

	padded := make([]byte, 16)
	copy(padded[16-len(snapshotID):], snapshotID)

	return padded
}
