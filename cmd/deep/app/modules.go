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

package app

import (
	"fmt"
	"io"
	"net/http"
	"path"

	"github.com/go-kit/log/level"
	"github.com/grafana/dskit/dns"
	"github.com/grafana/dskit/kv/codec"
	"github.com/grafana/dskit/kv/memberlist"
	"github.com/grafana/dskit/modules"
	"github.com/grafana/dskit/ring"
	"github.com/grafana/dskit/services"
	"github.com/intergral/deep/modules/compactor"
	"github.com/intergral/deep/modules/distributor"
	"github.com/intergral/deep/modules/frontend"
	frontend_v1pb "github.com/intergral/deep/modules/frontend/v1/frontendv1pb"
	"github.com/intergral/deep/modules/generator"
	"github.com/intergral/deep/modules/ingester"
	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/modules/querier"
	deep_storage "github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/modules/tracepoint"
	tpapi "github.com/intergral/deep/modules/tracepoint/api"
	"github.com/intergral/deep/modules/tracepoint/client"
	"github.com/intergral/deep/pkg/api"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/azure"
	"github.com/intergral/deep/pkg/deepdb/backend/gcs"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/backend/s3"
	"github.com/intergral/deep/pkg/deeppb"
	deep_ring "github.com/intergral/deep/pkg/ring"
	"github.com/intergral/deep/pkg/usagestats"
	"github.com/intergral/deep/pkg/util/log"
	util_log "github.com/intergral/deep/pkg/util/log"
	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/weaveworks/common/middleware"
	"github.com/weaveworks/common/server"
)

// The various modules that make up deep.
const (
	Ring                 string = "ring"
	TPRing               string = "tp-ring"
	MetricsGeneratorRing string = "metrics-generator-ring"
	Overrides            string = "overrides"
	Server               string = "server"
	InternalServer       string = "internal-server"
	Distributor          string = "distributor"
	Ingester             string = "ingester"
	TracepointClient     string = "tracepoint-client"
	Tracepoint           string = "tracepoint"
	TracepointAPI        string = "tracepoint-api"
	MetricsGenerator     string = "metrics-generator"
	Querier              string = "querier"
	QueryFrontend        string = "query-frontend"
	Compactor            string = "compactor"
	Store                string = "store"
	MemberlistKV         string = "memberlist-kv"
	SingleBinary         string = "all"
	ScalableSingleBinary string = "scalable-single-binary"
	UsageReport          string = "usage-report"
)

// initServer will create the server service that provides the listener and handler for HTTP and
// GRPC endpoints. Each module can then register new endpoints as they are needed.
func (t *App) initServer() (services.Service, error) {
	t.cfg.Server.MetricsNamespace = metricsNamespace
	t.cfg.Server.ExcludeRequestInLog = true

	prometheus.MustRegister(&t.cfg)

	if t.cfg.EnableGoRuntimeMetrics {
		// unregister default Go collector
		prometheus.Unregister(collectors.NewGoCollector())
		// register Go collector with all available runtime metrics
		prometheus.MustRegister(collectors.NewGoCollector(
			collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll),
		))
	}

	DisableSignalHandling(&t.cfg.Server)

	deepServer, err := server.New(t.cfg.Server)
	if err != nil {
		return nil, fmt.Errorf("failed to create server %w", err)
	}

	servicesToWaitFor := func() []services.Service {
		svs := []services.Service(nil)
		for m, s := range t.serviceMap {
			// Server should not wait for itself.
			if m != Server && m != InternalServer {
				svs = append(svs, s)
			}
		}
		return svs
	}

	t.Server = deepServer
	s := NewServerService(deepServer, servicesToWaitFor)

	return s, nil
}

func (t *App) initInternalServer() (services.Service, error) {
	if !t.cfg.InternalServer.Enable {
		return services.NewIdleService(nil, nil), nil
	}

	DisableSignalHandling(&t.cfg.InternalServer.Config)
	serv, err := server.New(t.cfg.InternalServer.Config)
	if err != nil {
		return nil, err
	}

	servicesToWaitFor := func() []services.Service {
		svs := []services.Service(nil)
		for m, s := range t.serviceMap {
			// Server should not wait for itself or the server
			if m != InternalServer && m != Server {
				svs = append(svs, s)
			}
		}
		return svs
	}

	t.InternalServer = serv
	s := NewServerService(t.InternalServer, servicesToWaitFor)

	return s, nil
}

