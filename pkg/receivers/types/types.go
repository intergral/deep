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

package types

import (
	"context"
	pb "github.com/intergral/go-deep-proto/poll/v1"
	tp "github.com/intergral/go-deep-proto/tracepoint/v1"
)

type Host interface {
	ReportFatalError(err error)
}

type Receiver interface {
	Start(ctx context.Context, host Host) error

	Shutdown(ctx context.Context) error
}

type ProcessSnapshots func(ctx context.Context, in *tp.Snapshot) (*tp.SnapshotResponse, error)
type ProcessPoll func(ctx context.Context, pollRequest *pb.PollRequest) (*pb.PollResponse, error)
