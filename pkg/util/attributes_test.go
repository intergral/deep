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

	v1common "github.com/intergral/deep/pkg/deeppb/common/v1"
	"github.com/stretchr/testify/assert"
)

func TestStringifyAnyValue(t *testing.T) {
	testCases := []struct {
		name     string
		v        *v1common.AnyValue
		expected string
	}{
		{
			name: "string value",
			v: &v1common.AnyValue{
				Value: &v1common.AnyValue_StringValue{
					StringValue: "test",
				},
			},
			expected: "test",
		},
		{
			name: "int value",
			v: &v1common.AnyValue{
				Value: &v1common.AnyValue_IntValue{
					IntValue: 1,
				},
			},
			expected: "1",
		},
		{
			name: "bool value",
			v: &v1common.AnyValue{
				Value: &v1common.AnyValue_BoolValue{
					BoolValue: true,
				},
			},
			expected: "true",
		},
		{
			name: "float value",
			v: &v1common.AnyValue{
				Value: &v1common.AnyValue_DoubleValue{
					DoubleValue: 1.1,
				},
			},
			expected: "1.1",
		},
		{
			name: "array value",
			v: &v1common.AnyValue{
				Value: &v1common.AnyValue_ArrayValue{
					ArrayValue: &v1common.ArrayValue{
						Values: []*v1common.AnyValue{
							{
								Value: &v1common.AnyValue_StringValue{
									StringValue: "test",
								},
							},
						},
					},
				},
			},
			expected: "[test]",
		},
		{
			name: "map value",
			v: &v1common.AnyValue{
				Value: &v1common.AnyValue_KvlistValue{
					KvlistValue: &v1common.KeyValueList{
						Values: []*v1common.KeyValue{
							{
								Key:   "key",
								Value: &v1common.AnyValue{Value: &v1common.AnyValue_StringValue{StringValue: "value"}},
							},
						},
					},
				},
			},
			expected: "{key:value}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			str := StringifyAnyValue(tc.v)
			assert.Equal(t, tc.expected, str)
		})
	}
}
