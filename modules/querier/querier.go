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

package querier

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/intergral/deep/modules/frontend/v1/frontendv1pb"
	"github.com/intergral/deep/pkg/deeppb"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/deepql"

	"github.com/cristalhq/hedgedhttp"
	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/grafana/dskit/ring"
	ring_client "github.com/grafana/dskit/ring/client"
	"github.com/grafana/dskit/services"
	"github.com/opentracing/opentracing-go"
	ot_log "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	httpgrpc_server "github.com/weaveworks/common/httpgrpc/server"
	"go.uber.org/multierr"
	"golang.org/x/sync/semaphore"

	ingester_client "github.com/intergral/deep/modules/ingester/client"
	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/pkg/api"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/hedgedmetrics"
	"github.com/intergral/deep/pkg/search"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/log"
	"github.com/intergral/deep/pkg/validation"
	"github.com/intergral/deep/pkg/worker"
)

var (
	metricIngesterClients = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "deep",
		Name:      "querier_ingester_clients",
		Help:      "The current number of ingester clients.",
	})
	metricEndpointDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "deep",
		Name:      "querier_external_endpoint_duration_seconds",
		Help:      "The duration of the external endpoints.",
		Buckets:   prometheus.DefBuckets,
	}, []string{"endpoint"})
	metricExternalHedgedRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "deep",
			Name:      "querier_external_endpoint_hedged_roundtrips_total",
			Help:      "Total number of hedged external requests. Registered as a gauge for code sanity. This is a counter.",
		},
	)
)

// Querier handlers queries.
type Querier struct {
	services.Service

	cfg    Config
	ring   ring.ReadRing
	pool   *ring_client.Pool
	engine *deepql.Engine
	store  storage.Store
	limits *overrides.Overrides

	searchClient     *http.Client
	searchPreferSelf *semaphore.Weighted

	subservices        *services.Manager
	subservicesWatcher *services.FailureWatcher
}

type responseFromIngesters struct {
	addr     string
	response interface{}
}

// New makes a new Querier.
func New(cfg Config, clientCfg ingester_client.Config, ring ring.ReadRing, store storage.Store, limits *overrides.Overrides) (*Querier, error) {
	factory := func(addr string) (ring_client.PoolClient, error) {
		return ingester_client.New(addr, clientCfg)
	}

	q := &Querier{
		cfg:  cfg,
		ring: ring,
		pool: ring_client.NewPool("querier_pool",
			clientCfg.PoolConfig,
			ring_client.NewRingServiceDiscovery(ring),
			factory,
			metricIngesterClients,
			log.Logger),
		engine:           deepql.NewEngine(),
		store:            store,
		limits:           limits,
		searchPreferSelf: semaphore.NewWeighted(int64(cfg.Search.PreferSelf)),
		searchClient:     http.DefaultClient,
	}

	//
	if cfg.Search.HedgeRequestsAt != 0 {
		var err error
		var stats *hedgedhttp.Stats
		q.searchClient, stats, err = hedgedhttp.NewClientAndStats(cfg.Search.HedgeRequestsAt, cfg.Search.HedgeRequestsUpTo, http.DefaultClient)
		if err != nil {
			return nil, err
		}
		hedgedmetrics.Publish(stats, metricExternalHedgedRequests)
	}

	q.Service = services.NewBasicService(q.starting, q.running, q.stopping)
	return q, nil
}

