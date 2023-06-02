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

package stats

import (
	"context"
	"sync/atomic" //lint:ignore faillint we can't use go.uber.org/atomic with a protobuf struct without wrapping it.
	"time"

	"github.com/weaveworks/common/httpgrpc"
)

type contextKey int

var ctxKey = contextKey(0)

// ContextWithEmptyStats returns a context with empty stats.
func ContextWithEmptyStats(ctx context.Context) (*Stats, context.Context) {
	stats := &Stats{}
	ctx = context.WithValue(ctx, ctxKey, stats)
	return stats, ctx
}

// FromContext gets the Stats out of the Context. Returns nil if stats have not
// been initialised in the context.
func FromContext(ctx context.Context) *Stats {
	o := ctx.Value(ctxKey)
	if o == nil {
		return nil
	}
	return o.(*Stats)
}

// IsEnabled returns whether stats tracking is enabled in the context.
func IsEnabled(ctx context.Context) bool {
	// When query statistics are enabled, the stats object is already initialised
	// within the context, so we can just check it.
	return FromContext(ctx) != nil
}

// AddWallTime adds some time to the counter.
func (s *Stats) AddWallTime(t time.Duration) {
	if s == nil {
		return
	}

	atomic.AddInt64((*int64)(&s.WallTime), int64(t))
}

// LoadWallTime returns current wall time.
func (s *Stats) LoadWallTime() time.Duration {
	if s == nil {
		return 0
	}

	return time.Duration(atomic.LoadInt64((*int64)(&s.WallTime)))
}

func (s *Stats) AddFetchedSeries(series uint64) {
	if s == nil {
		return
	}

	atomic.AddUint64(&s.FetchedSeriesCount, series)
}

func (s *Stats) LoadFetchedSeries() uint64 {
	if s == nil {
		return 0
	}

	return atomic.LoadUint64(&s.FetchedSeriesCount)
}

func (s *Stats) AddFetchedChunkBytes(bytes uint64) {
	if s == nil {
		return
	}

	atomic.AddUint64(&s.FetchedChunkBytes, bytes)
}

func (s *Stats) LoadFetchedChunkBytes() uint64 {
	if s == nil {
		return 0
	}

	return atomic.LoadUint64(&s.FetchedChunkBytes)
}

// Merge the provide Stats into this one.
func (s *Stats) Merge(other *Stats) {
	if s == nil || other == nil {
		return
	}

	s.AddWallTime(other.LoadWallTime())
	s.AddFetchedSeries(other.LoadFetchedSeries())
	s.AddFetchedChunkBytes(other.LoadFetchedChunkBytes())
}

func ShouldTrackHTTPGRPCResponse(r *httpgrpc.HTTPResponse) bool {
	// Do no track statistics for requests failed because of a server error.
	return r.Code < 500
}
