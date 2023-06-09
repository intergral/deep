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

package registry

import (
	"flag"
	"time"
)

type Config struct {
	// CollectionInterval controls how often to collect metrics.
	// Defaults to 15s.
	CollectionInterval time.Duration `yaml:"collection_interval"`

	// StaleDuration controls how quickly series become stale and are deleted from the registry. An active
	// series is deleted if it hasn't been updated more stale duration.
	// Defaults to 15m.
	StaleDuration time.Duration `yaml:"stale_duration"`

	// ExternalLabels are added to any time series generated by this instance.
	ExternalLabels map[string]string `yaml:"external_labels,omitempty"`

	// MaxLabelNameLength configures the maximum length of label names. Label names exceeding
	// this limit will be truncated.
	MaxLabelNameLength int `yaml:"max_label_name_length"`

	// MaxLabelValueLength configures the maximum length of label values. Label values exceeding
	// this limit will be truncated.
	MaxLabelValueLength int `yaml:"max_label_value_length"`
}

// RegisterFlagsAndApplyDefaults registers the flags.
func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {
	cfg.CollectionInterval = 15 * time.Second
	cfg.StaleDuration = 15 * time.Minute
	cfg.MaxLabelNameLength = 1024
	cfg.MaxLabelValueLength = 2048
}
