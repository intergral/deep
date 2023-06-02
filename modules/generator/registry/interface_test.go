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

package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_newLabelValuesWithMax(t *testing.T) {
	labelValues := newLabelValuesWithMax([]string{"abc", "abcdef"}, 5)

	assert.Equal(t, []string{"abc", "abcde"}, labelValues.getValues())
}

func Test_newLabelValuesWithMax_zeroLength(t *testing.T) {
	labelValues := newLabelValuesWithMax([]string{"abc", "abcdef"}, 0)

	assert.Equal(t, []string{"abc", "abcdef"}, labelValues.getValues())
}
