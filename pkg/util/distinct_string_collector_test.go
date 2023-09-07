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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDistinctStringCollector(t *testing.T) {
	d := NewDistinctStringCollector(10)

	d.Collect("123")
	d.Collect("4567")
	d.Collect("890")
	d.Collect("11")

	require.True(t, d.Exceeded())
	require.Equal(t, []string{"123", "4567", "890"}, d.Strings())
}
