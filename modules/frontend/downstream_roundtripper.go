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

package frontend

import (
	"net/http"
	"net/url"
	"path"

	"github.com/opentracing/opentracing-go"
)

// RoundTripper that forwards requests to downstream URL.
type downstreamRoundTripper struct {
	downstreamURL *url.URL
	transport     http.RoundTripper
}

func NewDownstreamRoundTripper(downstreamURL string, transport http.RoundTripper) (http.RoundTripper, error) {
	u, err := url.Parse(downstreamURL)
	if err != nil {
		return nil, err
	}

	return &downstreamRoundTripper{downstreamURL: u, transport: transport}, nil
}

func (d downstreamRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	tracer, span := opentracing.GlobalTracer(), opentracing.SpanFromContext(r.Context())
	if tracer != nil && span != nil {
		carrier := make(opentracing.TextMapCarrier, len(r.Header))
		for k, v := range r.Header {
			carrier.Set(k, v[0])
		}
		err := tracer.Inject(span.Context(), opentracing.TextMap, carrier)
		if err != nil {
			return nil, err
		}
	}

	r.URL.Scheme = d.downstreamURL.Scheme
	r.URL.Host = d.downstreamURL.Host
	r.URL.Path = path.Join(d.downstreamURL.Path, r.URL.Path)
	r.Host = ""
	return d.transport.RoundTrip(r)
}
