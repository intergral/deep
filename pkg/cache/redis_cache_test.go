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
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kit/log"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestRedisCache(t *testing.T) {
	c, err := mockRedisCache()
	require.Nil(t, err)
	defer c.redis.Close()

	keys := []string{"key1", "key2", "key3"}
	bufs := [][]byte{[]byte("data1"), []byte("data2"), []byte("data3")}
	miss := []string{"miss1", "miss2"}

	// ensure input correctness
	nHit := len(keys)
	require.Len(t, bufs, nHit)

	nMiss := len(miss)

	ctx := context.Background()

	c.Store(ctx, keys, bufs)

	// test hits
	found, data, missed := c.Fetch(ctx, keys)

	require.Len(t, found, nHit)
	require.Len(t, missed, 0)
	for i := 0; i < nHit; i++ {
		require.Equal(t, keys[i], found[i])
		require.Equal(t, bufs[i], data[i])
	}

	// test misses
	found, _, missed = c.Fetch(ctx, miss)

	require.Len(t, found, 0)
	require.Len(t, missed, nMiss)
	for i := 0; i < nMiss; i++ {
		require.Equal(t, miss[i], missed[i])
	}
}

func mockRedisCache() (*RedisCache, error) {
	redisServer, err := miniredis.Run()
	if err != nil {
		return nil, err

	}
	redisClient := &RedisClient{
		expiration: time.Minute,
		timeout:    100 * time.Millisecond,
		rdb: redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs: []string{redisServer.Addr()},
		}),
	}
	return NewRedisCache("mock", redisClient, nil, log.NewNopLogger()), nil
}
