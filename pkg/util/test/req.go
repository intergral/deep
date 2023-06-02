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

package test

import crand "crypto/rand"

//import (
//	crand "crypto/rand"
//	"encoding/json"
//	"github.com/intergral/deep/pkg/tempopb"
//	"math/rand"
//	"testing"
//	"time"
//
//	"github.com/golang/protobuf/proto"
//	v1_common "github.com/intergral/deep/pkg/tempopb/common/v1"
//	v1_resource "github.com/intergral/deep/pkg/tempopb/resource/v1"
//	v1_trace "github.com/intergral/deep/pkg/tempopb/trace/v1"
//	"github.com/stretchr/testify/require"
//)
//
//func MakeSpan(traceID []byte) *v1_trace.Span {
//	return MakeSpanWithAttributeCount(traceID, rand.Int()%10+1)
//}
//
//func MakeSpanWithAttributeCount(traceID []byte, count int) *v1_trace.Span {
//	attributes := make([]*v1_common.KeyValue, 0, count)
//	for i := 0; i < count; i++ {
//		attributes = append(attributes, &v1_common.KeyValue{
//			Key:   RandomString(),
//			Value: &v1_common.AnyValue{Value: &v1_common.AnyValue_StringValue{StringValue: RandomString()}},
//		})
//	}
//
//	now := time.Now()
//	s := &v1_trace.Span{
//		Name:         "test",
//		TraceId:      traceID,
//		SpanId:       make([]byte, 8),
//		ParentSpanId: make([]byte, 8),
//		Kind:         v1_trace.Span_SPAN_KIND_CLIENT,
//		Status: &v1_trace.Status{
//			Code:    1,
//			Message: "OK",
//		},
//		StartTimeUnixNano:      uint64(now.UnixNano()),
//		EndTimeUnixNano:        uint64(now.Add(time.Second).UnixNano()),
//		Attributes:             attributes,
//		DroppedLinksCount:      rand.Uint32(),
//		DroppedAttributesCount: rand.Uint32(),
//	}
//	_, err := crand.Read(s.SpanId)
//	if err != nil {
//		panic(err)
//	}
//
//	// add link
//	if rand.Intn(5) == 0 {
//		s.Links = append(s.Links, &v1_trace.Span_Link{
//			TraceId:    traceID,
//			SpanId:     make([]byte, 8),
//			TraceState: "state",
//			Attributes: []*v1_common.KeyValue{
//				{
//					Key: "linkkey",
//					Value: &v1_common.AnyValue{
//						Value: &v1_common.AnyValue_StringValue{
//							StringValue: "linkvalue",
//						},
//					},
//				},
//			},
//		})
//	}
//
//	// add attr
//	if rand.Intn(2) == 0 {
//		s.Attributes = append(s.Attributes, &v1_common.KeyValue{
//			Key: "key",
//			Value: &v1_common.AnyValue{
//				Value: &v1_common.AnyValue_StringValue{
//					StringValue: "value",
//				},
//			},
//		})
//	}
//
//	// add event
//	if rand.Intn(3) == 0 {
//		s.Events = append(s.Events, &v1_trace.Span_Event{
//			TimeUnixNano:           rand.Uint64(),
//			Name:                   "event",
//			DroppedAttributesCount: rand.Uint32(),
//			Attributes: []*v1_common.KeyValue{
//				{
//					Key: "eventkey",
//					Value: &v1_common.AnyValue{
//						Value: &v1_common.AnyValue_StringValue{
//							StringValue: "eventvalue",
//						},
//					},
//				},
//			},
//		})
//	}
//
//	return s
//}
//
//func MakeBatch(spans int, traceID []byte) *v1_trace.ResourceSpans {
//	traceID = ValidSnapshotID(traceID)
//
//	batch := &v1_trace.ResourceSpans{
//		Resource: &v1_resource.Resource{
//			Attributes: []*v1_common.KeyValue{
//				{
//					Key: "service.name",
//					Value: &v1_common.AnyValue{
//						Value: &v1_common.AnyValue_StringValue{
//							StringValue: "test-service",
//						},
//					},
//				},
//			},
//		},
//	}
//	var ss *v1_trace.ScopeSpans
//
//	for i := 0; i < spans; i++ {
//		// occasionally make a new ss
//		if ss == nil || rand.Int()%3 == 0 {
//			ss = &v1_trace.ScopeSpans{
//				Scope: &v1_common.InstrumentationScope{
//					Name:    "super library",
//					Version: "0.0.1",
//				},
//			}
//
//			batch.ScopeSpans = append(batch.ScopeSpans, ss)
//		}
//
//		ss.Spans = append(ss.Spans, MakeSpan(traceID))
//	}
//	return batch
//}
//
//func MakeTrace(requests int, traceID []byte) *tempopb.Trace {
//	traceID = ValidSnapshotID(traceID)
//
//	trace := &tempopb.Trace{
//		Batches: make([]*v1_trace.ResourceSpans, 0),
//	}
//
//	for i := 0; i < requests; i++ {
//		trace.Batches = append(trace.Batches, MakeBatch(rand.Int()%20+1, traceID))
//	}
//
//	return trace
//}
//
//func MakeTraceBytes(requests int, traceID []byte) *tempopb.TraceBytes {
//	trace := &tempopb.Trace{
//		Batches: make([]*v1_trace.ResourceSpans, 0),
//	}
//
//	for i := 0; i < requests; i++ {
//		trace.Batches = append(trace.Batches, MakeBatch(rand.Int()%20+1, traceID))
//	}
//
//	bytes, err := proto.Marshal(trace)
//	if err != nil {
//		panic(err)
//	}
//
//	traceBytes := &tempopb.TraceBytes{
//		Traces: [][]byte{bytes},
//	}
//
//	return traceBytes
//}
//
//func MakeTraceWithSpanCount(requests int, spansEach int, traceID []byte) *tempopb.Trace {
//	trace := &tempopb.Trace{
//		Batches: make([]*v1_trace.ResourceSpans, 0),
//	}
//
//	for i := 0; i < requests; i++ {
//		trace.Batches = append(trace.Batches, MakeBatch(spansEach, traceID))
//	}
//
//	return trace
//}

func ValidSnapshotID(snapshotID []byte) []byte {
	if len(snapshotID) == 0 {
		snapshotID = make([]byte, 16)
		_, err := crand.Read(snapshotID)
		if err != nil {
			panic(err)
		}
	}

	for len(snapshotID) < 16 {
		snapshotID = append(snapshotID, 0)
	}

	return snapshotID
}

//func RandomString() string {
//	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
//
//	s := make([]rune, 10)
//	for i := range s {
//		s[i] = letters[rand.Intn(len(letters))]
//	}
//	return string(s)
//}
//
//func TracesEqual(t *testing.T, t1 *tempopb.Trace, t2 *tempopb.Trace) {
//	if !proto.Equal(t1, t2) {
//		wantJSON, _ := json.MarshalIndent(t1, "", "  ")
//		gotJSON, _ := json.MarshalIndent(t2, "", "  ")
//
//		require.Equal(t, string(wantJSON), string(gotJSON))
//	}
//}
