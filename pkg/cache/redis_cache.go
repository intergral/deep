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

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	instr "github.com/weaveworks/common/instrument"

	util_log "github.com/intergral/deep/pkg/util/log"
	"github.com/intergral/deep/pkg/util/spanlogger"
)

// RedisCache type caches chunks in redis
type RedisCache struct {
	name            string
	redis           *RedisClient
	logger          log.Logger
	requestDuration *instr.HistogramCollector
}

// NewRedisCache creates a new RedisCache
func NewRedisCache(name string, redisClient *RedisClient, reg prometheus.Registerer, logger log.Logger) *RedisCache {
	util_log.WarnExperimentalUse("Redis cache")
	cache := &RedisCache{
		name:   name,
		redis:  redisClient,
		logger: logger,
		requestDuration: instr.NewHistogramCollector(
			promauto.With(reg).NewHistogramVec(prometheus.HistogramOpts{
				Namespace:   "deep",
				Subsystem:   "cache",
				Name:        "redis_request_duration_seconds",
				Help:        "Total time spent in seconds doing Redis requests.",
				Buckets:     prometheus.ExponentialBuckets(0.000016, 4, 8),
				ConstLabels: prometheus.Labels{"name": name},
			}, []string{"method", "status_code"}),
		),
	}
	if err := cache.redis.Ping(context.Background()); err != nil {
		level.Error(logger).Log("msg", "error connecting to redis", "name", name, "err", err)
	}
	return cache
}

func redisStatusCode(err error) string {
	// TODO: Figure out if there are more error types returned by Redis
	switch err {
	case nil:
		return "200"
	default:
		return "500"
	}
}

// Fetch gets keys from the cache. The keys that are found must be in the order of the keys requested.
func (c *RedisCache) Fetch(ctx context.Context, keys []string) (found []string, bufs [][]byte, missed []string) {
	const method = "RedisCache.MGet"
	var items [][]byte
	// Run a tracked request, using c.requestDuration to monitor requests.
	err := instr.CollectedRequest(ctx, method, c.requestDuration, redisStatusCode, func(ctx context.Context) error {
		spanLog, _ := spanlogger.New(ctx, method)
		defer spanLog.Finish()
		spanLog.LogFields(otlog.Int("keys requested", len(keys)))

		var err error
		items, err = c.redis.MGet(ctx, keys)
		if err != nil {
			_ = spanLog.Error(err)
			level.Error(c.logger).Log("msg", "failed to get from redis", "name", c.name, "err", err)
			return err
		}

		spanLog.LogFields(otlog.Int("keys found", len(items)))

		return nil
	})
	if err != nil {
		return found, bufs, keys
	}

	for i, key := range keys {
		if items[i] != nil {
			found = append(found, key)
			bufs = append(bufs, items[i])
		} else {
			missed = append(missed, key)
		}
	}

	return
}

// Store stores the key in the cache.
func (c *RedisCache) Store(ctx context.Context, keys []string, bufs [][]byte) {
	err := c.redis.MSet(ctx, keys, bufs)
	if err != nil {
		level.Error(c.logger).Log("msg", "failed to put to redis", "name", c.name, "err", err)
	}
}

// Stop stops the redis client.
func (c *RedisCache) Stop() {
	_ = c.redis.Close()
}
