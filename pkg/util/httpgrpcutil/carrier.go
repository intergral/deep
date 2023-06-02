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

package httpgrpcutil

import (
	"github.com/opentracing/opentracing-go"
	"github.com/weaveworks/common/httpgrpc"
)

// Used to transfer trace information from/to HTTP request.
type HttpgrpcHeadersCarrier httpgrpc.HTTPRequest

func (c *HttpgrpcHeadersCarrier) Set(key, val string) {
	c.Headers = append(c.Headers, &httpgrpc.Header{
		Key:    key,
		Values: []string{val},
	})
}

func (c *HttpgrpcHeadersCarrier) ForeachKey(handler func(key, val string) error) error {
	for _, h := range c.Headers {
		for _, v := range h.Values {
			if err := handler(h.Key, v); err != nil {
				return err
			}
		}
	}
	return nil
}

func GetParentSpanForRequest(tracer opentracing.Tracer, req *httpgrpc.HTTPRequest) (opentracing.SpanContext, error) {
	if tracer == nil {
		return nil, nil
	}

	carrier := (*HttpgrpcHeadersCarrier)(req)
	extracted, err := tracer.Extract(opentracing.HTTPHeaders, carrier)
	if err == opentracing.ErrSpanContextNotFound {
		err = nil
	}
	return extracted, err
}