func (t *App) initRing() (services.Service, error) {
	newRing, err := deep_ring.New(t.cfg.Ingester.LifecyclerConfig.RingConfig, "ingester", t.cfg.Ingester.OverrideRingKey, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to create ring %w", err)
	}
	t.ring = newRing

	t.Server.HTTP.Handle("/ingester/ring", t.ring)

	return t.ring, nil
}

func (t *App) initTPRing() (services.Service, error) {
	newRing, err := deep_ring.New(t.cfg.Tracepoint.LifecyclerConfig.RingConfig, "tracepoints", t.cfg.Tracepoint.OverrideRingKey, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to create ring %w", err)
	}
	t.tpRing = newRing

	t.Server.HTTP.Handle("/tracepoint/ring", t.tpRing)

	return t.tpRing, nil
}

func (t *App) initGeneratorRing() (services.Service, error) {
	generatorRing, err := deep_ring.New(t.cfg.Generator.Ring.ToRingConfig(), "metrics-generator", generator.RingKey, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics-generator ring %w", err)
	}
	t.generatorRing = generatorRing

	t.Server.HTTP.Handle("/metrics-generator/ring", t.generatorRing)

	return t.generatorRing, nil
}

func (t *App) initOverrides() (services.Service, error) {
	overridesService, err := overrides.NewOverrides(t.cfg.LimitsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create overrides %w", err)
	}
	t.overrides = overridesService

	prometheus.MustRegister(&t.cfg.LimitsConfig)

	if t.cfg.LimitsConfig.PerTenantOverrideConfig != "" {
		prometheus.MustRegister(t.overrides)
	}

	return t.overrides, nil
}

// initDistributor will create the distributor that accepts new snapshots from clients
// The distributor will then ensure the format of the snapshot and forward it to the ingester
func (t *App) initDistributor() (services.Service, error) {
	// todo: make ingester client a module instead of passing the config everywhere
	newDistributor, err := distributor.New(t.cfg.Distributor, t.tpClient, t.cfg.IngesterClient, t.ReceiverMiddleware, t.ring, t.cfg.GeneratorClient, t.generatorRing, t.overrides, log.Logger, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to create distributor %w", err)
	}
	t.distributor = newDistributor

	if newDistributor.DistributorRing != nil {
		t.Server.HTTP.Handle("/distributor/ring", t.distributor.DistributorRing)
	}

	return t.distributor, nil
}

// initTracepointClient will create a common client that can be used by service to connect to the tracepointService
func (t *App) initTracepointClient() (services.Service, error) {
	tpClient, err := client.New(t.cfg.Tracepoint.Client, t.tpRing, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracepoint ring client %w", err)
	}

	t.tpClient = tpClient
	return t.tpClient, nil
}

// initTracepoint creates the service that will actually deal with storing the tracepoint configs
func (t *App) initTracepoint() (services.Service, error) {
	t.cfg.Tracepoint.LifecyclerConfig.ListenPort = t.cfg.Server.GRPCListenPort
	newTracepointService, err := tracepoint.New(t.cfg.Tracepoint, t.store, log.Logger, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracepoint config service: %w", err)
	}
	t.tracepointService = newTracepointService

	deeppb.RegisterTracepointConfigServiceServer(t.Server.GRPC, t.tracepointService)

	return t.tracepointService, nil
}

