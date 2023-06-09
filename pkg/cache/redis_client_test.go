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
	"strings"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestRedisClient(t *testing.T) {
	single, err := mockRedisClientSingle()
	require.Nil(t, err)
	defer single.Close()

	cluster, err := mockRedisClientCluster()
	require.Nil(t, err)
	defer cluster.Close()

	ctx := context.Background()

	tests := []struct {
		name   string
		client *RedisClient
	}{
		{
			name:   "single redis client",
			client: single,
		},
		{
			name:   "cluster redis client",
			client: cluster,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := []string{"key1", "key2", "key3"}
			bufs := [][]byte{[]byte("data1"), []byte("data2"), []byte("data3")}
			miss := []string{"miss1", "miss2"}

			// set values
			err := tt.client.MSet(ctx, keys, bufs)
			require.Nil(t, err)

			// get keys
			values, err := tt.client.MGet(ctx, keys)
			require.Nil(t, err)
			require.Len(t, values, len(keys))
			for i, value := range values {
				require.Equal(t, values[i], value)
			}

			// get missing keys
			values, err = tt.client.MGet(ctx, miss)
			require.Nil(t, err)
			require.Len(t, values, len(miss))
			for _, value := range values {
				require.Nil(t, value)
			}
		})
	}
}

func mockRedisClientSingle() (*RedisClient, error) {
	redisServer, err := miniredis.Run()
	if err != nil {
		return nil, err
	}

	cfg := &RedisConfig{
		Expiration: time.Minute,
		Timeout:    100 * time.Millisecond,
		Endpoint: strings.Join([]string{
			redisServer.Addr(),
		}, ","),
	}

	return NewRedisClient(cfg), nil
}

func mockRedisClientCluster() (*RedisClient, error) {
	redisServer1, err := miniredis.Run()
	if err != nil {
		return nil, err
	}

	redisServer2, err := miniredis.Run()
	if err != nil {
		return nil, err
	}

	cfg := &RedisConfig{
		Expiration: time.Minute,
		Timeout:    100 * time.Millisecond,
		Endpoint: strings.Join([]string{
			redisServer1.Addr(),
			redisServer2.Addr(),
		}, ","),
	}

	return NewRedisClient(cfg), nil
}
