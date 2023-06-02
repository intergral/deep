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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_job(t *testing.T) {
	interval := durationPtr(200 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	var jobTimes []time.Duration
	lastRun := time.Now()

	go job(
		ctx,
		func(_ context.Context) {
			diff := time.Since(lastRun)
			lastRun = time.Now()

			jobTimes = append(jobTimes, diff)
			fmt.Println(diff)

			*interval = *interval + (20 * time.Millisecond)
		},
		func() time.Duration {
			return *interval
		},
	)

	time.Sleep(1 * time.Second)

	cancel()

	require.Len(t, jobTimes, 4)
	require.InDelta(t, 200*time.Millisecond, jobTimes[0], float64(10*time.Millisecond))
	require.InDelta(t, 220*time.Millisecond, jobTimes[1], float64(10*time.Millisecond))
	require.InDelta(t, 240*time.Millisecond, jobTimes[2], float64(10*time.Millisecond))
	require.InDelta(t, 260*time.Millisecond, jobTimes[3], float64(10*time.Millisecond))
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}