// initTracepointAPI is the main handler for changes to tracepoint configs
// here we accept the requests from HTTP and process the changes via tpClient
func (t *App) initTracepointAPI() (services.Service, error) {
	// validate worker config
	// if we're not in single binary mode and worker address is not specified - bail
	if t.cfg.Target != SingleBinary && t.cfg.Tracepoint.API.Worker.FrontendAddress == "" {
		return nil, fmt.Errorf("frontend worker address not specified")
	} else if t.cfg.Target == SingleBinary {
		// if we're in single binary mode with no worker address specified, register default endpoint
		if t.cfg.Tracepoint.API.Worker.FrontendAddress == "" {
			t.cfg.Tracepoint.API.Worker.FrontendAddress = fmt.Sprintf("127.0.0.1:%d", t.cfg.Server.GRPCListenPort)
			level.Warn(log.Logger).Log("msg", "Worker address is empty in single binary mode. Attempting automatic worker configuration. If queries are unresponsive consider configuring the worker explicitly.", "address", t.cfg.Tracepoint.API.Worker.FrontendAddress)
		}
	}
	tracepointAPI, err := tpapi.NewTracepointAPI(t.cfg.Tracepoint.API, t.tpClient, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracepoint API %w", err)
	}

	t.tracepointAPI = tracepointAPI

	// trying to split GET and POST in the server handler with .Methods() doesn't work,
	// so we do it ourselves in the handler
	loadHandler := t.HTTPAuthMiddleware.Wrap(http.HandlerFunc(t.tracepointAPI.LoadTracepointHandler))
	t.Server.HTTP.Handle(path.Join(api.PathPrefixTracepoints, addHTTPAPIPrefix(&t.cfg, api.PathTracepoints)), loadHandler)

	deleteHandler := t.HTTPAuthMiddleware.Wrap(http.HandlerFunc(t.tracepointAPI.DeleteTracepointHandler))
	t.Server.HTTP.Handle(path.Join(api.PathPrefixTracepoints, addHTTPAPIPrefix(&t.cfg, api.PathDeleteTracepoint)), deleteHandler)

	return t.tracepointAPI, t.tracepointAPI.CreateAndRegisterWorker(t.Server.HTTPServer.Handler)
}

func (t *App) initIngester() (services.Service, error) {
	t.cfg.Ingester.LifecyclerConfig.ListenPort = t.cfg.Server.GRPCListenPort
	newIngester, err := ingester.New(t.cfg.Ingester, t.store, t.overrides, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to create ingester: %w", err)
	}
	t.ingester = newIngester

	deeppb.RegisterIngesterServiceServer(t.Server.GRPC, t.ingester)
	deeppb.RegisterQuerierServiceServer(t.Server.GRPC, t.ingester)
	t.Server.HTTP.Path("/flush").Handler(http.HandlerFunc(t.ingester.FlushHandler))
	t.Server.HTTP.Path("/shutdown").Handler(http.HandlerFunc(t.ingester.ShutdownHandler))
	return t.ingester, nil
}

func (t *App) initGenerator() (services.Service, error) {
	t.cfg.Generator.Ring.ListenPort = t.cfg.Server.GRPCListenPort
	genSvc, err := generator.New(&t.cfg.Generator, t.overrides, prometheus.DefaultRegisterer, log.Logger)
	if err == generator.ErrUnconfigured && t.cfg.Target != MetricsGenerator { // just warn if we're not running the metrics-generator
		level.Warn(log.Logger).Log("msg", "metrics-generator is not configured.", "err", err)
		return services.NewIdleService(nil, nil), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics-generator %w", err)
	}
	t.generator = genSvc

	deeppb.RegisterMetricsGeneratorServer(t.Server.GRPC, t.generator)

	return t.generator, nil
}

func (t *App) initQuerier() (services.Service, error) {
	// validate worker config
	// if we're not in single binary mode and worker address is not specified - bail
	if t.cfg.Target != SingleBinary && t.cfg.Querier.Worker.FrontendAddress == "" {
		return nil, fmt.Errorf("frontend worker address not specified")
	} else if t.cfg.Target == SingleBinary {
		// if we're in single binary mode with no worker address specified, register default endpoint
		if t.cfg.Querier.Worker.FrontendAddress == "" {
			t.cfg.Querier.Worker.FrontendAddress = fmt.Sprintf("127.0.0.1:%d", t.cfg.Server.GRPCListenPort)
			level.Warn(log.Logger).Log("msg", "Worker address is empty in single binary mode. Attempting automatic worker configuration. If queries are unresponsive consider configuring the worker explicitly.", "address", t.cfg.Querier.Worker.FrontendAddress)
		}
	}

	// do not enable polling if this is the single binary. in that case the compactor will take care of polling
	if t.cfg.Target == Querier {
		t.store.EnablePolling(nil)
	}

	// todo: make ingester client a module instead of passing config everywhere
	newQuerier, err := querier.New(t.cfg.Querier, t.cfg.IngesterClient, t.ring, t.store, t.overrides)
	if err != nil {
		return nil, fmt.Errorf("failed to create querier %w", err)
	}
	t.querier = newQuerier

	frontEndMiddleware := middleware.Merge(
		t.HTTPAuthMiddleware,
	)

	tracesHandler := frontEndMiddleware.Wrap(http.HandlerFunc(t.querier.SnapshotByIdHandler))
	t.Server.HTTP.Handle(path.Join(api.PathPrefixQuerier, addHTTPAPIPrefix(&t.cfg, api.PathSnapshots)), tracesHandler)

	searchHandler := t.HTTPAuthMiddleware.Wrap(http.HandlerFunc(t.querier.SearchHandler))
	t.Server.HTTP.Handle(path.Join(api.PathPrefixQuerier, addHTTPAPIPrefix(&t.cfg, api.PathSearch)), searchHandler)

	searchTagsHandler := t.HTTPAuthMiddleware.Wrap(http.HandlerFunc(t.querier.SearchTagsHandler))
	t.Server.HTTP.Handle(path.Join(api.PathPrefixQuerier, addHTTPAPIPrefix(&t.cfg, api.PathSearchTags)), searchTagsHandler)

	searchTagValuesHandler := t.HTTPAuthMiddleware.Wrap(http.HandlerFunc(t.querier.SearchTagValuesHandler))
	t.Server.HTTP.Handle(path.Join(api.PathPrefixQuerier, addHTTPAPIPrefix(&t.cfg, api.PathSearchTagValues)), searchTagValuesHandler)

	searchTagValuesV2Handler := t.HTTPAuthMiddleware.Wrap(http.HandlerFunc(t.querier.SearchTagValuesV2Handler))
	t.Server.HTTP.Handle(path.Join(api.PathPrefixQuerier, addHTTPAPIPrefix(&t.cfg, api.PathSearchTagValuesV2)), searchTagValuesV2Handler)

	return t.querier, t.querier.CreateAndRegisterWorker(t.Server.HTTPServer.Handler)
}

