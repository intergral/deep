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

package otlpgrpc

import (
	"github.com/grafana/dskit/flagext"
	"github.com/pkg/errors"
)

type Config struct {
	Endpoints flagext.StringSlice `yaml:"endpoints"`
	TLS       TLSConfig           `yaml:"tls"`
}

func (cfg *Config) Validate() error {
	// TODO: Validate if endpoints are in form host:port?
	return cfg.TLS.Validate()
}

type TLSConfig struct {
	Insecure bool   `yaml:"insecure"`
	CertFile string `yaml:"cert_file"`
}

func (cfg *TLSConfig) Validate() error {
	if cfg.Insecure {
		return nil
	}

	if cfg.CertFile == "" {
		return errors.New("cert_file is empty")
	}

	return nil
}