func (q *Querier) CreateAndRegisterWorker(handler http.Handler) error {
	q.cfg.Worker.MaxConcurrentRequests = q.cfg.MaxConcurrentQueries
	querierWorker, err := worker.NewQuerierWorker(
		q.cfg.Worker.Config,
		httpgrpc_server.NewServer(handler),
		log.Logger,
		nil,
		func(client frontendv1pb.FrontendClient, ctx context.Context) (frontendv1pb.Frontend_ProcessClient, error) {
			return client.Process(ctx)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create frontend worker: %w", err)
	}

	return q.RegisterSubservices(querierWorker, q.pool)
}

func (q *Querier) RegisterSubservices(s ...services.Service) error {
	var err error
	q.subservices, err = services.NewManager(s...)
	q.subservicesWatcher = services.NewFailureWatcher()
	q.subservicesWatcher.WatchManager(q.subservices)
	return err
}

func (q *Querier) starting(ctx context.Context) error {
	if q.subservices != nil {
		err := services.StartManagerAndAwaitHealthy(ctx, q.subservices)
		if err != nil {
			return fmt.Errorf("failed to start subservices %w", err)
		}
	}

	return nil
}

func (q *Querier) running(ctx context.Context) error {
	if q.subservices != nil {
		select {
		case <-ctx.Done():
			return nil
		case err := <-q.subservicesWatcher.Chan():
			return fmt.Errorf("subservices failed %w", err)
		}
	} else {
		<-ctx.Done()
	}
	return nil
}

func (q *Querier) stopping(_ error) error {
	if q.subservices != nil {
		return services.StopManagerAndAwaitStopped(context.Background(), q.subservices)
	}
	return nil
}

func (q *Querier) FindSnapshotByID(ctx context.Context, req *deeppb.SnapshotByIDRequest, timeStart int64, timeEnd int64) (*deeppb.SnapshotByIDResponse, error) {
	if !validation.ValidSnapshotID(req.ID) {
		return nil, fmt.Errorf("invalid snapshot id")
	}

	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Querier.FindSnapshotByID")
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "Querier.FindSnapshotByID")
	defer span.Finish()

	span.SetTag("queryMode", req.QueryMode)
	var foundSnap *deep_tp.Snapshot = nil
	if req.QueryMode == QueryModeIngesters || req.QueryMode == QueryModeAll {
		var replicationSet ring.ReplicationSet
		var err error
		if q.cfg.QueryRelevantIngesters {
			tenantKey := util.TokenFor(tenantID, req.ID)
			replicationSet, err = q.ring.Get(tenantKey, ring.Read, nil, nil, nil)
		} else {
			replicationSet, err = q.ring.GetReplicationSetForOperation(ring.Read)
		}
		if err != nil {
			return nil, errors.Wrap(err, "error finding ingesters in Querier.SnapshotByIdHandler")
		}

		span.LogFields(ot_log.String("msg", "searching ingesters"))

		forEachFunc := func(funcCtx context.Context, client deeppb.QuerierServiceClient) (interface{}, error) {
			return client.FindSnapshotByID(funcCtx, req)
		}

		// get responses from all ingesters in parallel
		responses, err := q.forGivenIngesters(ctx, replicationSet, forEachFunc)
		if err != nil {
			return nil, errors.Wrap(err, "error querying ingesters in Querier.FindSnapshotByID")
		}

		found := false
		for _, r := range responses {
			t := r.response.(*deeppb.SnapshotByIDResponse).Snapshot
			if t != nil {
				found = true
				foundSnap = t
			}
		}
		span.LogFields(ot_log.String("msg", "done searching ingesters"), ot_log.Bool("found", found))
	}

	if foundSnap != nil {
		return &deeppb.SnapshotByIDResponse{
			Snapshot: foundSnap,
			Metrics: &deeppb.SnapshotByIDMetrics{
				FailedBlocks: 0,
			},
		}, nil
	}
	var failedBlocks int
	if req.QueryMode == QueryModeBlocks || req.QueryMode == QueryModeAll {
		span.LogFields(ot_log.String("msg", "searching store"))
		span.LogFields(ot_log.String("timeStart", fmt.Sprint(timeStart)))
		span.LogFields(ot_log.String("timeEnd", fmt.Sprint(timeEnd)))
		findResults, blockErrs, err := q.store.FindSnapshot(ctx, tenantID, req.ID, req.BlockStart, req.BlockEnd, timeStart, timeEnd)
		if err != nil {
			retErr := errors.Wrap(err, "error querying store in Querier.SnapshotByIdHandler")
			ot_log.Error(retErr)
			return nil, retErr
		}

		if blockErrs != nil {
			failedBlocks = len(blockErrs)
			span.LogFields(
				ot_log.Int("failedBlocks", failedBlocks),
				ot_log.Error(multierr.Combine(blockErrs...)),
			)
			_ = level.Warn(log.Logger).Log("msg", fmt.Sprintf("failed to query %d blocks", failedBlocks), "blockErrs", multierr.Combine(blockErrs...))
		}

		span.LogFields(
			ot_log.String("msg", "done searching store"),
			ot_log.Bool("foundSnapshots", findResults != nil))

		if findResults != nil {
			return &deeppb.SnapshotByIDResponse{Snapshot: findResults, Metrics: &deeppb.SnapshotByIDMetrics{FailedBlocks: uint32(failedBlocks)}}, nil
		}
	}

	return &deeppb.SnapshotByIDResponse{}, nil
}

