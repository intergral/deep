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

package ingester

import (
	"fmt"
	"math"

	"github.com/intergral/deep/modules/overrides"
)

const (
	errMaxSnapshotsPerUserLimitExceeded = "per-user snapshots limit (local: %d global: %d actual local: %d) exceeded"
)

// RingCount is the interface exposed by a ring implementation which allows
// to count members
type RingCount interface {
	HealthyInstancesCount() int
}

// Limiter implements primitives to get the maximum number of snapshots
// an ingester can handle for a specific tenant
type Limiter struct {
	limits            *overrides.Overrides
	ring              RingCount
	replicationFactor int
}

// NewLimiter makes a new limiter
func NewLimiter(limits *overrides.Overrides, ring RingCount, replicationFactor int) *Limiter {
	return &Limiter{
		limits:            limits,
		ring:              ring,
		replicationFactor: replicationFactor,
	}
}

// AssertMaxSnapshotsPerUser ensures limit has not been reached compared to the current
// number of streams in input and returns an error if so.
func (l *Limiter) AssertMaxSnapshotsPerUser(userID string, snapshots int) error {
	actualLimit := l.maxSnapshotsPerUser(userID)
	if snapshots < actualLimit {
		return nil
	}

	localLimit := l.limits.MaxLocalSnapshotsPerUser(userID)
	globalLimit := l.limits.MaxGlobalSnapshotsPerUser(userID)

	return fmt.Errorf(errMaxSnapshotsPerUserLimitExceeded, localLimit, globalLimit, actualLimit)
}

func (l *Limiter) maxSnapshotsPerUser(userID string) int {
	localLimit := l.limits.MaxLocalSnapshotsPerUser(userID)

	// We can assume that snapshots are evenly distributed across ingesters
	// so we do convert the global limit into a local limit
	globalLimit := l.limits.MaxGlobalSnapshotsPerUser(userID)
	localLimit = l.minNonZero(localLimit, l.convertGlobalToLocalLimit(globalLimit))

	// If both the local and global limits are disabled, we just
	// use the largest int value
	if localLimit == 0 {
		localLimit = math.MaxInt32
	}

	return localLimit
}

func (l *Limiter) convertGlobalToLocalLimit(globalLimit int) int {
	if globalLimit == 0 {
		return 0
	}

	// Given we don't need a super accurate count (ie. when the ingesters
	// topology changes) and we prefer to always be in favor of the tenant,
	// we can use a per-ingester limit equal to:
	// (global limit / number of ingesters) * replication factor
	numIngesters := l.ring.HealthyInstancesCount()

	// May happen because the number of ingesters is asynchronously updated.
	// If happens, we just deeprarily ignore the global limit.
	if numIngesters > 0 {
		return int((float64(globalLimit) / float64(numIngesters)) * float64(l.replicationFactor))
	}

	return 0
}

func (l *Limiter) minNonZero(first, second int) int {
	if first == 0 || (second != 0 && first > second) {
		return second
	}

	return first
}
