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

package cache_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/intergral/deep/pkg/cache"
)

func TestMemcached(t *testing.T) {
	t.Run("unbatched", func(t *testing.T) {
		client := newMockMemcache()
		memcache := cache.NewMemcached(cache.MemcachedConfig{}, client,
			"test", nil, log.NewNopLogger())

		testMemcache(t, memcache)
	})

	t.Run("batched", func(t *testing.T) {
		client := newMockMemcache()
		memcache := cache.NewMemcached(cache.MemcachedConfig{
			BatchSize:   10,
			Parallelism: 5,
		}, client, "test", nil, log.NewNopLogger())

		testMemcache(t, memcache)
	})
}

func testMemcache(t *testing.T, memcache *cache.Memcached) {
	numKeys := 1000

	ctx := context.Background()
	keysIncMissing := make([]string, 0, numKeys)
	keys := make([]string, 0, numKeys)
	bufs := make([][]byte, 0, numKeys)

	// Insert 1000 keys skipping all multiples of 5.
	for i := 0; i < numKeys; i++ {
		keysIncMissing = append(keysIncMissing, fmt.Sprint(i))
		if i%5 == 0 {
			continue
		}

		keys = append(keys, fmt.Sprint(i))
		bufs = append(bufs, []byte(fmt.Sprint(i)))
	}
	memcache.Store(ctx, keys, bufs)

	found, bufs, missing := memcache.Fetch(ctx, keysIncMissing)
	for i := 0; i < numKeys; i++ {
		if i%5 == 0 {
			require.Equal(t, fmt.Sprint(i), missing[0])
			missing = missing[1:]
			continue
		}

		require.Equal(t, fmt.Sprint(i), found[0])
		require.Equal(t, fmt.Sprint(i), string(bufs[0]))
		found = found[1:]
		bufs = bufs[1:]
	}
}

// mockMemcache whose calls fail 1/3rd of the time.
type mockMemcacheFailing struct {
	*mockMemcache
	calls atomic.Uint64
}

func newMockMemcacheFailing() *mockMemcacheFailing {
	return &mockMemcacheFailing{
		mockMemcache: newMockMemcache(),
	}
}

func (c *mockMemcacheFailing) GetMulti(keys []string) (map[string]*memcache.Item, error) {
	calls := c.calls.Inc()
	if calls%3 == 0 {
		return nil, errors.New("fail")
	}

	return c.mockMemcache.GetMulti(keys)
}

func TestMemcacheFailure(t *testing.T) {
	t.Run("unbatched", func(t *testing.T) {
		client := newMockMemcacheFailing()
		memcache := cache.NewMemcached(cache.MemcachedConfig{}, client,
			"test", nil, log.NewNopLogger())

		testMemcacheFailing(t, memcache)
	})

	t.Run("batched", func(t *testing.T) {
		client := newMockMemcacheFailing()
		memcache := cache.NewMemcached(cache.MemcachedConfig{
			BatchSize:   10,
			Parallelism: 5,
		}, client, "test", nil, log.NewNopLogger())

		testMemcacheFailing(t, memcache)
	})
}

func testMemcacheFailing(t *testing.T, memcache *cache.Memcached) {
	numKeys := 1000

	ctx := context.Background()
	keysIncMissing := make([]string, 0, numKeys)
	keys := make([]string, 0, numKeys)
	bufs := make([][]byte, 0, numKeys)
	// Insert 1000 keys skipping all multiples of 5.
	for i := 0; i < numKeys; i++ {
		keysIncMissing = append(keysIncMissing, fmt.Sprint(i))
		if i%5 == 0 {
			continue
		}
		keys = append(keys, fmt.Sprint(i))
		bufs = append(bufs, []byte(fmt.Sprint(i)))
	}
	memcache.Store(ctx, keys, bufs)

	for i := 0; i < 10; i++ {
		found, bufs, missing := memcache.Fetch(ctx, keysIncMissing)

		require.Equal(t, len(found), len(bufs))
		for i := range found {
			require.Equal(t, found[i], string(bufs[i]))
		}

		keysReturned := make(map[string]struct{})
		for _, key := range found {
			_, ok := keysReturned[key]
			require.False(t, ok, "duplicate key returned")

			keysReturned[key] = struct{}{}
		}
		for _, key := range missing {
			_, ok := keysReturned[key]
			require.False(t, ok, "duplicate key returned")

			keysReturned[key] = struct{}{}
		}

		for _, key := range keys {
			_, ok := keysReturned[key]
			require.True(t, ok, "key missing %s", key)
		}
	}
}

func TestMemcacheStop(t *testing.T) {
	t.Run("unbatched", func(_ *testing.T) {
		client := newMockMemcacheFailing()
		memcache := cache.NewMemcached(cache.MemcachedConfig{}, client,
			"test", nil, log.NewNopLogger())

		testMemcachedStopping(memcache)
	})

	t.Run("batched", func(_ *testing.T) {
		client := newMockMemcacheFailing()
		memcache := cache.NewMemcached(cache.MemcachedConfig{
			BatchSize:   10,
			Parallelism: 5,
		}, client, "test", nil, log.NewNopLogger())

		testMemcachedStopping(memcache)
	})
}

func testMemcachedStopping(memcache *cache.Memcached) {
	numKeys := 1000
	ctx := context.Background()
	keys := make([]string, 0, numKeys)
	bufs := make([][]byte, 0, numKeys)
	for i := 0; i < numKeys; i++ {
		keys = append(keys, fmt.Sprint(i))
		bufs = append(bufs, []byte(fmt.Sprint(i)))
	}

	memcache.Store(ctx, keys, bufs)

	go memcache.Fetch(ctx, keys)
	memcache.Stop()
}