// forGivenIngesters runs f, in parallel, for given ingesters
func (q *Querier) forGivenIngesters(ctx context.Context, replicationSet ring.ReplicationSet, f func(ctx context.Context, client deeppb.QuerierServiceClient) (interface{}, error)) ([]responseFromIngesters, error) {
	if ctx.Err() != nil {
		_ = level.Debug(log.Logger).Log("foreGivenIngesters context error", "ctx.Err()", ctx.Err().Error())
		return nil, ctx.Err()
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "Querier.forGivenIngesters")
	defer span.Finish()

	doFunc := func(funcCtx context.Context, ingester *ring.InstanceDesc) (interface{}, error) {
		if funcCtx.Err() != nil {
			_ = level.Warn(log.Logger).Log("funcCtx.Err()", funcCtx.Err().Error())
			return nil, funcCtx.Err()
		}

		client, err := q.pool.GetClientFor(ingester.Addr)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get client for %s", ingester.Addr))
		}

		resp, err := f(funcCtx, client.(deeppb.QuerierServiceClient))
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to execute f() for %s", ingester.Addr))
		}

		return responseFromIngesters{ingester.Addr, resp}, nil
	}

	results, err := replicationSet.Do(ctx, q.cfg.ExtraQueryDelay, doFunc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get response from ingesters")
	}

	responses := make([]responseFromIngesters, 0, len(results))
	for _, result := range results {
		responses = append(responses, result.(responseFromIngesters))
	}

	return responses, nil
}

func (q *Querier) SearchRecent(ctx context.Context, req *deeppb.SearchRequest) (*deeppb.SearchResponse, error) {
	_, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Querier.Search")
	}

	replicationSet, err := q.ring.GetReplicationSetForOperation(ring.Read)
	if err != nil {
		return nil, errors.Wrap(err, "error finding ingesters in Querier.Search")
	}

	responses, err := q.forGivenIngesters(ctx, replicationSet, func(ctx context.Context, client deeppb.QuerierServiceClient) (interface{}, error) {
		return client.SearchRecent(ctx, req)
	})
	if err != nil {
		return nil, errors.Wrap(err, "error querying ingesters in Querier.Search")
	}

	return q.postProcessIngesterSearchResults(req, responses), nil
}

func (q *Querier) SearchTags(ctx context.Context, req *deeppb.SearchTagsRequest) (*deeppb.SearchTagsResponse, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Querier.SearchTags")
	}

	limit := q.limits.MaxBytesPerTagValuesQuery(tenantID)
	distinctValues := util.NewDistinctStringCollector(limit)

	// Get results from all ingesters
	replicationSet, err := q.ring.GetReplicationSetForOperation(ring.Read)
	if err != nil {
		return nil, errors.Wrap(err, "error finding ingesters in Querier.SearchTags")
	}
	lookupResults, err := q.forGivenIngesters(ctx, replicationSet, func(ctx context.Context, client deeppb.QuerierServiceClient) (interface{}, error) {
		return client.SearchTags(ctx, req)
	})
	if err != nil {
		return nil, errors.Wrap(err, "error querying ingesters in Querier.SearchTags")
	}
	for _, resp := range lookupResults {
		for _, res := range resp.response.(*deeppb.SearchTagsResponse).TagNames {
			distinctValues.Collect(res)
		}
	}

	if distinctValues.Exceeded() {
		level.Warn(log.Logger).Log("msg", "size of tags in instance exceeded limit, reduce cardinality or size of tags", "tenantID", tenantID, "limit", limit, "total", distinctValues.TotalDataSize())
	}

	resp := &deeppb.SearchTagsResponse{
		TagNames: distinctValues.Strings(),
	}

	return resp, nil
}

