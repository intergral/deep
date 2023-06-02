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

package spanmetrics

import (
	"flag"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	Name = "span-metrics"

	dimService  = "service"
	dimFilePath = "file_path"
	dimLineNo   = "line_no"
)

type Config struct {
	// Buckets for latency histogram in seconds.
	HistogramBuckets []float64 `yaml:"histogram_buckets"`
	// Intrinsic dimensions (labels) added to the metric, that are generated from fixed span
	// data. The dimensions service, span_name, span_kind, and status_code are enabled by
	// default, whereas the dimension status_message must be enabled explicitly.
	IntrinsicDimensions IntrinsicDimensions `yaml:"intrinsic_dimensions"`
	// Additional dimensions (labels) to be added to the metric. The dimensions are generated
	// from span attributes and are created along with the intrinsic dimensions.
	Dimensions []string `yaml:"dimensions"`

	// If enabled attribute value will be used for metric calculation
	SpanMultiplierKey string `yaml:"span_multiplier_key"`
}

func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {
	cfg.HistogramBuckets = prometheus.ExponentialBuckets(0.002, 2, 14)
	cfg.IntrinsicDimensions.Service = true
	cfg.IntrinsicDimensions.FilePath = true
	cfg.IntrinsicDimensions.LineNo = true
}

type IntrinsicDimensions struct {
	Service  bool `yaml:"service"`
	FilePath bool `yaml:"file_path"`
	LineNo   bool `yaml:"line_no"`
}

func (ic *IntrinsicDimensions) ApplyFromMap(dimensions map[string]bool) error {
	for label, active := range dimensions {
		switch label {
		case dimService:
			ic.Service = active
		case dimFilePath:
			ic.FilePath = active
		case dimLineNo:
			ic.LineNo = active
		default:
			return errors.Errorf("%s is not a valid intrinsic dimension", label)
		}
	}
	return nil
}
