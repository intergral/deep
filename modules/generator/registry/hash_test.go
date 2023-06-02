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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_hashLabelValues(t *testing.T) {
	testCases := []struct {
		v1, v2 []string
	}{
		{[]string{"foo"}, []string{"bar"}},
		{[]string{"foo", "bar"}, []string{"foob", "ar"}},
		{[]string{"foo", "bar"}, []string{"foobar", ""}},
		{[]string{"foo", "bar"}, []string{"foo\nbar", ""}},
		{[]string{"foo_", "bar"}, []string{"foo", "_bar"}},
		{[]string{"123", "456"}, []string{"1234", "56"}},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("%s - %s", strings.Join(testCase.v1, ","), strings.Join(testCase.v2, ",")), func(t *testing.T) {
			assert.NotEqual(t, hashLabelValues(testCase.v1), hashLabelValues(testCase.v2))
		})
	}
}
