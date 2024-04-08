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

package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/intergral/deep/pkg/deepql"

	"github.com/go-kit/log"
	"github.com/golang/protobuf/jsonpb"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/grafana/dskit/services"
	"github.com/intergral/deep/modules/frontend/v1/frontendv1pb"
	"github.com/intergral/deep/modules/tracepoint/client"
	"github.com/intergral/deep/pkg/api"
	"github.com/intergral/deep/pkg/deeppb"
	cp "github.com/intergral/deep/pkg/deeppb/common/v1"
	pb "github.com/intergral/deep/pkg/deeppb/poll/v1"
	rp "github.com/intergral/deep/pkg/deeppb/resource/v1"
	"github.com/intergral/deep/pkg/worker"
	"github.com/opentracing/opentracing-go"
	httpgrpc_server "github.com/weaveworks/common/httpgrpc/server"
)

type TracepointAPI struct {
	services.Service

	cfg    Config
	client *client.TPClient
	log    log.Logger

	subServices        *services.Manager
	subServicesWatcher *services.FailureWatcher
}

func (ta *TracepointAPI) starting(ctx context.Context) error {
	if ta.subServices != nil {
		err := services.StartManagerAndAwaitHealthy(ctx, ta.subServices)
		if err != nil {
			return fmt.Errorf("failed to start subservices %w", err)
		}
	}

	return nil
}

func (ta *TracepointAPI) running(ctx context.Context) error {
	if ta.subServices != nil {
		select {
		case <-ctx.Done():
			return nil
		case err := <-ta.subServicesWatcher.Chan():
			return fmt.Errorf("subservices failed %w", err)
		}
	} else {
		<-ctx.Done()
	}
	return nil
}

func (ta *TracepointAPI) stopping(_ error) error {
	if ta.subServices != nil {
		return services.StopManagerAndAwaitStopped(context.Background(), ta.subServices)
	}
	return nil
}

func NewTracepointAPI(cfg Config, tpClient *client.TPClient, log log.Logger) (*TracepointAPI, error) {
	service := &TracepointAPI{
		cfg:    cfg,
		client: tpClient,
		log:    log,
	}

	service.Service = services.NewBasicService(service.starting, service.running, service.stopping)

	return service, nil
}

func (ta *TracepointAPI) QueryTracepointHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(ta.cfg.LoadTracepoint.Timeout))
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "TracepointAPI.QueryTracepointHandler")
	defer span.Finish()

	isQuery, query := api.IsDeepQLReq(r)
	if !isQuery {
		http.Error(w, "request doesn't contain deepql query", http.StatusBadRequest)
		return
	}

	span.SetTag("deepql", query)
	expr, err := deepql.ParseString(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if expr.IsTrigger() {
		query, err := deepql.NewEngine().ExecuteTriggerQuery(ctx, expr, func(ctx context.Context, request *deeppb.CreateTracepointRequest) (*deeppb.LoadTracepointResponse, error) {
			_, err := ta.client.CreateTracepoint(ctx, request)
			if err != nil {
				return nil, err
			}
			return ta.client.LoadTracepoints(ctx, &deeppb.LoadTracepointRequest{
				Request: &pb.PollRequest{},
			})
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("x-deepql-type", "tracepoint")
		api.ParseMessageToHttp(w, r, span, query.Response)
		return
	}

	if expr.IsCommand() {
		query, err := deepql.NewEngine().ExecuteCommandQuery(ctx, expr, func(ctx context.Context, request *deepql.CommandRequest) (*deeppb.LoadTracepointResponse, error) {
			if request.LoadRequest != nil {
				return ta.client.LoadTracepoints(ctx, request.LoadRequest)
			}
			if request.DeleteRequest != nil {
				_, err := ta.client.DeleteTracepoint(ctx, request.DeleteRequest)
				if err != nil {
					return nil, err
				}
				return ta.client.LoadTracepoints(ctx, &deeppb.LoadTracepointRequest{
					Request: &pb.PollRequest{},
				})
			}
			return nil, errors.New("unknown command request")
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("x-deepql-type", "tracepoint")
		api.ParseMessageToHttp(w, r, span, query.Response)
		return
	}

	http.Error(w, "unsupported query operation", http.StatusBadRequest)
}

func (ta *TracepointAPI) LoadTracepointHandler(w http.ResponseWriter, r *http.Request) {
	// trying to split GET and POST in the server handler with .Methods() doesn't work
	// so if we are a POST then pass to CreateTracepointHandler
	if strings.ToLower(r.Method) == "post" {
		ta.CreateTracepointHandler(w, r)
		return
	}

	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(ta.cfg.LoadTracepoint.Timeout))
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "TracepointAPI.LoadTracepoint")
	defer span.Finish()

	req, err := ta.parseLoadRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tracepoints, err := ta.client.LoadTracepoints(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	api.ParseMessageToHttp(w, r, span, tracepoints.Response)
}

func (ta *TracepointAPI) DeleteTracepointHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(ta.cfg.LoadTracepoint.Timeout))
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "TracepointAPI.DeleteTracepoint")
	defer span.Finish()

	req, err := ta.parseDeleteRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tracepoints, err := ta.client.DeleteTracepoint(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	api.ParseMessageToHttp(w, r, span, tracepoints)
}

func (ta *TracepointAPI) CreateTracepointHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(ta.cfg.LoadTracepoint.Timeout))
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "TracepointAPI.CreateTracepoint")
	defer span.Finish()

	req, err := ta.parseCreateRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tracepoints, err := ta.client.CreateTracepoint(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	api.ParseMessageToHttp(w, r, span, tracepoints)
}

