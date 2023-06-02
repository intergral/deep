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

package instrumentation

import (
	"github.com/cristalhq/hedgedhttp"
	"github.com/intergral/deep/pkg/hedgedmetrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	hedgedRequestsMetrics = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "deepdb",
			Name:      "backend_hedged_roundtrips_total",
			Help:      "Total number of hedged backend requests. Registered as a gauge for code sanity. This is a counter.",
		},
	)
)

// PublishHedgedMetrics flushes metrics from hedged requests every 10 seconds
func PublishHedgedMetrics(s *hedgedhttp.Stats) {
	hedgedmetrics.Publish(s, hedgedRequestsMetrics)
}
