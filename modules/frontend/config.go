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
	"flag"
	"net/http"
	"time"

	"github.com/intergral/deep/pkg/api"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/intergral/deep/modules/frontend/transport"
	v1 "github.com/intergral/deep/modules/frontend/v1"
	"github.com/intergral/deep/pkg/usagestats"
)

var statVersion = usagestats.NewString("frontend_version")

type Config struct {
	Config               v1.Config          `yaml:",inline"`
	MaxRetries           int                `yaml:"max_retries,omitempty"`
	TolerateFailedBlocks int                `yaml:"tolerate_failed_blocks,omitempty"`
	Search               SearchConfig       `yaml:"search"`
	SnapshotByID         SnapshotByIDConfig `yaml:"snapshot_by_id"`
}

type SearchConfig struct {
	Sharder SearchSharderConfig `yaml:",inline"`
	SLO     SLOConfig           `yaml:",inline"`
}

type SnapshotByIDConfig struct {
	QueryShards int           `yaml:"query_shards,omitempty"`
	Hedging     HedgingConfig `yaml:",inline"`
	SLO         SLOConfig     `yaml:",inline"`
}

type HedgingConfig struct {
	HedgeRequestsAt   time.Duration `yaml:"hedge_requests_at"`
	HedgeRequestsUpTo int           `yaml:"hedge_requests_up_to"`
}

type SLOConfig struct {
	DurationSLO        time.Duration `yaml:"duration_slo,omitempty"`
	ThroughputBytesSLO float64       `yaml:"throughput_bytes_slo,omitempty"`
}

func (cfg *Config) RegisterFlagsAndApplyDefaults(string, *flag.FlagSet) {
	slo := SLOConfig{
		DurationSLO:        0,
		ThroughputBytesSLO: 0,
	}

	cfg.Config.MaxOutstandingPerTenant = 2000
	cfg.MaxRetries = 2
	cfg.TolerateFailedBlocks = 0
	cfg.Search = SearchConfig{
		Sharder: SearchSharderConfig{
			QueryBackendAfter:     15 * time.Minute,
			QueryIngestersUntil:   30 * time.Minute,
			DefaultLimit:          20,
			MaxLimit:              0,
			MaxDuration:           168 * time.Hour, // 1 week
			ConcurrentRequests:    defaultConcurrentRequests,
			TargetBytesPerRequest: defaultTargetBytesPerRequest,
		},
		SLO: slo,
	}
	cfg.SnapshotByID = SnapshotByIDConfig{
		QueryShards: 50,
		SLO:         slo,
		Hedging: HedgingConfig{
			HedgeRequestsAt:   2 * time.Second,
			HedgeRequestsUpTo: 2,
		},
	}
}

type CortexNoQuerierLimits struct{}

var _ v1.Limits = (*CortexNoQuerierLimits)(nil)

func (CortexNoQuerierLimits) MaxQueriersPerUser(string) int { return 0 }

// InitFrontend initializes V1 frontend
//
// Returned RoundTripper can be wrapped in more round-tripper middlewares, and then eventually registered
// into HTTP server using the Handler from this package. Returned RoundTripper is always non-nil
// (if there are no errors), and it uses the returned frontend (if any).
func InitFrontend(cfg v1.Config, limits v1.Limits, log log.Logger, reg prometheus.Registerer) (http.RoundTripper, *v1.Frontend, error) {
	statVersion.Set("v1")
	// No scheduler = use original frontend.
	fr, err := v1.New(cfg, limits, log, reg)
	if err != nil {
		return nil, nil, err
	}
	// create both round trippers
	searchRt := transport.AdaptGrpcRoundTripperToHTTPRoundTripper(fr.QuerierRoundTrip)
	tracepointRt := transport.AdaptGrpcRoundTripperToHTTPRoundTripper(fr.TracepointRoundTrip)
	// filter the round trippers based on the paths
	tripper := transport.NewSplittingRoundTripper(&transport.SplitRule{
		Rules: []string{
			api.PathPrefixTracepoints,
		},
		RoundTripper: tracepointRt,
	}, &transport.SplitRule{
		Rules: []string{
			api.PathPrefixQuerier,
		},
		RoundTripper: searchRt,
	})

	return tripper, fr, nil
}
