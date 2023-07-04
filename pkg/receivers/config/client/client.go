// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"net"
	"strings"
)

type ctxKey struct{}

// Info contains data related to the clients connecting to receivers.
type Info struct {
	// Addr for the client connecting to this collector. Available in a
	// best-effort basis, and generally reliable for receivers making use of
	// confighttp.ToServer and configgrpc.ToServerOption.
	Addr net.Addr

	// Auth information from the incoming request as provided by
	// configauth.ServerAuthenticator implementations tied to the receiver for
	// this connection.
	Auth AuthData

	// Metadata is the request metadata from the client connecting to this connector.
	// Experimental: *NOTE* this structure is subject to change or removal in the future.
	Metadata Metadata
}

// Metadata is an immutable map, meant to contain request metadata.
type Metadata struct {
	data map[string][]string
}

// AuthData represents the authentication data as seen by authenticators tied to
// the receivers.
type AuthData interface {
	// GetAttribute returns the value for the given attribute. Authenticator
	// implementations might define different data types for different
	// attributes. While "string" is used most of the time, a key named
	// "membership" might return a list of strings.
	GetAttribute(string) interface{}

	// GetAttributes returns the names of all attributes in this authentication
	// data.
	GetAttributeNames() []string
}

const MetadataHostName = "Host"

// NewContext takes an existing context and derives a new context with the
// client.Info value stored on it.
func NewContext(ctx context.Context, c Info) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// FromContext takes a context and returns a ClientInfo from it.
// When a ClientInfo isn't present, a new empty one is returned.
func FromContext(ctx context.Context) Info {
	c, ok := ctx.Value(ctxKey{}).(Info)
	if !ok {
		c = Info{}
	}
	return c
}

// NewMetadata creates a new Metadata object to use in Info. md is used as-is.
func NewMetadata(md map[string][]string) Metadata {
	return Metadata{
		data: md,
	}
}

// Get gets the value of the key from metadata, returning a copy.
func (m Metadata) Get(key string) []string {
	vals := m.data[key]
	if len(vals) == 0 {
		// we didn't find the key, but perhaps it just has different cases?
		for k, v := range m.data {
			if strings.EqualFold(key, k) {
				vals = v
				// we optimize for the next lookup
				m.data[key] = v
			}
		}

		// if it's still not found, it's really not here
		if len(vals) == 0 {
			return nil
		}
	}

	ret := make([]string, len(vals))
	copy(ret, vals)

	return ret
}
