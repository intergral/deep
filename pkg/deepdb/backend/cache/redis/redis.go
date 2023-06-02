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

package redis

import (
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/intergral/deep/pkg/cache"
)

type Config struct {
	ClientConfig cache.RedisConfig `yaml:",inline"`

	TTL time.Duration `yaml:"ttl"`
}

func NewClient(cfg *Config, cfgBackground *cache.BackgroundConfig, logger log.Logger) cache.Cache {
	if cfg.ClientConfig.Timeout == 0 {
		cfg.ClientConfig.Timeout = 100 * time.Millisecond
	}
	if cfg.ClientConfig.Expiration == 0 {
		cfg.ClientConfig.Expiration = cfg.TTL
	}

	client := cache.NewRedisClient(&cfg.ClientConfig)
	c := cache.NewRedisCache("deep", client, prometheus.DefaultRegisterer, logger)

	return cache.NewBackground("deep", *cfgBackground, c, prometheus.DefaultRegisterer)
}