func (q *Querier) SearchTagValues(ctx context.Context, req *deeppb.SearchTagValuesRequest) (*deeppb.SearchTagValuesResponse, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Querier.SearchTagValues")
	}

	limit := q.limits.MaxBytesPerTagValuesQuery(tenantID)
	distinctValues := util.NewDistinctStringCollector(limit)

	// Virtual tags values. Get these first.
	for _, v := range search.GetVirtualTagValues(req.TagName) {
		distinctValues.Collect(v)
	}

	// Get results from all ingesters
	replicationSet, err := q.ring.GetReplicationSetForOperation(ring.Read)
	if err != nil {
		return nil, errors.Wrap(err, "error finding ingesters in Querier.SearchTagValues")
	}
	lookupResults, err := q.forGivenIngesters(ctx, replicationSet, func(ctx context.Context, client deeppb.QuerierServiceClient) (interface{}, error) {
		return client.SearchTagValues(ctx, req)
	})
	if err != nil {
		return nil, errors.Wrap(err, "error querying ingesters in Querier.SearchTagValues")
	}
	for _, resp := range lookupResults {
		for _, res := range resp.response.(*deeppb.SearchTagValuesResponse).TagValues {
			distinctValues.Collect(res)
		}
	}

	if distinctValues.Exceeded() {
		level.Warn(log.Logger).Log("msg", "size of tag values in instance exceeded limit, reduce cardinality or size of tags", "tag", req.TagName, "tenantID", tenantID, "limit", limit, "total", distinctValues.TotalDataSize())
	}

	resp := &deeppb.SearchTagValuesResponse{
		TagValues: distinctValues.Strings(),
	}

	return resp, nil
}

func (q *Querier) SearchTagValuesV2(ctx context.Context, req *deeppb.SearchTagValuesRequest) (*deeppb.SearchTagValuesV2Response, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Querier.SearchTagValues")
	}

	limit := q.limits.MaxBytesPerTagValuesQuery(tenantID)
	distinctValues := util.NewDistinctValueCollector(limit, func(v *deeppb.TagValue) int { return len(v.Type) + len(v.Value) })

	// Virtual tags values. Get these first.
	for _, v := range search.GetVirtualTagValuesV2(req.TagName) {
		distinctValues.Collect(v)
	}

	// with v2 search we can confidently bail if GetVirtualTagValuesV2 gives us any hits. this doesn't work
	// in v1 search b/c intrinsic tags like "status" are conflated with attributes named "status"
	if distinctValues.TotalDataSize() > 0 {
		return valuesToV2Response(distinctValues), nil
	}

	// Get results from all ingesters
	replicationSet, err := q.ring.GetReplicationSetForOperation(ring.Read)
	if err != nil {
		return nil, errors.Wrap(err, "error finding ingesters in Querier.SearchTagValues")
	}
	lookupResults, err := q.forGivenIngesters(ctx, replicationSet, func(ctx context.Context, client deeppb.QuerierServiceClient) (interface{}, error) {
		return client.SearchTagValuesV2(ctx, req)
	})
	if err != nil {
		return nil, errors.Wrap(err, "error querying ingesters in Querier.SearchTagValues")
	}
	for _, resp := range lookupResults {
		for _, res := range resp.response.(*deeppb.SearchTagValuesV2Response).TagValues {
			distinctValues.Collect(res)
		}
	}

	if distinctValues.Exceeded() {
		level.Warn(log.Logger).Log("msg", "size of tag values in instance exceeded limit, reduce cardinality or size of tags", "tag", req.TagName, "tenantID", tenantID, "limit", limit, "total", distinctValues.TotalDataSize())
	}

	return valuesToV2Response(distinctValues), nil
}

func valuesToV2Response(distinctValues *util.DistinctValueCollector[*deeppb.TagValue]) *deeppb.SearchTagValuesV2Response {
	resp := &deeppb.SearchTagValuesV2Response{}
	for _, v := range distinctValues.Values() {
		v2 := v
		resp.TagValues = append(resp.TagValues, v2)
	}

	return resp
}

// SearchBlock searches the specified subset of the block for the passed tags.
func (q *Querier) SearchBlock(ctx context.Context, req *deeppb.SearchBlockRequest) (*deeppb.SearchResponse, error) {
	// if we have no external configuration always search in the querier
	if len(q.cfg.Search.ExternalEndpoints) == 0 {
		return q.internalSearchBlock(ctx, req)
	}

	// if we have external configuration but there's an open slot locally then search in the querier
	if q.searchPreferSelf.TryAcquire(1) {
		defer q.searchPreferSelf.Release(1)
		return q.internalSearchBlock(ctx, req)
	}

	// proxy externally!
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id for externalEndpoint")
	}
	maxBytes := q.limits.MaxBytesPerSnapshot(tenantID)

	endpoint := q.cfg.Search.ExternalEndpoints[rand.Intn(len(q.cfg.Search.ExternalEndpoints))]
	return q.searchExternalEndpoint(ctx, endpoint, maxBytes, req)
}

