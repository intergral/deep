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

package deepql

import "context"

type Operands []Static

type Condition struct {
	Attribute Attribute
	Op        Operator
	Operands  Operands
}

// FilterSnapshots is a hint that allows the calling code to filter down spans to only
// those that metadata needs to be retrieved for. If the returned Spanset has no
// spans it is discarded and will not appear in FetchSpansResponse. The bool
// return value is used to indicate if the Fetcher should continue iterating or if
// it can bail out.
type FilterSnapshots func(result *SnapshotResult) ([]*SnapshotResult, error)

type FetchSnapshotRequest struct {
	StartTimeUnixNanos uint64
	EndTimeUnixNanos   uint64
	Conditions         []Condition

	// AllConditions, by default the storage layer fetches spans meeting any of the criteria.
	// This hint is for common cases like { x && y && z } where the storage layer
	// can make extra optimizations by returning only snapshots that meet
	// all criteria.
	AllConditions bool
	Filter        FilterSnapshots
}

type SnapshotResult struct {
	// these fields are actually used by the engine to evaluate queries
	Scalar   Static
	Snapshot Snapshot

	SnapshotID         []byte
	ServiceName        string
	FilePath           string
	LineNo             uint32
	StartTimeUnixNanos uint64
	DurationNanos      uint64
}

func (s *SnapshotResult) clone() *SnapshotResult {
	return &SnapshotResult{
		SnapshotID:         s.SnapshotID,
		Scalar:             s.Scalar,
		ServiceName:        s.ServiceName,
		FilePath:           s.FilePath,
		LineNo:             s.LineNo,
		StartTimeUnixNanos: s.StartTimeUnixNanos,
		DurationNanos:      s.DurationNanos,
		Snapshot:           s.Snapshot, // we're not deep cloning into the spans themselves
	}
}

type Snapshot interface {
	// Attributes are the actual fields used by the engine to evaluate queries
	// if a Filter parameter is passed the spans returned will only have this field populated
	Attributes() map[Attribute]Static

	ID() []byte
	StartTimeUnixNanos() uint64
	EndTimeUnixNanos() uint64
}

func (f *FetchSnapshotRequest) appendCondition(c ...Condition) {
	f.Conditions = append(f.Conditions, c...)
}

type SnapshotResultIterator interface {
	Next(context.Context) (*SnapshotResult, error)
	Close()
}

type FetchSnapshotResponse struct {
	Results SnapshotResultIterator
	Bytes   func() uint64
}

type SnapshotResultFetcher interface {
	Fetch(context.Context, FetchSnapshotRequest) (FetchSnapshotResponse, error)
}

type SnapshotResultFetcherWrapper struct {
	f func(ctx context.Context, req FetchSnapshotRequest) (FetchSnapshotResponse, error)
}

var _ = (SnapshotResultFetcher)(&SnapshotResultFetcherWrapper{})

func NewSnapshotResultFetcherWrapper(f func(ctx context.Context, req FetchSnapshotRequest) (FetchSnapshotResponse, error)) SnapshotResultFetcher {
	return SnapshotResultFetcherWrapper{f}
}

func (s SnapshotResultFetcherWrapper) Fetch(ctx context.Context, request FetchSnapshotRequest) (FetchSnapshotResponse, error) {
	return s.f(ctx, request)
}
