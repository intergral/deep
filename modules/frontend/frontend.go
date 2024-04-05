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

package frontend

import (
	"bytes"
	"fmt"
	"github.com/intergral/deep/pkg/deepql"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/util"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/golang/protobuf/jsonpb" //nolint:all //deprecated
	"github.com/golang/protobuf/proto"  //nolint:all //deprecated
	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/pkg/api"
	"github.com/intergral/deep/pkg/deepdb"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	snapshotByIDOp = "snapshots"
	searchOp       = "search"
)

type QueryFrontend struct {
	SnapshotByID, Search  http.Handler
	logger                log.Logger
	store                 storage.Store
	LoadTracepointHandler http.Handler
	DelTracepointHandler  http.Handler
}

// New returns a new QueryFrontend
func New(cfg Config, next http.RoundTripper, tpNext http.RoundTripper, o *overrides.Overrides, store storage.Store, logger log.Logger, registerer prometheus.Registerer) (*QueryFrontend, error) {
	_ = level.Info(logger).Log("msg", "creating middleware in query frontend")

	if cfg.SnapshotByID.QueryShards < minQueryShards || cfg.SnapshotByID.QueryShards > maxQueryShards {
		return nil, fmt.Errorf("frontend query shards should be between %d and %d (both inclusive)", minQueryShards, maxQueryShards)
	}

	if cfg.Search.Sharder.ConcurrentRequests <= 0 {
		return nil, fmt.Errorf("frontend search concurrent requests should be greater than 0")
	}

	if cfg.Search.Sharder.TargetBytesPerRequest <= 0 {
		return nil, fmt.Errorf("frontend search target bytes per request should be greater than 0")
	}

	if cfg.Search.Sharder.QueryIngestersUntil < cfg.Search.Sharder.QueryBackendAfter {
		return nil, fmt.Errorf("query backend after should be less than or equal to query ingester until")
	}

	queriesPerTenant := promauto.With(registerer).NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "query_frontend",
		Name:      "queries_total",
		Help:      "Total queries received per tenant.",
	}, []string{"tenant", "op", "status"})

	retryWare := newRetryWare(cfg.MaxRetries, registerer)

	snapshotByIDMiddleware := MergeMiddlewares(newSnapshotByIDMiddleware(cfg, logger), retryWare)
	searchMiddleware := MergeMiddlewares(newSearchMiddleware(cfg, o, store, logger), retryWare)

	snapshotByIDCounter := queriesPerTenant.MustCurryWith(prometheus.Labels{"op": snapshotByIDOp})
	searchCounter := queriesPerTenant.MustCurryWith(prometheus.Labels{"op": searchOp})
	loadTp := queriesPerTenant.MustCurryWith(prometheus.Labels{"op": "loadtp"})
	delTp := queriesPerTenant.MustCurryWith(prometheus.Labels{"op": "deltp"})

	snapshots := snapshotByIDMiddleware.Wrap(next)
	search := searchMiddleware.Wrap(next)

	tpMiddleware := newTracepointForwardMiddleware()
	tpHandler := tpMiddleware.Wrap(tpNext)

	return &QueryFrontend{
		SnapshotByID:          newHandler(snapshots, snapshotByIDCounter, logger),
		Search:                newHandler(search, searchCounter, logger),
		LoadTracepointHandler: newHandler(tpHandler, loadTp, logger),
		DelTracepointHandler:  newHandler(tpHandler, delTp, logger),
		logger:                logger,
		store:                 store,
	}, nil
}

func newTracepointForwardMiddleware() Middleware {
	return MiddlewareFunc(func(next http.RoundTripper) http.RoundTripper {
		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			// We just need to modify the uri to match the api expectation
			r.RequestURI = buildUpstreamRequestURI(api.PathPrefixTracepoints, r.RequestURI, r.URL.Query())
			resp, err := next.RoundTrip(r)
			return resp, err
		})
	})
}

