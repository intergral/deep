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

package storage

import (
	"net/url"
	"testing"
	"time"

	prometheus_common_config "github.com/prometheus/common/config"
	prometheus_config "github.com/prometheus/prometheus/config"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestConfig(t *testing.T) {
	cfgStr := `
path: /var/wal/deep
wal:
  wal_compression: true
remote_write_flush_deadline: 5m
remote_write:
  - url: http://prometheus/api/prom/push
    headers:
      foo: bar
`

	var cfg Config
	cfg.RegisterFlagsAndApplyDefaults("", nil)

	err := yaml.UnmarshalStrict([]byte(cfgStr), &cfg)
	assert.NoError(t, err)

	walCfg := agentDefaultOptions()
	walCfg.WALCompression = true

	remoteWriteConfig := prometheus_config.DefaultRemoteWriteConfig
	prometheusURL, err := url.Parse("http://prometheus/api/prom/push")
	assert.NoError(t, err)
	remoteWriteConfig.URL = &prometheus_common_config.URL{URL: prometheusURL}
	remoteWriteConfig.Headers = map[string]string{
		"foo": "bar",
	}

	expectedCfg := Config{
		Path:                     "/var/wal/deep",
		Wal:                      walCfg,
		RemoteWriteFlushDeadline: 5 * time.Minute,
		RemoteWrite: []prometheus_config.RemoteWriteConfig{
			remoteWriteConfig,
		},
	}
	assert.Equal(t, expectedCfg, cfg)
}