// initQueryFrontend creates a common front end API that will then distribute the calls to the appropriate backends
// we have 2 backends querier and tracepointAPI
// querier handles search queries - e.g. get snapshots by id, or deepql queries
// tracepointAPI handles requests to change the config for tracepoints
func (t *App) initQueryFrontend() (services.Service, error) {
	// we create to 2 bridges (roundTrippers) one for each backend
	roundTripper, tpTripper, v1, err := frontend.InitFrontend(t.cfg.Frontend.Config, frontend.CortexNoQuerierLimits{}, log.Logger, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, err
	}
	t.frontend = v1

	// create query frontend
	queryFrontend, err := frontend.New(t.cfg.Frontend, roundTripper, tpTripper, t.overrides, t.store, log.Logger, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, err
	}

	// wrap handlers with auth
	frontEndMiddleware := middleware.Merge(
		t.HTTPAuthMiddleware,
		httpGzipMiddleware(),
	)

	traceByIDHandler := frontEndMiddleware.Wrap(queryFrontend.SnapshotByID)
	searchHandler := frontEndMiddleware.Wrap(queryFrontend.Search)

	// register grpc server for workers to connect to
	frontend_v1pb.RegisterFrontendServer(t.Server.GRPC, t.frontend)

	// http snapshot by id endpoint
	t.Server.HTTP.Handle(addHTTPAPIPrefix(&t.cfg, api.PathSnapshots), traceByIDHandler)

	// http search endpoints
	t.Server.HTTP.Handle(addHTTPAPIPrefix(&t.cfg, api.PathSearch), searchHandler)
	t.Server.HTTP.Handle(addHTTPAPIPrefix(&t.cfg, api.PathSearchTags), searchHandler)
	t.Server.HTTP.Handle(addHTTPAPIPrefix(&t.cfg, api.PathSearchTagValues), searchHandler)
	t.Server.HTTP.Handle(addHTTPAPIPrefix(&t.cfg, api.PathSearchTagValuesV2), searchHandler)

	loadTracepointHandler := frontEndMiddleware.Wrap(queryFrontend.LoadTracepointHandler)
	delTracepointHandler := frontEndMiddleware.Wrap(queryFrontend.DelTracepointHandler)

	// http tracepoint endpoints
	t.Server.HTTP.Handle(addHTTPAPIPrefix(&t.cfg, api.PathTracepoints), loadTracepointHandler)
	t.Server.HTTP.Handle(addHTTPAPIPrefix(&t.cfg, api.PathDeleteTracepoint), delTracepointHandler)

	// the query frontend needs to have knowledge of the blocks, so it can shard search jobs
	t.store.EnablePolling(nil)

	// http query echo endpoint
	t.Server.HTTP.Handle(addHTTPAPIPrefix(&t.cfg, api.PathEcho), echoHandler())

	// http endpoint to see usage stats data
	t.Server.HTTP.Handle(addHTTPAPIPrefix(&t.cfg, api.PathUsageStats), usageStatsHandler(t.cfg.UsageReport))

	return t.frontend, nil
}