// newSnapshotByIDMiddleware creates a new frontend middleware responsible for handling get snapshot requests.
func newSnapshotByIDMiddleware(cfg Config, logger log.Logger) Middleware {
	return MiddlewareFunc(func(next http.RoundTripper) http.RoundTripper {
		// We're constructing middleware in this statement, each middleware wraps the next one from left-to-right
		// - the ShardingWare shards queries by splitting the block ID space
		// - the RetryWare retries requests that have failed (error or http status 500)
		rt := NewRoundTripper(
			next,
			newSnapshotByIDSharder(cfg.SnapshotByID.QueryShards, cfg.TolerateFailedBlocks, cfg.SnapshotByID.SLO, logger),
			newHedgedRequestWare(cfg.SnapshotByID.Hedging),
		)

		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
			// validate snapshot
			_, err := api.ParseSnapshotID(r)
			if err != nil {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(err.Error())),
					Header:     http.Header{},
				}, nil
			}

			// validate start and end parameter
			_, _, _, _, _, reqErr := api.ValidateAndSanitizeRequest(r)
			if reqErr != nil {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(reqErr.Error())),
					Header:     http.Header{},
				}, nil
			}

			// check marshalling format
			marshallingFormat := api.HeaderAcceptJSON
			if r.Header.Get(api.HeaderAccept) == api.HeaderAcceptProtobuf {
				marshallingFormat = api.HeaderAcceptProtobuf
			}

			// enforce all communication internal to Deep to be in protobuf bytes
			r.Header.Set(api.HeaderAccept, api.HeaderAcceptProtobuf)

			resp, err := rt.RoundTrip(r)

			// todo : should all of this request/response content type be up a level and be used for all query types?
			if resp != nil && resp.StatusCode == http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if err != nil {
					return nil, errors.Wrap(err, "error reading response body at query frontend")
				}
				responseObject := &deeppb.SnapshotByIDResponse{}
				err = proto.Unmarshal(body, responseObject)
				if err != nil {
					return nil, err
				}

				if marshallingFormat == api.HeaderAcceptJSON {
					var jsonSnapshot bytes.Buffer
					marshaller := &jsonpb.Marshaler{}
					err = marshaller.Marshal(&jsonSnapshot, responseObject.Snapshot)
					if err != nil {
						return nil, err
					}
					resp.Body = io.NopCloser(bytes.NewReader(jsonSnapshot.Bytes()))
				} else {
					snapshotBuffer, err := proto.Marshal(responseObject.Snapshot)
					if err != nil {
						return nil, err
					}
					resp.Body = io.NopCloser(bytes.NewReader(snapshotBuffer))
				}

				if resp.Header != nil {
					resp.Header.Set(api.HeaderContentType, marshallingFormat)
				}
			}
			span := opentracing.SpanFromContext(r.Context())
			if span != nil {
				span.SetTag("contentType", marshallingFormat)
			}

			return resp, err
		})
	})
}

// newSearchMiddleware creates a new frontend middleware to handle search and search tags requests.
func newSearchMiddleware(cfg Config, o *overrides.Overrides, reader deepdb.Reader, logger log.Logger) Middleware {
	return MiddlewareFunc(func(next http.RoundTripper) http.RoundTripper {
		ingesterSearchRT := next
		backendSearchRT := NewRoundTripper(next, newSearchSharder(reader, o, cfg.Search.Sharder, cfg.Search.SLO, logger))

		return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {

			if is, q := api.IsDeepQLReq(r); is {
				expr, err := deepql.ParseString(q)
				if err != nil {
					return nil, err
				}

				if !expr.IsSearch() {
					// forward to tracepoint handler as we are a command/trigger ql
					r.RequestURI = buildUpstreamRequestURI(api.PathPrefixTracepoints, api.PathTracepointsQuery, r.URL.Query())
					resp, err := next.RoundTrip(r)
					return resp, err
				}

				// we might be ql but we are search so continue
			}

			// backend search queries require sharding so we pass through a special roundtripper
			if api.IsBackendSearch(r) {
				return backendSearchRT.RoundTrip(r)
			}

			// ingester search queries only need to be proxied to a single querier
			tenantID, _ := util.ExtractTenantID(r.Context())

			r.Header.Set(util.TenantIDHeaderName, tenantID)
			r.RequestURI = buildUpstreamRequestURI(api.PathPrefixQuerier, r.RequestURI, nil)

			return ingesterSearchRT.RoundTrip(r)
		})
	})
}

// buildUpstreamRequestURI returns a uri based on the passed parameters
// we do this because weaveworks/common uses the RequestURI field to translate from http.Request to httpgrpc.Request
// https://github.com/weaveworks/common/blob/47e357f4e1badb7da17ad74bae63e228bdd76e8f/httpgrpc/server/server.go#L48
func buildUpstreamRequestURI(prefix, originalURI string, params url.Values) string {
	const queryDelimiter = "?"

	uri := path.Join(prefix, originalURI)
	if len(params) > 0 {
		uri += queryDelimiter + params.Encode()
	}

	return uri
}
