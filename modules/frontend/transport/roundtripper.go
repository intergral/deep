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

package transport

import (
	"bytes"
	"context"
	frontend_v1pb "github.com/intergral/deep/modules/frontend/v1/frontendv1pb"
	"io"
	"io/ioutil"
	"net/http"
)

// GrpcRoundTripper is similar to http.RoundTripper, but works with HTTP requests converted to protobuf messages.
type GrpcRoundTripper func(context.Context, *frontend_v1pb.HTTPRequest) (*frontend_v1pb.HTTPResponse, error)

func AdaptGrpcRoundTripperToHTTPRoundTripper(r GrpcRoundTripper) http.RoundTripper {
	return &grpcRoundTripperAdapter{roundTripper: r}
}

// This adapter wraps GrpcRoundTripper and converted it into http.RoundTripper
type grpcRoundTripperAdapter struct {
	roundTripper func(context.Context, *frontend_v1pb.HTTPRequest) (*frontend_v1pb.HTTPResponse, error)
}

type buffer struct {
	buff []byte
	io.ReadCloser
}

func (b *buffer) Bytes() []byte {
	return b.buff
}

func (a *grpcRoundTripperAdapter) RoundTrip(r *http.Request) (*http.Response, error) {
	req, err := HTTPRequest(r)
	if err != nil {
		return nil, err
	}

	resp, err := a.roundTripper(r.Context(), req)
	if err != nil {
		return nil, err
	}

	httpResp := &http.Response{
		StatusCode:    int(resp.Code),
		Body:          &buffer{buff: resp.Body, ReadCloser: io.NopCloser(bytes.NewReader(resp.Body))},
		Header:        http.Header{},
		ContentLength: int64(len(resp.Body)),
	}
	for _, h := range resp.Headers {
		httpResp.Header[h.Key] = h.Values
	}
	return httpResp, nil
}

// HTTPRequest wraps an ordinary HTTPRequest with a gRPC one
func HTTPRequest(r *http.Request) (*frontend_v1pb.HTTPRequest, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return &frontend_v1pb.HTTPRequest{
		Method:  r.Method,
		Url:     r.RequestURI,
		Body:    body,
		Headers: fromHeader(r.Header),
	}, nil
}

func fromHeader(hs http.Header) []*frontend_v1pb.Header {
	result := make([]*frontend_v1pb.Header, 0, len(hs))
	for k, vs := range hs {
		result = append(result, &frontend_v1pb.Header{
			Key:    k,
			Values: vs,
		})
	}
	return result
}
