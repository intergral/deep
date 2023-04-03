package trace

import (
	"testing"

	"github.com/intergral/deep/pkg/tempopb"
	v1 "github.com/intergral/deep/pkg/tempopb/trace/v1"
	"github.com/stretchr/testify/assert"
)

func TestSortTrace(t *testing.T) {
	tests := []struct {
		input    *tempopb.Trace
		expected *tempopb.Trace
	}{
		{
			input:    &tempopb.Trace{},
			expected: &tempopb.Trace{},
		},

		{
			input: &tempopb.Trace{
				Batches: []*v1.ResourceSpans{
					{
						ScopeSpans: []*v1.ScopeSpans{
							{
								Spans: []*v1.Span{
									{
										StartTimeUnixNano: 2,
									},
								},
							},
						},
					},
					{
						ScopeSpans: []*v1.ScopeSpans{
							{
								Spans: []*v1.Span{
									{
										StartTimeUnixNano: 1,
									},
								},
							},
						},
					},
				},
			},
			expected: &tempopb.Trace{
				Batches: []*v1.ResourceSpans{
					{
						ScopeSpans: []*v1.ScopeSpans{
							{
								Spans: []*v1.Span{
									{
										StartTimeUnixNano: 1,
									},
								},
							},
						},
					},
					{
						ScopeSpans: []*v1.ScopeSpans{
							{
								Spans: []*v1.Span{
									{
										StartTimeUnixNano: 2,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		SortTrace(tt.input)

		assert.Equal(t, tt.expected, tt.input)
	}
}
