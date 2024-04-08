/*
 * Copyright (C) 2024  Intergral GmbH
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

import (
	"context"

	"github.com/intergral/deep/pkg/deeppb"
)

type Operands []Static

type Condition struct {
	Attribute string
	Op        Operator
	Operands  Operands
}

type FetchSnapshotRequest struct {
	StartTimeUnixNanos uint64
	EndTimeUnixNanos   uint64
	Conditions         []Condition
}

type SnapshotResult struct {
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
		ServiceName:        s.ServiceName,
		FilePath:           s.FilePath,
		LineNo:             s.LineNo,
		StartTimeUnixNanos: s.StartTimeUnixNanos,
		DurationNanos:      s.DurationNanos,
	}
}

type Snapshot interface {
	// Attributes are the actual fields used by the engine to evaluate queries
	// if a Filter parameter is passed the snapshots returned will only have this field populated
	Attributes() map[string]Static

	ID() []byte
	StartTimeUnixNanos() uint64
	EndTimeUnixNanos() uint64
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

type TriggerHandler func(context.Context, *deeppb.CreateTracepointRequest) (*deeppb.LoadTracepointResponse, error)

type CommandRequest struct {
	DeleteRequest *deeppb.DeleteTracepointRequest
	LoadRequest   *deeppb.LoadTracepointRequest
}

type CommandHandler func(context.Context, *CommandRequest) (*deeppb.LoadTracepointResponse, error)
