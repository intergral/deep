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

package client

import (
	"context"
	"github.com/go-kit/log"
	"github.com/grafana/dskit/ring"
	ring_client "github.com/grafana/dskit/ring/client"
	"github.com/grafana/dskit/services"
	"github.com/grpc-ecosystem/grpc-opentracing/go/otgrpc"
	"github.com/intergral/deep/pkg/deeppb"
	pb "github.com/intergral/deep/pkg/deeppb/poll/v1"
	"github.com/intergral/deep/pkg/util"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/weaveworks/common/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"hash/fnv"
	"io"
)

type TPClient struct {
	services.Service
	pool *ring_client.Pool
	ring ring.ReadRing
}

type tpClient struct {
	deeppb.TracepointConfigServiceClient
	grpc_health_v1.HealthClient
	io.Closer
}

func New(clientCfg Config, tracepointRing ring.ReadRing, logger log.Logger) (*TPClient, error) {

	metricIngesterClients := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "deep",
		Name:      "tracepoint_config_clients",
		Help:      "The current number of tracepoint config clients.",
	})

	pool := ring_client.NewPool("tp_pool",
		clientCfg.PoolConfig,
		ring_client.NewRingServiceDiscovery(tracepointRing),
		func(addr string) (ring_client.PoolClient, error) {
			return newClient(addr, clientCfg)
		},
		metricIngesterClients,
		logger)

	client := TPClient{pool: pool, ring: tracepointRing}

	client.Service = services.NewBasicService(client.starting, client.running, client.stopping)

	return &client, nil
}

func (ts *TPClient) starting(context.Context) error {
	return nil
}

func (ts *TPClient) running(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	}
}

func (ts *TPClient) stopping(error) error {
	return nil
}

// New returns a new ingester client.
func newClient(addr string, cfg Config) (*tpClient, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	instrumentationOpts, err := cfg.GRPCClientConfig.DialOption(instrumentation())
	if err != nil {
		return nil, err
	}

	opts = append(opts, instrumentationOpts...)
	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	return &tpClient{
		TracepointConfigServiceClient: deeppb.NewTracepointConfigServiceClient(conn),
		HealthClient:                  grpc_health_v1.NewHealthClient(conn),
		Closer:                        conn,
	}, nil
}

func instrumentation() ([]grpc.UnaryClientInterceptor, []grpc.StreamClientInterceptor) {
	return []grpc.UnaryClientInterceptor{
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer()),
			middleware.ClientUserHeaderInterceptor,
		}, []grpc.StreamClientInterceptor{
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer()),
			middleware.StreamClientUserHeaderInterceptor,
		}
}

func (ts *TPClient) CreateTracepoint(ctx context.Context, req *deeppb.CreateTracepointRequest) (*deeppb.CreateTracepointResponse, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Querier.Search")
	}

	tokenFor := TokenFor(tenantID)

	get, err := ts.ring.Get(tokenFor, ring.Read, nil, nil, nil)
	_, err = get.Do(ctx, 0, func(funCtx context.Context, desc *ring.InstanceDesc) (interface{}, error) {
		//todo error handling
		client, _ := ts.pool.GetClientFor(desc.Addr)

		grpClient := client.(*tpClient)
		tracepoints, err := grpClient.CreateTracepoint(funCtx, req)
		if err != nil {
			print(err)
		}

		return tracepoints, nil
	})

	return &deeppb.CreateTracepointResponse{}, nil
}

func (ts *TPClient) DeleteTracepoint(ctx context.Context, req *deeppb.DeleteTracepointRequest) (*deeppb.DeleteTracepointResponse, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Querier.Search")
	}

	tokenFor := TokenFor(tenantID)

	get, err := ts.ring.Get(tokenFor, ring.Read, nil, nil, nil)
	_, err = get.Do(ctx, 0, func(funCtx context.Context, desc *ring.InstanceDesc) (interface{}, error) {
		//todo error handling
		client, _ := ts.pool.GetClientFor(desc.Addr)

		grpClient := client.(*tpClient)
		tracepoints, err := grpClient.DeleteTracepoint(funCtx, req)
		if err != nil {
			print(err)
		}

		return tracepoints, nil
	})

	return &deeppb.DeleteTracepointResponse{}, nil
}

func (ts *TPClient) LoadTracepoints(ctx context.Context, req *deeppb.LoadTracepointRequest) (*deeppb.LoadTracepointResponse, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Tracepoint.LoadTracepoints")
	}

	tokenFor := TokenFor(tenantID)

	get, err := ts.ring.Get(tokenFor, ring.Read, nil, nil, nil)

	doResults, err := get.Do(ctx, 0, func(funCtx context.Context, desc *ring.InstanceDesc) (interface{}, error) {
		//todo error handling
		client, _ := ts.pool.GetClientFor(desc.Addr)

		grpClient := client.(*tpClient)
		tracepoints, err := grpClient.LoadTracepoints(funCtx, req)
		if err != nil {
			print(err)
		}

		return tracepoints, nil
	})

	if len(doResults) == 0 {
		// no results so return empty stub
		return &deeppb.LoadTracepointResponse{Response: &pb.PollResponse{
			TsNanos:      req.Request.TsNanos,
			CurrentHash:  "",
			Response:     nil,
			ResponseType: pb.ResponseType_NO_CHANGE,
		}}, nil
	}

	var responses []*deeppb.LoadTracepointResponse

	for _, result := range doResults {
		response := result.(*deeppb.LoadTracepointResponse)
		if response != nil {
			responses = append(responses, response)
		}
	}

	if len(responses) == 0 {
		// no results so return empty stub
		return &deeppb.LoadTracepointResponse{Response: &pb.PollResponse{
			TsNanos:      req.Request.TsNanos,
			CurrentHash:  "",
			Response:     nil,
			ResponseType: pb.ResponseType_NO_CHANGE,
		}}, nil
	}

	return responses[0], nil
}

// TokenFor generates a token used for finding ingesters from ring
func TokenFor(tenantID string) uint32 {
	h := fnv.New32()
	_, _ = h.Write([]byte(tenantID))
	return h.Sum32()
}
