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
	"fmt"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/golang/protobuf/proto"
	"github.com/grafana/dskit/backoff"
	"github.com/intergral/deep/modules/frontend/v1/frontendv1pb"
	"github.com/intergral/deep/modules/querier/stats"
	"github.com/weaveworks/common/httpgrpc"
	"google.golang.org/grpc"
	"net/http"
	"time"
)

var (
	processorBackoffConfig = backoff.Config{
		MinBackoff: 50 * time.Millisecond,
		MaxBackoff: 1 * time.Second,
	}
)

func NewFrontendProcessor(cfg Config, handler RequestHandler, log log.Logger, prcFunc ProcessorFunction) Processor {
	return &frontendProcessor{
		log:            log,
		handler:        handler,
		maxMessageSize: cfg.GRPCClientConfig.MaxSendMsgSize,
		querierID:      cfg.QuerierID,
		prcFunc:        prcFunc,
	}
}

// Handles incoming queries from frontend.
type frontendProcessor struct {
	handler        RequestHandler
	maxMessageSize int
	querierID      string

	log     log.Logger
	prcFunc func(frontendv1pb.FrontendClient, context.Context) (frontendv1pb.Frontend_ProcessClient, error)
}

func (fp frontendProcessor) ProcessQueriesOnSingleStream(ctx context.Context, conn *grpc.ClientConn, address string) {
	client := frontendv1pb.NewFrontendClient(conn)

	procBackoff := backoff.New(ctx, processorBackoffConfig)
	for procBackoff.Ongoing() {
		c, err := fp.prcFunc(client, ctx)
		if err != nil {
			level.Error(fp.log).Log("msg", "error contacting frontend", "address", address, "err", err)
			procBackoff.Wait()
			continue
		}

		if err := fp.process(c); err != nil {
			level.Error(fp.log).Log("msg", "error processing requests", "address", address, "err", err)
			procBackoff.Wait()
			continue
		}

		procBackoff.Reset()
	}
}

func (fp frontendProcessor) NotifyShutdown(ctx context.Context, conn *grpc.ClientConn, address string) {
	client := frontendv1pb.NewFrontendClient(conn)

	req := &frontendv1pb.NotifyClientShutdownRequest{ClientID: fp.querierID}
	if _, err := client.NotifyClientShutdown(ctx, req); err != nil {
		// Since we're shutting down there's nothing we can do except logging it.
		level.Warn(fp.log).Log("msg", "failed to notify tracepoint shutdown to query-frontend", "address", address, "err", err)
	}
}

func (fp frontendProcessor) process(c frontendv1pb.Frontend_ProcessTracepointClient) error {
	// Build a child context so we can cancel a query when the stream is closed.
	ctx, cancel := context.WithCancel(c.Context())
	defer cancel()

	for {
		request, err := c.Recv()
		if err != nil {
			return err
		}

		switch request.Type {
		case frontendv1pb.Type_HTTP_REQUEST:
			// Handle the request on a "background" goroutine, so we go back to
			// blocking on c.Recv().  This allows us to detect the stream closing
			// and cancel the query.  We don't actually handle queries in parallel
			// here, as we're running in lock step with the server - each Recv is
			// paired with a Send.
			go fp.runRequest(ctx, request.HttpRequest, request.StatsEnabled, func(response *frontendv1pb.HTTPResponse, stats *stats.Stats) error {
				return c.Send(&frontendv1pb.ClientToFrontend{
					HttpResponse: response,
					Stats:        stats,
				})
			})

		case frontendv1pb.Type_GET_ID:
			err := c.Send(&frontendv1pb.ClientToFrontend{ClientID: fp.querierID})
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unknown request type: %v", request.Type)
		}
	}
}

func (fp frontendProcessor) runRequest(ctx context.Context, request *frontendv1pb.HTTPRequest, statsEnabled bool, sendHTTPResponse func(response *frontendv1pb.HTTPResponse, stats *stats.Stats) error) {
	var querierStats *stats.Stats
	if statsEnabled {
		querierStats, ctx = stats.ContextWithEmptyStats(ctx)
	}

	response, err := fp.handle(ctx, request)
	if err != nil {
		var ok bool
		response, ok = fp.responseError(err)
		if !ok {
			response = &frontendv1pb.HTTPResponse{
				Code: http.StatusInternalServerError,
				Body: []byte(err.Error()),
			}
		}
	}

	// Ensure responses that are too big are not retried.
	if len(response.Body) >= fp.maxMessageSize {
		errMsg := fmt.Sprintf("response larger than the max (%d vs %d)", len(response.Body), fp.maxMessageSize)
		response = &frontendv1pb.HTTPResponse{
			Code: http.StatusRequestEntityTooLarge,
			Body: []byte(errMsg),
		}
		level.Error(fp.log).Log("msg", "error processing query", "err", errMsg)
	}

	if err := sendHTTPResponse(response, querierStats); err != nil {
		level.Error(fp.log).Log("msg", "error processing requests", "err", err)
	}
}

func (fp frontendProcessor) handle(ctx context.Context, request *frontendv1pb.HTTPRequest) (*frontendv1pb.HTTPResponse, error) {
	handle, err := fp.handler.Handle(ctx, fp.toHttpGrpc(request))
	return fp.fromHttpGrpc(handle), err
}

func (fp frontendProcessor) fromHttpGrpc(response *httpgrpc.HTTPResponse) *frontendv1pb.HTTPResponse {
	marshal, _ := proto.Marshal(response)
	ourResponse := &frontendv1pb.HTTPResponse{}
	_ = proto.Unmarshal(marshal, ourResponse)
	return ourResponse
}

func (fp frontendProcessor) toHttpGrpc(request *frontendv1pb.HTTPRequest) *httpgrpc.HTTPRequest {
	marshal, _ := proto.Marshal(request)
	req := &httpgrpc.HTTPRequest{}
	_ = proto.Unmarshal(marshal, req)
	return req
}

func (fp frontendProcessor) responseError(err error) (*frontendv1pb.HTTPResponse, bool) {
	fromError, b := httpgrpc.HTTPResponseFromError(err)
	if fromError == nil {
		return nil, b
	}
	return fp.fromHttpGrpc(fromError), b
}