func (t *App) initCompactor() (services.Service, error) {
	if t.cfg.Target == ScalableSingleBinary && t.cfg.Compactor.ShardingRing.KVStore.Store == "" {
		t.cfg.Compactor.ShardingRing.KVStore.Store = "memberlist"
	}

	newCompactor, err := compactor.New(t.cfg.Compactor, t.store, t.overrides, prometheus.DefaultRegisterer)
	if err != nil {
		return nil, fmt.Errorf("failed to create compactor %w", err)
	}
	t.compactor = newCompactor

	if t.compactor.Ring != nil {
		t.Server.HTTP.Handle("/compactor/ring", t.compactor.Ring)
	}

	return t.compactor, nil
}

// initStore will create the store service that can read/write to the configured storage service (S3, local disk etc)
func (t *App) initStore() (services.Service, error) {
	store, err := deep_storage.NewStore(t.cfg.StorageConfig, log.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create store %w", err)
	}
	t.store = store

	return t.store, nil
}

func (t *App) initMemberListKV() (services.Service, error) {
	reg := prometheus.DefaultRegisterer
	t.cfg.MemberlistKV.MetricsRegisterer = reg
	t.cfg.MemberlistKV.MetricsNamespace = metricsNamespace
	t.cfg.MemberlistKV.Codecs = []codec.Codec{
		ring.GetCodec(),
		usagestats.JSONCodec,
	}

	dnsProviderReg := prometheus.WrapRegistererWithPrefix(
		"deep_",
		prometheus.WrapRegistererWith(
			prometheus.Labels{"name": "memberlist"},
			reg,
		),
	)

	dnsProvider := dns.NewProvider(log.Logger, dnsProviderReg, dns.GolangResolverType)
	t.MemberlistKV = memberlist.NewKVInitService(&t.cfg.MemberlistKV, log.Logger, dnsProvider, reg)

	t.cfg.Ingester.LifecyclerConfig.RingConfig.KVStore.MemberlistKV = t.MemberlistKV.GetMemberlistKV
	t.cfg.Generator.Ring.KVStore.MemberlistKV = t.MemberlistKV.GetMemberlistKV
	t.cfg.Distributor.DistributorRing.KVStore.MemberlistKV = t.MemberlistKV.GetMemberlistKV
	t.cfg.Compactor.ShardingRing.KVStore.MemberlistKV = t.MemberlistKV.GetMemberlistKV

	t.cfg.Tracepoint.LifecyclerConfig.RingConfig.KVStore.MemberlistKV = t.MemberlistKV.GetMemberlistKV

	t.Server.HTTP.Handle("/memberlist", t.MemberlistKV)

	return t.MemberlistKV, nil
}

func (t *App) initUsageReport() (services.Service, error) {
	if !t.cfg.UsageReport.Enabled {
		return nil, nil
	}

	t.cfg.UsageReport.Leader = false
	if t.isModuleActive(Ingester) {
		t.cfg.UsageReport.Leader = true
	}

	usagestats.Target(t.cfg.Target)

	var err error
	var reader backend.RawReader
	var writer backend.RawWriter

	switch t.cfg.StorageConfig.TracePoint.Backend {
	case "local":
		reader, writer, _, err = local.New(t.cfg.StorageConfig.TracePoint.Local)
	case "gcs":
		reader, writer, _, err = gcs.New(t.cfg.StorageConfig.TracePoint.GCS)
	case "s3":
		reader, writer, _, err = s3.New(t.cfg.StorageConfig.TracePoint.S3)
	case "azure":
		reader, writer, _, err = azure.New(t.cfg.StorageConfig.TracePoint.Azure)
	default:
		err = fmt.Errorf("unknown backend %s", t.cfg.StorageConfig.TracePoint.Backend)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to initialize usage report: %w", err)
	}

	ur, err := usagestats.NewReporter(t.cfg.UsageReport, t.cfg.Ingester.LifecyclerConfig.RingConfig.KVStore, reader, writer, util_log.Logger, prometheus.DefaultRegisterer)
	if err != nil {
		level.Info(util_log.Logger).Log("msg", "failed to initialize usage report", "err", err)
		return nil, nil
	}
	t.usageReport = ur
	return ur, nil
}

