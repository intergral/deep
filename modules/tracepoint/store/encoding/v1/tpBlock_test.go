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

package v1

import (
	cp "github.com/intergral/deep/pkg/deeppb/common/v1"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestTargeting(t *testing.T) {

	exampleResource := []*cp.KeyValue{
		{
			Key:   "service.name",
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "test"}},
		},
		{
			Key:   "sdk.language",
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "python"}},
		},
		{
			Key:   "os.name",
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "linux"}},
		},
	}

	tests := []struct {
		name      string
		expected  bool
		targeting []*cp.KeyValue
		resource  []*cp.KeyValue
	}{
		{
			name:      "Match All",
			expected:  true,
			targeting: nil,
			resource:  exampleResource,
		},
		{
			name:     "Match None",
			expected: false,
			targeting: []*cp.KeyValue{{
				Key:   "os.name",
				Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "windows"}},
			}},
			resource: exampleResource,
		},
		{
			name:     "Match single",
			expected: true,
			targeting: []*cp.KeyValue{{
				Key:   "os.name",
				Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "linux"}},
			}},
			resource: exampleResource,
		},
		{
			name:     "Match many",
			expected: true,
			targeting: []*cp.KeyValue{
				{
					Key:   "os.name",
					Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "linux"}},
				}, {
					Key:   "sdk.language",
					Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "python"}},
				},
			},
			resource: exampleResource,
		},
		{
			name:     "Not match one",
			expected: false,
			targeting: []*cp.KeyValue{
				{
					Key:   "os.name",
					Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "windows"}},
				}, {
					Key:   "sdk.language",
					Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "python"}},
				},
			},
			resource: exampleResource,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			tp := &deeptp.TracePointConfig{
				ID:        "one",
				Targeting: test.targeting,
			}
			block := &tpBlock{}

			matches := block.matches(tp, test.resource)

			assert.Equal(t, test.expected, matches)
		})
	}
}
