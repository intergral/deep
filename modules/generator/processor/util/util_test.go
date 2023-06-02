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

	"github.com/stretchr/testify/assert"

	v1_common "github.com/intergral/deep/pkg/deeppb/common/v1"
)

func TestFindServiceName(t *testing.T) {
	testCases := []struct {
		name                string
		attributes          []*v1_common.KeyValue
		expectedServiceName string
		expectedOk          bool
	}{
		{
			"empty attributes",
			nil,
			"",
			false,
		},
		{
			"service name",
			[]*v1_common.KeyValue{
				{
					Key: "cluster",
					Value: &v1_common.AnyValue{
						Value: &v1_common.AnyValue_StringValue{
							StringValue: "test",
						},
					},
				},
				{
					Key: "service.name",
					Value: &v1_common.AnyValue{
						Value: &v1_common.AnyValue_StringValue{
							StringValue: "my-service",
						},
					},
				},
			},
			"my-service",
			true,
		},
		{
			"service name",
			[]*v1_common.KeyValue{
				{
					Key: "service.name",
					Value: &v1_common.AnyValue{
						Value: &v1_common.AnyValue_StringValue{
							StringValue: "",
						},
					},
				},
			},
			"",
			true,
		},
		{
			"no service name",
			[]*v1_common.KeyValue{
				{
					Key: "cluster",
					Value: &v1_common.AnyValue{
						Value: &v1_common.AnyValue_StringValue{
							StringValue: "test",
						},
					},
				},
			},
			"",
			false,
		},
		{
			"service name is other type",
			[]*v1_common.KeyValue{
				{
					Key: "service.name",
					Value: &v1_common.AnyValue{
						Value: &v1_common.AnyValue_BoolValue{
							BoolValue: false,
						},
					},
				},
			},
			"false",
			true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svcName, ok := FindServiceName(tc.attributes)

			assert.Equal(t, tc.expectedOk, ok)
			assert.Equal(t, tc.expectedServiceName, svcName)
		})
	}
}
