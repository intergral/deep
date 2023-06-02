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

package cache

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/cache"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/stretchr/testify/assert"
)

type mockClient struct {
	client map[string][]byte
}

func (m *mockClient) Store(_ context.Context, key []string, val [][]byte) {
	m.client[key[0]] = val[0]
}

func (m *mockClient) Fetch(_ context.Context, key []string) (found []string, bufs [][]byte, missing []string) {
	val, ok := m.client[key[0]]
	if ok {
		found = append(found, key[0])
		bufs = append(bufs, val)
	} else {
		missing = append(missing, key[0])
	}
	return
}

func (m *mockClient) Stop() {
}

// NewMockClient makes a new mockClient.
func NewMockClient() cache.Cache {
	return &mockClient{
		client: map[string][]byte{},
	}
}
func TestReadWrite(t *testing.T) {
	tenantID := "test"
	blockID := uuid.New()

	tests := []struct {
		name          string
		readerRead    []byte
		readerName    string
		shouldCache   bool
		expectedRead  []byte
		expectedCache []byte
	}{
		{
			name:          "should cache",
			readerName:    "foo",
			readerRead:    []byte{0x02},
			shouldCache:   true,
			expectedRead:  []byte{0x02},
			expectedCache: []byte{0x02},
		},
		{
			name:         "should not cache",
			readerName:   "bar",
			shouldCache:  false,
			readerRead:   []byte{0x02},
			expectedRead: []byte{0x02},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockR := &backend.MockRawReader{
				R: tt.readerRead,
			}
			mockW := &backend.MockRawWriter{}

			// READ
			r, _, _ := NewCache(mockR, mockW, NewMockClient())

			ctx := context.Background()
			reader, _, _ := r.Read(ctx, tt.readerName, backend.KeyPathForBlock(blockID, tenantID), tt.shouldCache)
			read, _ := io.ReadAll(reader)
			assert.Equal(t, tt.expectedRead, read)

			// clear reader and re-request
			mockR.R = nil

			reader, _, _ = r.Read(ctx, tt.readerName, backend.KeyPathForBlock(blockID, tenantID), tt.shouldCache)
			read, _ = io.ReadAll(reader)
			assert.Equal(t, len(tt.expectedCache), len(read))

			// WRITE
			_, w, _ := NewCache(mockR, mockW, NewMockClient())
			_ = w.Write(ctx, tt.readerName, backend.KeyPathForBlock(blockID, tenantID), bytes.NewReader(tt.readerRead), int64(len(tt.readerRead)), tt.shouldCache)
			reader, _, _ = r.Read(ctx, tt.readerName, backend.KeyPathForBlock(blockID, tenantID), tt.shouldCache)
			read, _ = io.ReadAll(reader)
			assert.Equal(t, len(tt.expectedCache), len(read))
		})
	}
}

func TestList(t *testing.T) {
	tenantID := "test"
	blockID := uuid.New()

	tests := []struct {
		name          string
		readerList    []string
		expectedList  []string
		expectedCache []string
	}{
		{
			name:          "list passthrough",
			readerList:    []string{"1"},
			expectedList:  []string{"1"},
			expectedCache: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockR := &backend.MockRawReader{
				L: tt.readerList,
			}
			mockW := &backend.MockRawWriter{}

			rw, _, _ := NewCache(mockR, mockW, NewMockClient())

			ctx := context.Background()
			list, _ := rw.List(ctx, backend.KeyPathForBlock(blockID, tenantID))
			assert.Equal(t, tt.expectedList, list)

			// clear reader and re-request.  things should be cached!
			mockR.L = nil

			// list is not cached
			list, _ = rw.List(ctx, backend.KeyPathForBlock(blockID, tenantID))
			assert.Equal(t, tt.expectedCache, list)
		})
	}
}