func (q *Querier) internalSearchBlock(ctx context.Context, req *deeppb.SearchBlockRequest) (*deeppb.SearchResponse, error) {
	tenantID, err := util.ExtractTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error extracting tenant id in Querier.BackendSearch")
	}

	blockID, err := uuid.Parse(req.BlockID)
	if err != nil {
		return nil, err
	}

	enc, err := backend.ParseEncoding(req.Encoding)
	if err != nil {
		return nil, err
	}

	meta := &backend.BlockMeta{
		Version:       req.Version,
		TenantID:      tenantID,
		Encoding:      enc,
		Size:          req.Size,
		IndexPageSize: req.IndexPageSize,
		TotalRecords:  req.TotalRecords,
		BlockID:       blockID,
		DataEncoding:  req.DataEncoding,
		FooterSize:    req.FooterSize,
	}

	opts := common.DefaultSearchOptions()
	opts.StartPage = int(req.StartPage)
	opts.TotalPages = int(req.PagesToSearch)
	opts.MaxBytes = q.limits.MaxBytesPerSnapshot(tenantID)

	if api.IsDeepQLQuery(req.SearchReq) {
		fetcher := deepql.NewSnapshotResultFetcherWrapper(func(ctx context.Context, req deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
			return q.store.Fetch(ctx, meta, req, opts)
		})

		return q.engine.Execute(ctx, req.SearchReq, fetcher)
	}

	return q.store.Search(ctx, meta, req.SearchReq, opts)
}

func (q *Querier) postProcessIngesterSearchResults(req *deeppb.SearchRequest, rr []responseFromIngesters) *deeppb.SearchResponse {
	response := &deeppb.SearchResponse{
		Metrics: &deeppb.SearchMetrics{},
	}

	snapshots := map[string]*deeppb.SnapshotSearchMetadata{}

	for _, r := range rr {
		sr := r.response.(*deeppb.SearchResponse)
		for _, t := range sr.Snapshots {
			// Just simply take first result for each snapshot
			if _, ok := snapshots[t.SnapshotID]; !ok {
				snapshots[t.SnapshotID] = t
			}
		}
		if sr.Metrics != nil {
			response.Metrics.InspectedBytes += sr.Metrics.InspectedBytes
			response.Metrics.InspectedSnapshots += sr.Metrics.InspectedSnapshots
			response.Metrics.InspectedBlocks += sr.Metrics.InspectedBlocks
			response.Metrics.SkippedBlocks += sr.Metrics.SkippedBlocks
		}
	}

	for _, t := range snapshots {
		response.Snapshots = append(response.Snapshots, t)
	}

	// Sort and limit results
	sort.Slice(response.Snapshots, func(i, j int) bool {
		return response.Snapshots[i].StartTimeUnixNano > response.Snapshots[j].StartTimeUnixNano
	})
	if req.Limit != 0 && int(req.Limit) < len(response.Snapshots) {
		response.Snapshots = response.Snapshots[:req.Limit]
	}

	return response
}

func (q *Querier) searchExternalEndpoint(ctx context.Context, externalEndpoint string, maxBytes int, searchReq *deeppb.SearchBlockRequest) (*deeppb.SearchResponse, error) {
	req, err := http.NewRequest(http.MethodGet, externalEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("external endpoint failed to make new request: %w", err)
	}
	req, err = api.BuildSearchBlockRequest(req, searchReq)
	if err != nil {
		return nil, fmt.Errorf("external endpoint failed to build search block request: %w", err)
	}
	req = api.AddServerlessParams(req, maxBytes)
	err = util.InjectTenantIDIntoHTTPRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("external endpoint failed to inject tenant id: %w", err)
	}
	start := time.Now()
	resp, err := q.searchClient.Do(req)
	metricEndpointDuration.WithLabelValues(externalEndpoint).Observe(time.Since(start).Seconds())
	if err != nil {
		return nil, fmt.Errorf("external endpoint failed to call http: %s, %w", externalEndpoint, err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("external endpoint failed to read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("external endpoint returned %d, %s", resp.StatusCode, string(body))
	}
	var searchResp deeppb.SearchResponse
	err = jsonpb.Unmarshal(bytes.NewReader(body), &searchResp)
	if err != nil {
		return nil, fmt.Errorf("external endpoint failed to unmarshal body: %s, %w", string(body), err)
	}
	return &searchResp, nil
}
