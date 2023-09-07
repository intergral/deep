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

package test

import (
	crand "crypto/rand"
	"math/rand"
)

func ValidSnapshotID(snapshotID []byte) []byte {
	if len(snapshotID) == 0 {
		snapshotID = make([]byte, 16)
		_, err := crand.Read(snapshotID)
		if err != nil {
			panic(err)
		}
	}

	for len(snapshotID) < 16 {
		snapshotID = append(snapshotID, 0)
	}

	return snapshotID
}

func RandomString() string {
	return RandomStringLen(10)
}

func RandomStringLen(length int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	s := make([]rune, length)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
