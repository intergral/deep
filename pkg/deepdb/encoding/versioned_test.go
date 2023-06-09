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

package encoding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromVersionErrors(t *testing.T) {
	encoding, err := FromVersion("definitely-not-a-real-version")
	assert.Error(t, err)
	assert.Nil(t, encoding)
}

func TestAllVersions(t *testing.T) {
	for _, v := range AllEncodings() {
		encoding, err := FromVersion(v.Version())

		require.Equal(t, v.Version(), encoding.Version())
		require.NoError(t, err)
	}
}
