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

package azure

import (
	"time"

	"github.com/grafana/dskit/flagext"
)

type Config struct {
	StorageAccountName string         `yaml:"storage_account_name"`
	StorageAccountKey  flagext.Secret `yaml:"storage_account_key"`
	UseManagedIdentity bool           `yaml:"use_managed_identity"`
	UseFederatedToken  bool           `yaml:"use_federated_token"`
	UserAssignedID     string         `yaml:"user_assigned_id"`
	ContainerName      string         `yaml:"container_name"`
	Endpoint           string         `yaml:"endpoint_suffix"`
	MaxBuffers         int            `yaml:"max_buffers"`
	BufferSize         int            `yaml:"buffer_size"`
	HedgeRequestsAt    time.Duration  `yaml:"hedge_requests_at"`
	HedgeRequestsUpTo  int            `yaml:"hedge_requests_up_to"`
}
