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
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	prometheus_config "github.com/prometheus/prometheus/config"
	"github.com/weaveworks/common/user"

	"github.com/intergral/deep/pkg/util"
)

// generateTenantRemoteWriteConfigs creates a copy of the remote write configurations with the
// X-Scope-OrgID header present for the given tenant, unless Tempo is run in single tenant mode.
func generateTenantRemoteWriteConfigs(originalCfgs []prometheus_config.RemoteWriteConfig, tenant string, logger log.Logger) []*prometheus_config.RemoteWriteConfig {
	var cloneCfgs []*prometheus_config.RemoteWriteConfig

	for _, originalCfg := range originalCfgs {
		cloneCfg := &prometheus_config.RemoteWriteConfig{}
		*cloneCfg = originalCfg

		// Inject/overwrite X-Scope-OrgID header in multi-tenant setups
		if tenant != util.FakeTenantID {
			// Copy headers so we can modify them
			cloneCfg.Headers = copyMap(cloneCfg.Headers)

			// Ensure that no variation of the X-Scope-OrgId header can be added, which might trick authentication
			for k, v := range cloneCfg.Headers {
				if strings.EqualFold(user.OrgIDHeaderName, strings.TrimSpace(k)) {
					level.Warn(logger).Log("msg", "discarding X-Scope-OrgId header", "key", k, "value", v)
					delete(cloneCfg.Headers, k)
				}
			}

			cloneCfg.Headers[user.OrgIDHeaderName] = tenant
		}

		cloneCfgs = append(cloneCfgs, cloneCfg)
	}

	return cloneCfgs
}

// copyMap creates a new map containing all values from the given map.
func copyMap(m map[string]string) map[string]string {
	newMap := make(map[string]string, len(m))

	for k, v := range m {
		newMap[k] = v
	}

	return newMap
}
