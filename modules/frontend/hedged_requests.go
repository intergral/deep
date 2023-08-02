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

package frontend

import (
	"net/http"
	"time"

	"github.com/cristalhq/hedgedhttp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	hedgedMetricsPublishDuration = 10 * time.Second
)

var (
	hedgedRequestsMetrics = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "deep",
			Name:      "query_frontend_hedged_roundtrips_total",
			Help:      "Total number of hedged snapshot by ID requests. Registered as a gauge for code sanity. This is a counter.",
		},
	)
)

func newHedgedRequestWare(cfg HedgingConfig) Middleware {
	return MiddlewareFunc(func(next http.RoundTripper) http.RoundTripper {
		if cfg.HedgeRequestsAt == 0 {
			return next
		}
		ret, stats, err := hedgedhttp.NewRoundTripperAndStats(cfg.HedgeRequestsAt, cfg.HedgeRequestsUpTo, next)
		if err != nil {
			panic(err)
		}
		publishHedgedMetrics(stats)
		return ret
	})
}

// PublishHedgedMetrics flushes metrics from hedged requests every 10 seconds
func publishHedgedMetrics(s *hedgedhttp.Stats) {
	ticker := time.NewTicker(hedgedMetricsPublishDuration)
	go func() {
		for range ticker.C {
			snap := s.Snapshot()
			hedgedRequests := int64(snap.ActualRoundTrips) - int64(snap.RequestedRoundTrips)
			if hedgedRequests < 0 {
				hedgedRequests = 0
			}
			hedgedRequestsMetrics.Set(float64(hedgedRequests))
		}
	}()
}
