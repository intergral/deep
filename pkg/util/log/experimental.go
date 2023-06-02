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

package log

import (
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var experimentalFeaturesInUse = promauto.NewCounter(
	prometheus.CounterOpts{
		Namespace: "cortex",
		Name:      "experimental_features_in_use_total",
		Help:      "The number of experimental features in use.",
	},
)

// WarnExperimentalUse logs a warning and increments the experimental features metric.
func WarnExperimentalUse(feature string) {
	level.Warn(Logger).Log("msg", "experimental feature in use", "feature", feature)
	experimentalFeaturesInUse.Inc()
}
