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

package io

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testBufLength = 10

func TestReadAllWithEstimate(t *testing.T) {
	buf := make([]byte, testBufLength)
	_, err := rand.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, testBufLength, len(buf))
	assert.Equal(t, testBufLength, cap(buf))

	actualBuf, err := ReadAllWithEstimate(bytes.NewReader(buf), int64(testBufLength))
	assert.NoError(t, err)
	assert.Equal(t, buf, actualBuf)
	assert.Equal(t, testBufLength, len(actualBuf))
	assert.Equal(t, testBufLength+1, cap(actualBuf)) // one extra byte used in ReadAllWithEstimate
}
