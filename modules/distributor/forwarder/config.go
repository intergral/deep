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

package forwarder

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/intergral/deep/modules/distributor/forwarder/otlpgrpc"
)

const (
	OTLPGRPCBackend = "otlpgrpc"
)

type Config struct {
	Name     string          `yaml:"name"`
	Backend  string          `yaml:"backend"`
	OTLPGRPC otlpgrpc.Config `yaml:"otlpgrpc"`
}

func (cfg *Config) Validate() error {
	if cfg.Name == "" {
		return errors.New("name is empty")
	}

	switch cfg.Backend {
	case OTLPGRPCBackend:
		return cfg.OTLPGRPC.Validate()
	default:
	}

	return fmt.Errorf("%s backend is not supported", cfg.Backend)
}

type ConfigList []Config

func (cfgs ConfigList) Validate() error {
	for i, cfg := range cfgs {
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("failed to validate config at index=%d: %w", i, err)
		}
	}

	return nil
}
