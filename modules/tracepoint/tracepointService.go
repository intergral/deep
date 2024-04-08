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

package tracepoint

import (
	"context"
	"fmt"
	pb "github.com/intergral/deep/pkg/deeppb/poll/v1"
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/deepql"
	"sync"

	gkLog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/ring"
	"github.com/grafana/dskit/services"
	"github.com/intergral/deep/modules/storage"
	tp_store "github.com/intergral/deep/modules/tracepoint/store"
	"github.com/intergral/deep/pkg/deeppb"
	cp "github.com/intergral/deep/pkg/deeppb/common/v1"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// ErrReadOnly is returned when we are shutting down
var ErrReadOnly = errors.New("tracepoint is shutting down")

const (
	tracepointRingKey = "tpRing"
)

type TPService struct {
	services.Service
	deeppb.UnimplementedTracepointConfigServiceServer

	instancesMtx sync.RWMutex

	cfg        Config
	lifecycler *ring.Lifecycler
	store      *tp_store.TPStore
	log        gkLog.Logger
	readonly   bool
}

func (ts *TPService) Flush() {
	err := ts.store.FlushAll(context.Background())
	if err != nil {
		level.Error(ts.log).Log("msg", "error flushing tracepoint store", "err", err)
	}
}

func (ts *TPService) TransferOut(context.Context) error {
	return ring.ErrTransferDisabled
}

// New will create a new TPService that handles reading and writing tracepoint changes to disk
func New(cfg Config, store storage.Store, logger gkLog.Logger, reg prometheus.Registerer) (*TPService, error) {
	newStore, err := tp_store.NewStore(store)
	if err != nil {
		return nil, fmt.Errorf("cannot create new tracepoint store %w", err)
	}

	service := &TPService{
		cfg:   cfg,
		store: newStore,
		log:   logger,
	}

	service.Service = services.NewBasicService(service.starting, service.running, service.stopping)

	lc, err := ring.NewLifecycler(cfg.LifecyclerConfig, service, "tracepoint", cfg.OverrideRingKey, true, log.Logger, prometheus.WrapRegistererWithPrefix("deep_", reg))
	if err != nil {
		return nil, fmt.Errorf("NewLifecycler failed: %w", err)
	}
	service.lifecycler = lc

	return service, nil
}

func (ts *TPService) starting(ctx context.Context) error {
	// Important: we want to keep lifecycler running until we ask it to stop, so we need to give it independent context
	if err := ts.lifecycler.StartAsync(context.Background()); err != nil {
		return fmt.Errorf("failed to start lifecycler: %w", err)
	}
	if err := ts.lifecycler.AwaitRunning(ctx); err != nil {
		return fmt.Errorf("failed to start lifecycle: %w", err)
	}

	return nil
}

func (ts *TPService) running(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	}
}

func (ts *TPService) stopping(_ error) error {
	if err := services.StopAndAwaitTerminated(context.Background(), ts.lifecycler); err != nil {
		level.Warn(log.Logger).Log("msg", "failed to stop tracepoint lifecycler", "err", err)
	}

	ts.setReadOnly()
	return nil
}

func (ts *TPService) setReadOnly() {
	ts.instancesMtx.Lock()
	defer ts.instancesMtx.Unlock()

	ts.readonly = true
}

func (ts *TPService) LoadTracepoints(ctx context.Context, req *deeppb.LoadTracepointRequest) (*deeppb.LoadTracepointResponse, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Tracepoint.LoadTracepoints")
	}

	attributes := make([]*cp.KeyValue, 0)
	if req.Request.Resource != nil {
		attributes = req.Request.Resource.Attributes
	}

	tpStore, err := ts.store.ForResource(ctx, tenantID, attributes)
	if err != nil {
		return nil, err
	}

	return tpStore.ProcessRequest(req)
}

func (ts *TPService) CreateTracepoint(ctx context.Context, req *deeppb.CreateTracepointRequest) (*deeppb.CreateTracepointResponse, error) {
	if ts.readonly {
		return nil, ErrReadOnly
	}
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Tracepoint.LoadTracepoints")
	}

	tpStore, err := ts.store.ForOrg(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	err = tpStore.AddTracepoint(req.Tracepoint)

	err = ts.store.Flush(ctx, tpStore)

	return &deeppb.CreateTracepointResponse{}, nil
}

func (ts *TPService) DeleteTracepoint(ctx context.Context, req *deeppb.DeleteTracepointRequest) (*deeppb.DeleteTracepointResponse, error) {
	if ts.readonly {
		return nil, ErrReadOnly
	}
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Tracepoint.LoadTracepoints")
	}

	tpStore, err := ts.store.ForOrg(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	err = tpStore.DeleteTracepoints(req.TracepointID)

	err = ts.store.Flush(ctx, tpStore)

	return &deeppb.DeleteTracepointResponse{}, nil
}

func (ts *TPService) ExecuteDeepQl(ctx context.Context, req *deeppb.DeepQlRequest) (*deeppb.DeepQlResponse, error) {
	if ts.readonly {
		return nil, ErrReadOnly
	}
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in ExecuteDeepQl")
	}

	orgStore, err := ts.store.ForOrg(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	expr, err := deepql.ParseString(req.Query)
	if err != nil {
		return nil, err
	}
	engine := deepql.NewEngine()

	if expr.IsTrigger() {
		affect, all, err := engine.ExecuteTriggerQuery(ctx, expr, func(ctx context.Context, request *deeppb.CreateTracepointRequest) (*tp.TracePointConfig, []*tp.TracePointConfig, error) {
			tracepoint, err := ts.CreateTracepoint(ctx, request)
			if err != nil {
				return nil, nil, err
			}
			tracepoints, err := ts.LoadTracepoints(ctx, &deeppb.LoadTracepointRequest{Request: &pb.PollRequest{}})
			if err != nil {
				return nil, nil, err
			}
			return tracepoint.Created, tracepoints.Response.Response, nil
		})
		if err != nil {
			return nil, err
		}
		return &deeppb.DeepQlResponse{
			All: all,
			Affected: []*tp.TracePointConfig{
				affect,
			},
			Type: "create",
		}, nil
	}

	if expr.IsCommand() {
		affect, all, commandType, err := engine.ExecuteCommandQuery(ctx, expr, func(ctx context.Context, request *deepql.CommandRequest) ([]*tp.TracePointConfig, []*tp.TracePointConfig, string, error) {
			return ts.runQuery(ctx, orgStore, request)
		})
		if err != nil {
			return nil, err
		}
		return &deeppb.DeepQlResponse{
			All:      all,
			Affected: affect,
			Type:     commandType,
		}, nil
	}

	return nil, errors.New("invalid query expression")
}

func (ts *TPService) runQuery(ctx context.Context, store tp_store.OrgTPStore, request *deepql.CommandRequest) ([]*tp.TracePointConfig, []*tp.TracePointConfig, string, error) {
	tps, err := store.FindTracepoints(request)
	if err != nil {
		return nil, nil, request.Command, err
	}

	if request.Command == deepql.DeleteType {
		ids := make([]string, len(tps))
		for i, t := range tps {
			ids[i] = t.ID
		}
		err := store.DeleteTracepoints(ids...)
		if err != nil {
			return nil, nil, request.Command, err
		}
	}

	all, err := store.LoadAll()
	if err != nil {
		return nil, nil, request.Command, err
	}
	if len(tps) == 0 {
		return nil, all, request.Command, nil
	}

	return tps, all, request.Command, nil
}