func (ta *TracepointAPI) parseLoadRequest(r *http.Request) (*deeppb.LoadTracepointRequest, error) {
	query := r.URL.Query()
	ts := uint64(time.Now().UnixNano())
	ch := ""
	var atts []*cp.KeyValue
	for key, val := range query {
		switch key {
		case "ts":
			conv, err := strconv.Atoi(val[0])
			if err != nil {
				return nil, err
			}
			ts = uint64(conv)
		case "hash":
			ch = val[0]
		default:
			atts = append(atts, &cp.KeyValue{
				Key:   key,
				Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: val[0]}},
			})
		}
	}

	return &deeppb.LoadTracepointRequest{Request: &pb.PollRequest{
		TsNanos:     ts,
		CurrentHash: ch,
		Resource: &rp.Resource{
			Attributes: atts,
		},
	}}, nil
}

func (ta *TracepointAPI) parseCreateRequest(r *http.Request) (*deeppb.CreateTracepointRequest, error) {
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(r.Body)

	var bodyTp deeppb.CreateTracepointRequest
	err := jsonpb.Unmarshal(r.Body, &bodyTp)
	if err != nil {
		return nil, err
	}

	bodyTp.Tracepoint.ID = uuid.New().String()

	return &bodyTp, nil
}

func (ta *TracepointAPI) parseDeleteRequest(r *http.Request) (*deeppb.DeleteTracepointRequest, error) {
	vars := mux.Vars(r)
	tpID, ok := vars[api.URLParamTracepointID]
	if !ok {
		return nil, fmt.Errorf("please provide a tracepoint ID")
	}

	return &deeppb.DeleteTracepointRequest{TracepointID: tpID}, nil
}

func (ta *TracepointAPI) CreateAndRegisterWorker(handler http.Handler) error {
	querierWorker, err := worker.NewQuerierWorker(
		ta.cfg.Worker.Config,
		httpgrpc_server.NewServer(handler),
		ta.log,
		nil,
		func(frontendClient frontendv1pb.FrontendClient, ctx context.Context) (frontendv1pb.Frontend_ProcessClient, error) {
			return frontendClient.ProcessTracepoint(ctx)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create frontend worker: %w", err)
	}

	return ta.RegisterSubservices(querierWorker)
}

func (ta *TracepointAPI) RegisterSubservices(s ...services.Service) error {
	var err error
	ta.subServices, err = services.NewManager(s...)
	ta.subServicesWatcher = services.NewFailureWatcher()
	ta.subServicesWatcher.WatchManager(ta.subServices)
	return err
}
