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

package spanlogger

import (
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/log"
)

type noopTracer struct{}

type noopSpan struct{}
type noopSpanContext struct{}

var (
	defaultNoopSpanContext = noopSpanContext{}
	defaultNoopSpan        = noopSpan{}
	defaultNoopTracer      = noopTracer{}
)

const (
	emptyString = ""
)

func (n noopSpanContext) ForeachBaggageItem(handler func(k, v string) bool) {}

func (n noopSpan) Context() opentracing.SpanContext                       { return defaultNoopSpanContext }
func (n noopSpan) SetBaggageItem(key, val string) opentracing.Span        { return defaultNoopSpan }
func (n noopSpan) BaggageItem(key string) string                          { return emptyString }
func (n noopSpan) SetTag(key string, value interface{}) opentracing.Span  { return n }
func (n noopSpan) LogFields(fields ...log.Field)                          {}
func (n noopSpan) LogKV(keyVals ...interface{})                           {}
func (n noopSpan) Finish()                                                {}
func (n noopSpan) FinishWithOptions(opts opentracing.FinishOptions)       {}
func (n noopSpan) SetOperationName(operationName string) opentracing.Span { return n }
func (n noopSpan) Tracer() opentracing.Tracer                             { return defaultNoopTracer }
func (n noopSpan) LogEvent(event string)                                  {}
func (n noopSpan) LogEventWithPayload(event string, payload interface{})  {}
func (n noopSpan) Log(data opentracing.LogData)                           {}

// StartSpan belongs to the Tracer interface.
func (n noopTracer) StartSpan(operationName string, opts ...opentracing.StartSpanOption) opentracing.Span {
	return defaultNoopSpan
}

// Inject belongs to the Tracer interface.
func (n noopTracer) Inject(sp opentracing.SpanContext, format interface{}, carrier interface{}) error {
	return nil
}

// Extract belongs to the Tracer interface.
func (n noopTracer) Extract(format interface{}, carrier interface{}) (opentracing.SpanContext, error) {
	return nil, opentracing.ErrSpanContextNotFound
}
