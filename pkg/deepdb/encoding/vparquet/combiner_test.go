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

package vparquet

//
//import (
//	"github.com/dustin/go-humanize"
//	"testing"
//
//	"github.com/intergral/deep/pkg/util/test"
//	"github.com/stretchr/testify/assert"
//)
//
//func TestCombiner(t *testing.T) {
//
//	methods := []func(a, b *Trace) (*Trace, int){
//		func(a, b *Trace) (*Trace, int) {
//			c := NewCombiner()
//			c.Consume(a)
//			c.Consume(b)
//			return c.Result()
//		},
//	}
//
//	tests := []struct {
//		traceA        *Trace
//		traceB        *Trace
//		expectedTotal int
//		expectedTrace *Trace
//	}{
//		{
//			traceA:        nil,
//			traceB:        &Trace{},
//			expectedTotal: -1,
//		},
//		{
//			traceA:        &Trace{},
//			traceB:        nil,
//			expectedTotal: -1,
//		},
//		{
//			traceA:        &Trace{},
//			traceB:        &Trace{},
//			expectedTotal: 0,
//		},
//		// root meta from second overrides empty first
//		{
//			traceA: &Trace{
//				TraceID: []byte{0x00, 0x01},
//			},
//			traceB: &Trace{
//				TraceID:           []byte{0x00, 0x01},
//				RootServiceName:   "serviceNameB",
//				RootSpanName:      "spanNameB",
//				StartTimeUnixNano: 10,
//				EndTimeUnixNano:   20,
//				DurationNanos:     10,
//			},
//			expectedTrace: &Trace{
//				TraceID:           []byte{0x00, 0x01},
//				RootServiceName:   "serviceNameB",
//				RootSpanName:      "spanNameB",
//				StartTimeUnixNano: 10,
//				EndTimeUnixNano:   20,
//				DurationNanos:     10,
//			},
//		},
//		// if both set first root name wins
//		{
//			traceA: &Trace{
//				TraceID:         []byte{0x00, 0x01},
//				RootServiceName: "serviceNameA",
//				RootSpanName:    "spanNameA",
//			},
//			traceB: &Trace{
//				TraceID:         []byte{0x00, 0x01},
//				RootServiceName: "serviceNameB",
//				RootSpanName:    "spanNameB",
//			},
//			expectedTrace: &Trace{
//				TraceID:         []byte{0x00, 0x01},
//				RootServiceName: "serviceNameA",
//				RootSpanName:    "spanNameA",
//			},
//		},
//		// second trace start/end override
//		{
//			traceA: &Trace{
//				TraceID:           []byte{0x00, 0x01},
//				StartTimeUnixNano: 10,
//				EndTimeUnixNano:   20,
//				DurationNanos:     10,
//			},
//			traceB: &Trace{
//				TraceID:           []byte{0x00, 0x01},
//				StartTimeUnixNano: 5,
//				EndTimeUnixNano:   25,
//				DurationNanos:     20,
//			},
//			expectedTrace: &Trace{
//				TraceID:           []byte{0x00, 0x01},
//				StartTimeUnixNano: 5,
//				EndTimeUnixNano:   25,
//				DurationNanos:     20,
//			},
//		},
//		// second trace start/end ignored
//		{
//			traceA: &Trace{
//				TraceID:           []byte{0x00, 0x01},
//				StartTimeUnixNano: 10,
//				EndTimeUnixNano:   20,
//				DurationNanos:     10,
//			},
//			traceB: &Trace{
//				TraceID:           []byte{0x00, 0x01},
//				StartTimeUnixNano: 12,
//				EndTimeUnixNano:   18,
//				DurationNanos:     6,
//			},
//			expectedTrace: &Trace{
//				TraceID:           []byte{0x00, 0x01},
//				StartTimeUnixNano: 10,
//				EndTimeUnixNano:   20,
//				DurationNanos:     10,
//			},
//		},
//		{
//			traceA: &Trace{
//				TraceID:         []byte{0x00, 0x01},
//				RootServiceName: "serviceNameA",
//				ResourceSpans: []ResourceSpans{
//					{
//						Resource: Resource{
//							ServiceName: "serviceNameA",
//						},
//						ScopeSpans: []ScopeSpan{
//							{
//								Spans: []Span{
//									{
//										ID:         []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
//										StatusCode: 0,
//									},
//								},
//							},
//						},
//					},
//				},
//			},
//			traceB: &Trace{
//				TraceID:         []byte{0x00, 0x01},
//				RootServiceName: "serviceNameB",
//				ResourceSpans: []ResourceSpans{
//					{
//						Resource: Resource{
//							ServiceName: "serviceNameB",
//						},
//						ScopeSpans: []ScopeSpan{
//							{
//								Spans: []Span{
//									{
//										ID:           []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
//										ParentSpanID: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
//										StatusCode:   0,
//									},
//								},
//							},
//						},
//					},
//				},
//			},
//			expectedTotal: 2,
//			expectedTrace: &Trace{
//				TraceID:         []byte{0x00, 0x01},
//				RootServiceName: "serviceNameA",
//				ResourceSpans: []ResourceSpans{
//					{
//						Resource: Resource{
//							ServiceName: "serviceNameA",
//						},
//						ScopeSpans: []ScopeSpan{
//							{
//								Spans: []Span{
//									{
//										ID:         []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
//										StatusCode: 0,
//									},
//								},
//							},
//						},
//					},
//					{
//						Resource: Resource{
//							ServiceName: "serviceNameB",
//						},
//						ScopeSpans: []ScopeSpan{
//							{
//								Spans: []Span{
//									{
//										ID:           []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
//										ParentSpanID: []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
//										StatusCode:   0,
//									},
//								},
//							},
//						},
//					},
//				},
//			},
//		},
//		/*{
//			traceA:        sameTrace,
//			traceB:        sameTrace,
//			expectedTotal: 100,
//		},*/
//	}
//
//	for _, tt := range tests {
//		for _, m := range methods {
//			actualTrace, actualTotal := m(tt.traceA, tt.traceB)
//			assert.Equal(t, tt.expectedTotal, actualTotal)
//			if tt.expectedTrace != nil {
//				assert.Equal(t, tt.expectedTrace, actualTrace)
//			}
//		}
//	}
//}
//
//func BenchmarkCombine(b *testing.B) {
//
//	batchCount := 100
//	spanCounts := []int{
//		100, 1000, 10000,
//	}
//
//	for _, spanCount := range spanCounts {
//		b.Run("SpanCount:"+humanize.SI(float64(batchCount*spanCount), ""), func(b *testing.B) {
//			id1 := test.ValidTraceID(nil)
//			tr1 := traceToParquet(id1, test.MakeTraceWithSpanCount(batchCount, spanCount, id1), nil)
//
//			id2 := test.ValidTraceID(nil)
//			tr2 := traceToParquet(id2, test.MakeTraceWithSpanCount(batchCount, spanCount, id2), nil)
//
//			b.ResetTimer()
//
//			for i := 0; i < b.N; i++ {
//				c := NewCombiner()
//				c.ConsumeWithFinal(tr1, false)
//				c.ConsumeWithFinal(tr2, true)
//				c.Result()
//			}
//		})
//	}
//}
//
//func BenchmarkSortTrace(b *testing.B) {
//
//	batchCount := 100
//	spanCounts := []int{
//		100, 1000, 10000,
//	}
//
//	for _, spanCount := range spanCounts {
//		b.Run("SpanCount:"+humanize.SI(float64(batchCount*spanCount), ""), func(b *testing.B) {
//
//			id := test.ValidTraceID(nil)
//			tr := traceToParquet(id, test.MakeTraceWithSpanCount(batchCount, spanCount, id), nil)
//
//			b.ResetTimer()
//
//			for i := 0; i < b.N; i++ {
//				SortTrace(tr)
//			}
//		})
//	}
//}
