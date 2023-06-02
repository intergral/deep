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

package compactor

//
//import (
//	"math"
//	"testing"
//
//	"github.com/intergral/deep/modules/overrides"
//	"github.com/intergral/deep/pkg/model"
//	"github.com/intergral/deep/pkg/model/trace"
//	"github.com/intergral/deep/pkg/tempopb"
//	v1 "github.com/intergral/deep/pkg/tempopb/trace/v1"
//	"github.com/intergral/deep/pkg/util/test"
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/require"
//)
//
//func TestCombineLimitsNotHit(t *testing.T) {
//	o, err := overrides.NewOverrides(overrides.Limits{
//		MaxBytesPerSnapshot: math.MaxInt,
//	})
//	require.NoError(t, err)
//
//	c := &Compactor{
//		overrides: o,
//	}
//
//	trace := test.MakeTraceWithSpanCount(2, 10, nil)
//	t1 := &tempopb.Trace{
//		Batches: []*v1.ResourceSpans{
//			trace.Batches[0],
//		},
//	}
//	t2 := &tempopb.Trace{
//		Batches: []*v1.ResourceSpans{
//			trace.Batches[1],
//		},
//	}
//	obj1 := encode(t, t1)
//	obj2 := encode(t, t2)
//
//	actual, wasCombined, err := c.Combine(model.CurrentEncoding, "test", obj1, obj2)
//	assert.NoError(t, err)
//	assert.Equal(t, true, wasCombined)
//	assert.Equal(t, encode(t, trace), actual) // entire trace should be returned
//}
//
//func TestCombineLimitsHit(t *testing.T) {
//	o, err := overrides.NewOverrides(overrides.Limits{
//		MaxBytesPerSnapshot: 1,
//	})
//	require.NoError(t, err)
//
//	c := &Compactor{
//		overrides: o,
//	}
//
//	trace := test.MakeTraceWithSpanCount(2, 10, nil)
//	t1 := &tempopb.Trace{
//		Batches: []*v1.ResourceSpans{
//			trace.Batches[0],
//		},
//	}
//	t2 := &tempopb.Trace{
//		Batches: []*v1.ResourceSpans{
//			trace.Batches[1],
//		},
//	}
//	obj1 := encode(t, t1)
//	obj2 := encode(t, t2)
//
//	actual, wasCombined, err := c.Combine(model.CurrentEncoding, "test", obj1, obj2)
//	assert.NoError(t, err)
//	assert.Equal(t, true, wasCombined)
//	assert.Equal(t, encode(t, t1), actual) // only t1 was returned b/c the combined trace was greater than the threshold
//}
//
//func TestCombineDoesntEnforceZero(t *testing.T) {
//	o, err := overrides.NewOverrides(overrides.Limits{
//		MaxBytesPerSnapshot: 0,
//	})
//	require.NoError(t, err)
//
//	c := &Compactor{
//		overrides: o,
//	}
//
//	trace := test.MakeTraceWithSpanCount(2, 10, nil)
//	t1 := &tempopb.Trace{
//		Batches: []*v1.ResourceSpans{
//			trace.Batches[0],
//		},
//	}
//	t2 := &tempopb.Trace{
//		Batches: []*v1.ResourceSpans{
//			trace.Batches[1],
//		},
//	}
//	obj1 := encode(t, t1)
//	obj2 := encode(t, t2)
//
//	actual, wasCombined, err := c.Combine(model.CurrentEncoding, "test", obj1, obj2)
//	assert.NoError(t, err)
//	assert.Equal(t, true, wasCombined)
//	assert.Equal(t, encode(t, trace), actual) // entire trace should be returned
//}

//func TestCountSpans(t *testing.T) {
//	t1 := test.MakeTraceWithSpanCount(1, 10, nil)
//	t2 := test.MakeTraceWithSpanCount(2, 13, nil)
//	t1ExpectedSpans := 10
//	t2ExpectedSpans := 26
//
//	b1 := encode(t, t1)
//	b2 := encode(t, t2)
//
//	b1Total := countSpans(model.CurrentEncoding, b1)
//	b2Total := countSpans(model.CurrentEncoding, b2)
//	total := countSpans(model.CurrentEncoding, b1, b2)
//
//	assert.Equal(t, t1ExpectedSpans, b1Total)
//	assert.Equal(t, t2ExpectedSpans, b2Total)
//	assert.Equal(t, t1ExpectedSpans+t2ExpectedSpans, total)
//}
//
//func encode(t *testing.T, tr *tempopb.Trace) []byte {
//	trace.SortTrace(tr)
//
//	sd := model.MustNewSegmentDecoder(model.CurrentEncoding)
//
//	segment, err := sd.PrepareForWrite(tr, 0, 0)
//	require.NoError(t, err)
//
//	obj, err := sd.ToObject([][]byte{segment})
//	require.NoError(t, err)
//
//	return obj
//}
