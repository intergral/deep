package tracepoint

import (
	"context"
	"fmt"
	"github.com/grafana/dskit/ring"
	"github.com/grafana/dskit/services"
	"github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/modules/tracepoint/store"
	"github.com/intergral/deep/pkg/deeppb"
	cp "github.com/intergral/deep/pkg/deeppb/common/v1"
	"github.com/intergral/deep/pkg/util/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/user"
)

const (
	tracepointRingKey = "tpRing"
)

type TPService struct {
	services.Service
	deeppb.UnimplementedTracepointConfigServiceServer

	cfg        Config
	lifecycler *ring.Lifecycler
	store      *store.TPStore
}

func (ts *TPService) Flush() {
	//TODO implement me
	return
}

func (ts *TPService) TransferOut(ctx context.Context) error {
	//TODO implement me
	return nil
}

func New(cfg Config, storeConfig storage.Config, reg prometheus.Registerer) (*TPService, error) {
	newStore, err := store.NewStore(storeConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create new tracepoint store %w", err)
	}

	service := &TPService{
		cfg:   cfg,
		store: newStore,
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
	return nil
}

func (ts *TPService) LoadTracepoints(ctx context.Context, req *deeppb.LoadTracepointRequest) (*deeppb.LoadTracepointResponse, error) {
	orgId, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting org id in Tracepoint.LoadTracepoints")
	}

	attributes := make([]*cp.KeyValue, 0)
	if req.Request.Resource != nil {
		attributes = req.Request.Resource.Attributes
	}

	tpStore, err := ts.store.ForResource(ctx, orgId, attributes)
	if err != nil {
		return nil, err
	}

	return tpStore.ProcessRequest(req)
}

func (ts *TPService) CreateTracepoint(ctx context.Context, req *deeppb.CreateTracepointRequest) (*deeppb.CreateTracepointResponse, error) {
	orgId, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting org id in Tracepoint.LoadTracepoints")
	}

	tpStore, err := ts.store.ForOrg(ctx, orgId)
	if err != nil {
		return nil, err
	}

	err = tpStore.AddTracepoint(req.Tracepoint)

	err = ts.store.Flush(ctx, tpStore)

	return &deeppb.CreateTracepointResponse{}, nil
}

func (ts *TPService) DeleteTracepoint(ctx context.Context, req *deeppb.DeleteTracepointRequest) (*deeppb.DeleteTracepointResponse, error) {
	orgId, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting org id in Tracepoint.LoadTracepoints")
	}

	tpStore, err := ts.store.ForOrg(ctx, orgId)
	if err != nil {
		return nil, err
	}

	err = tpStore.DeleteTracepoint(req.TracepointID)

	err = ts.store.Flush(ctx, tpStore)

	return &deeppb.DeleteTracepointResponse{}, nil
}
