package compactor

import (
	"math"
	"testing"

	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/pkg/deeppb"
	v1 "github.com/intergral/deep/pkg/deeppb/trace/v1"
	"github.com/intergral/deep/pkg/model"
	"github.com/intergral/deep/pkg/model/trace"
	"github.com/intergral/deep/pkg/util/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombineLimitsNotHit(t *testing.T) {
	o, err := overrides.NewOverrides(overrides.Limits{
		MaxBytesPerTrace: math.MaxInt,
	})
	require.NoError(t, err)

	c := &Compactor{
		overrides: o,
	}

	trace := test.MakeTraceWithSpanCount(2, 10, nil)
	t1 := &deeppb.Trace{
		Batches: []*v1.ResourceSpans{
			trace.Batches[0],
		},
	}
	t2 := &deeppb.Trace{
		Batches: []*v1.ResourceSpans{
			trace.Batches[1],
		},
	}
	obj1 := encode(t, t1)
	obj2 := encode(t, t2)

	actual, wasCombined, err := c.Combine(model.CurrentEncoding, "test", obj1, obj2)
	assert.NoError(t, err)
	assert.Equal(t, true, wasCombined)
	assert.Equal(t, encode(t, trace), actual) // entire trace should be returned
}

func TestCombineLimitsHit(t *testing.T) {
	o, err := overrides.NewOverrides(overrides.Limits{
		MaxBytesPerTrace: 1,
	})
	require.NoError(t, err)

	c := &Compactor{
		overrides: o,
	}

	trace := test.MakeTraceWithSpanCount(2, 10, nil)
	t1 := &deeppb.Trace{
		Batches: []*v1.ResourceSpans{
			trace.Batches[0],
		},
	}
	t2 := &deeppb.Trace{
		Batches: []*v1.ResourceSpans{
			trace.Batches[1],
		},
	}
	obj1 := encode(t, t1)
	obj2 := encode(t, t2)

	actual, wasCombined, err := c.Combine(model.CurrentEncoding, "test", obj1, obj2)
	assert.NoError(t, err)
	assert.Equal(t, true, wasCombined)
	assert.Equal(t, encode(t, t1), actual) // only t1 was returned b/c the combined trace was greater than the threshold
}

func TestCombineDoesntEnforceZero(t *testing.T) {
	o, err := overrides.NewOverrides(overrides.Limits{
		MaxBytesPerTrace: 0,
	})
	require.NoError(t, err)

	c := &Compactor{
		overrides: o,
	}

	trace := test.MakeTraceWithSpanCount(2, 10, nil)
	t1 := &deeppb.Trace{
		Batches: []*v1.ResourceSpans{
			trace.Batches[0],
		},
	}
	t2 := &deeppb.Trace{
		Batches: []*v1.ResourceSpans{
			trace.Batches[1],
		},
	}
	obj1 := encode(t, t1)
	obj2 := encode(t, t2)

	actual, wasCombined, err := c.Combine(model.CurrentEncoding, "test", obj1, obj2)
	assert.NoError(t, err)
	assert.Equal(t, true, wasCombined)
	assert.Equal(t, encode(t, trace), actual) // entire trace should be returned
}

func TestCountSpans(t *testing.T) {
	t1 := test.MakeTraceWithSpanCount(1, 10, nil)
	t2 := test.MakeTraceWithSpanCount(2, 13, nil)
	t1ExpectedSpans := 10
	t2ExpectedSpans := 26

	b1 := encode(t, t1)
	b2 := encode(t, t2)

	b1Total := countSpans(model.CurrentEncoding, b1)
	b2Total := countSpans(model.CurrentEncoding, b2)
	total := countSpans(model.CurrentEncoding, b1, b2)

	assert.Equal(t, t1ExpectedSpans, b1Total)
	assert.Equal(t, t2ExpectedSpans, b2Total)
	assert.Equal(t, t1ExpectedSpans+t2ExpectedSpans, total)
}

func encode(t *testing.T, tr *deeppb.Trace) []byte {
	trace.SortTrace(tr)

	sd := model.MustNewSegmentDecoder(model.CurrentEncoding)

	segment, err := sd.PrepareForWrite(tr, 0, 0)
	require.NoError(t, err)

	obj, err := sd.ToObject([][]byte{segment})
	require.NoError(t, err)

	return obj
}
