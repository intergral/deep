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

package usagestats

import (
	"flag"

	"github.com/grafana/dskit/backoff"
	"github.com/intergral/deep/pkg/util"
)

type Config struct {
	Enabled bool           `yaml:"reporting_enabled"`
	Leader  bool           `yaml:"-"`
	Backoff backoff.Config `yaml:"backoff"`
}

// RegisterFlags adds the flags required to config this to the given FlagSet
func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {
	f.BoolVar(&cfg.Enabled, util.PrefixConfig(prefix, "enabled"), true, "Enable anonymous usage reporting.")
	cfg.Backoff.RegisterFlagsWithPrefix(prefix, f)
	cfg.Backoff.MaxRetries = 0
}
