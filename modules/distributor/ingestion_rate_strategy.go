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

package distributor

import (
	"github.com/grafana/dskit/limiter"
	"github.com/intergral/deep/modules/overrides"
)

// ReadLifecycler represents the read interface to the lifecycler.
type ReadLifecycler interface {
	HealthyInstancesCount() int
}

type localStrategy struct {
	limits *overrides.Overrides
}

func newLocalIngestionRateStrategy(limits *overrides.Overrides) limiter.RateLimiterStrategy {
	return &localStrategy{
		limits: limits,
	}
}

func (s *localStrategy) Limit(userID string) float64 {
	return s.limits.IngestionRateLimitBytes(userID)
}

func (s *localStrategy) Burst(userID string) int {
	return s.limits.IngestionBurstSizeBytes(userID)
}

type globalStrategy struct {
	limits *overrides.Overrides
	ring   ReadLifecycler
}

func newGlobalIngestionRateStrategy(limits *overrides.Overrides, ring ReadLifecycler) limiter.RateLimiterStrategy {
	return &globalStrategy{
		limits: limits,
		ring:   ring,
	}
}

func (s *globalStrategy) Limit(userID string) float64 {
	numDistributors := s.ring.HealthyInstancesCount()

	if numDistributors == 0 {
		return s.limits.IngestionRateLimitBytes(userID)
	}

	return s.limits.IngestionRateLimitBytes(userID) / float64(numDistributors)
}

func (s *globalStrategy) Burst(userID string) int {
	// The meaning of burst doesn't change for the global strategy, in order
	// to keep it easier to understand for users / operators.
	return s.limits.IngestionBurstSizeBytes(userID)
}