func (t *App) setupModuleManager() error {
	mm := modules.NewManager(log.Logger)

	mm.RegisterModule(Server, t.initServer, modules.UserInvisibleModule)
	mm.RegisterModule(InternalServer, t.initInternalServer, modules.UserInvisibleModule)
	mm.RegisterModule(MemberlistKV, t.initMemberListKV, modules.UserInvisibleModule)
	mm.RegisterModule(Ring, t.initRing, modules.UserInvisibleModule)
	mm.RegisterModule(TPRing, t.initTPRing, modules.UserInvisibleModule)
	mm.RegisterModule(MetricsGeneratorRing, t.initGeneratorRing, modules.UserInvisibleModule)
	mm.RegisterModule(Overrides, t.initOverrides, modules.UserInvisibleModule)
	mm.RegisterModule(Distributor, t.initDistributor)
	mm.RegisterModule(Ingester, t.initIngester)
	mm.RegisterModule(TracepointClient, t.initTracepointClient)
	mm.RegisterModule(Tracepoint, t.initTracepoint)
	mm.RegisterModule(TracepointAPI, t.initTracepointAPI)
	mm.RegisterModule(Querier, t.initQuerier)
	mm.RegisterModule(QueryFrontend, t.initQueryFrontend)
	mm.RegisterModule(Compactor, t.initCompactor)
	mm.RegisterModule(MetricsGenerator, t.initGenerator)
	mm.RegisterModule(Store, t.initStore, modules.UserInvisibleModule)
	mm.RegisterModule(SingleBinary, nil)
	mm.RegisterModule(ScalableSingleBinary, nil)
	mm.RegisterModule(UsageReport, t.initUsageReport)

	deps := map[string][]string{
		Server: {InternalServer},
		// Store:        nil,
		Overrides:            {Server},
		MemberlistKV:         {Server},
		QueryFrontend:        {Store, Server, Overrides, UsageReport},
		Ring:                 {Server, MemberlistKV},
		TPRing:               {Server, MemberlistKV},
		MetricsGeneratorRing: {Server, MemberlistKV},
		Distributor:          {Ring, Server, Overrides, UsageReport, MetricsGeneratorRing, TracepointClient},
		Ingester:             {Store, Server, Overrides, MemberlistKV, UsageReport},
		TracepointClient:     {TPRing},
		Tracepoint:           {Store, Server, Overrides, MemberlistKV, UsageReport, TracepointClient},
		TracepointAPI:        {Server, TracepointClient, Tracepoint},
		MetricsGenerator:     {Server, Overrides, MemberlistKV, UsageReport},
		Querier:              {Store, Ring, Overrides, UsageReport},
		Compactor:            {Store, Server, Overrides, MemberlistKV, UsageReport},
		SingleBinary:         {Compactor, QueryFrontend, Querier, Tracepoint, TracepointAPI, Ingester, Distributor, MetricsGenerator},
		ScalableSingleBinary: {SingleBinary},
		UsageReport:          {MemberlistKV},
	}

	for mod, targets := range deps {
		if err := mm.AddDependency(mod, targets...); err != nil {
			return err
		}
	}

	t.ModuleManager = mm

	t.deps = deps

	return nil
}

func (t *App) isModuleActive(m string) bool {
	if t.cfg.Target == m {
		return true
	}
	if t.recursiveIsModuleActive(t.cfg.Target, m) {
		return true
	}

	return false
}

func (t *App) recursiveIsModuleActive(target, m string) bool {
	if targetDeps, ok := t.deps[target]; ok {
		for _, dep := range targetDeps {
			if dep == m {
				return true
			}
			if t.recursiveIsModuleActive(dep, m) {
				return true
			}
		}
	}
	return false
}

func addHTTPAPIPrefix(cfg *Config, apiPath string) string {
	return path.Join(cfg.HTTPAPIPrefix, apiPath)
}

func echoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "echo", http.StatusOK)
	}
}

func usageStatsHandler(urCfg usagestats.Config) http.HandlerFunc {
	if !urCfg.Enabled {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "usage-stats is not enabled", http.StatusOK)
		}
	}

	// usage stats is Enabled, build and return usage stats json
	reportStr, err := jsoniter.MarshalToString(usagestats.BuildStats())
	if err != nil {
		return func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "error building usage report", http.StatusInternalServerError)
		}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, reportStr)
	}
}
