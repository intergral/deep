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

	tp := &deeptp.TracePointConfig{
		ID:        "one",
		Targeting: nil, // to targeting matches all
	}

	block := &tpBlock{}

	matches := block.matches(tp, []*cp.KeyValue{})

	assert.True(t, matches)
}

func TestTargeting_1(t *testing.T) {

	tp := &deeptp.TracePointConfig{
		ID:        "one",
		Targeting: nil, // to targeting matches all
	}

	block := &tpBlock{}

	matches := block.matches(tp, []*cp.KeyValue{{
		Key:   "service.name",
		Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "test"}},
	}})

	assert.True(t, matches)
}

func TestTargeting_2(t *testing.T) {

	tp := &deeptp.TracePointConfig{
		ID: "one",
		Targeting: []*cp.KeyValue{
			{Key: "service.name", Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "other"}}},
		},
	}

	block := &tpBlock{}

	matches := block.matches(tp, []*cp.KeyValue{{
		Key:   "service.name",
		Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "test"}},
	}})

	assert.False(t, matches)
}

func TestTargeting_3(t *testing.T) {

	tp := &deeptp.TracePointConfig{
		ID: "one",
		Targeting: []*cp.KeyValue{
			{Key: "service.name", Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "test"}}},
		},
	}

	block := &tpBlock{}

	matches := block.matches(tp, []*cp.KeyValue{{
		Key:   "service.name",
		Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "test"}},
	}})

	assert.True(t, matches)
}

func TestTargeting_4(t *testing.T) {

	tp := &deeptp.TracePointConfig{
		ID: "one",
		Targeting: []*cp.KeyValue{
			{Key: "service.name", Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "test"}}},
		},
	}

	block := &tpBlock{}

	matches := block.matches(tp, []*cp.KeyValue{{
		Key:   "service.name",
		Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "test"}},
	},
		{
			Key:   "some.tag",
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "value"}},
		},
	},
	)

	assert.True(t, matches)
}
