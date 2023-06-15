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

package worker

import (
	"context"
	"github.com/grafana/dskit/grpcclient"
	"github.com/intergral/deep/modules/frontend/v1/frontendv1pb"
	"github.com/weaveworks/common/httpgrpc"
	"google.golang.org/grpc"
	"time"
)

type Config struct {
	FrontendAddress string        `yaml:"frontend_address"`
	DNSLookupPeriod time.Duration `yaml:"dns_lookup_duration"`

	Parallelism           int  `yaml:"parallelism"`
	MatchMaxConcurrency   bool `yaml:"match_max_concurrent"`
	MaxConcurrentRequests int  `yaml:"-"`

	QuerierID string `yaml:"id"`

	GRPCClientConfig grpcclient.Config `yaml:"grpc_client_config"`
}

// Single processor handles all streaming operations to query-frontend or query-scheduler to fetch queries
// and process them.
type Processor interface {
	// Each invocation of processQueriesOnSingleStream starts new streaming operation to query-frontend
	// or query-scheduler to fetch queries and execute them.
	//
	// This method must react on context being finished, and stop when that happens.
	//
	// ProcessorManager (not Processor) is responsible for starting as many goroutines as needed for each connection.
	ProcessQueriesOnSingleStream(ctx context.Context, conn *grpc.ClientConn, address string)

	// notifyShutdown notifies the remote query-frontend or query-scheduler that the querier is
	// shutting down.
	NotifyShutdown(ctx context.Context, conn *grpc.ClientConn, address string)
}

// Handler for HTTP requests wrapped in protobuf messages.
type RequestHandler interface {
	Handle(context.Context, *httpgrpc.HTTPRequest) (*httpgrpc.HTTPResponse, error)
}

type ProcessorFunction func(client frontendv1pb.FrontendClient, ctx context.Context) (frontendv1pb.Frontend_ProcessClient, error)
