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
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/intergral/deep/pkg/api"
	"github.com/intergral/deep/pkg/deeppb"
	"github.com/opentracing/opentracing-go"
	ot_log "github.com/opentracing/opentracing-go/log"
)

const (
	BlockStartKey = "blockStart"
	BlockEndKey   = "blockEnd"
	QueryModeKey  = "mode"

	QueryModeIngesters = "ingesters"
	QueryModeBlocks    = "blocks"
	QueryModeAll       = "all"
)

func (q *Querier) SnapshotByIdHandler(w http.ResponseWriter, r *http.Request) {
	// Enforce the query timeout while querying backends
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(q.cfg.SnapshotByIDConfig.QueryTimeout))
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Querier.SnapshotByIdHandler")
	defer span.Finish()

	byteID, err := api.ParseSnapshotID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// validate request
	blockStart, blockEnd, queryMode, timeStart, timeEnd, err := api.ValidateAndSanitizeRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	span.LogFields(
		ot_log.String("msg", "validated request"),
		ot_log.String("blockStart", blockStart),
		ot_log.String("blockEnd", blockEnd),
		ot_log.String("queryMode", queryMode),
		ot_log.String("timeStart", fmt.Sprint(timeStart)),
		ot_log.String("timeEnd", fmt.Sprint(timeEnd)))

	resp, err := q.FindSnapshotByID(ctx, &deeppb.SnapshotByIDRequest{
		ID:         byteID,
		BlockStart: blockStart,
		BlockEnd:   blockEnd,
		QueryMode:  queryMode,
	}, timeStart, timeEnd)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// record not found here, but continue on so we can marshal metrics
	// to the body
	if resp.Snapshot == nil {
		w.WriteHeader(http.StatusNotFound)
	}

	api.ParseMessageToHttp(w, r, span, resp)
}

func (q *Querier) SearchHandler(w http.ResponseWriter, r *http.Request) {
	isSearchBlock := api.IsSearchBlock(r)

	// Enforce the query timeout while querying backends
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(q.cfg.Search.QueryTimeout))
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Querier.SearchHandler")
	defer span.Finish()

	span.SetTag("requestURI", r.RequestURI)
	span.SetTag("isSearchBlock", isSearchBlock)

	var resp *deeppb.SearchResponse
	if !isSearchBlock {
		req, err := api.ParseSearchRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		span.SetTag("SearchRequest", req.String())

		resp, err = q.SearchRecent(ctx, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		req, err := api.ParseSearchBlockRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		span.SetTag("SearchRequestBlock", req.String())

		resp, err = q.SearchBlock(ctx, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	api.ParseMessageToHttp(w, r, span, resp)
}

func (q *Querier) SearchTagsHandler(w http.ResponseWriter, r *http.Request) {
	// Enforce the query timeout while querying backends
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(q.cfg.Search.QueryTimeout))
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Querier.SearchTagsHandler")
	defer span.Finish()

	req := &deeppb.SearchTagsRequest{}

	resp, err := q.SearchTags(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	api.ParseMessageToHttp(w, r, span, resp)
}

func (q *Querier) SearchTagValuesHandler(w http.ResponseWriter, r *http.Request) {
	// Enforce the query timeout while querying backends
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(q.cfg.Search.QueryTimeout))
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Querier.SearchTagValuesHandler")
	defer span.Finish()

	vars := mux.Vars(r)
	tagName, ok := vars["tagName"]
	if !ok {
		http.Error(w, "please provide a tagName", http.StatusBadRequest)
		return
	}
	req := &deeppb.SearchTagValuesRequest{
		TagName: tagName,
	}

	resp, err := q.SearchTagValues(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	api.ParseMessageToHttp(w, r, span, resp)
}

func (q *Querier) SearchTagValuesV2Handler(w http.ResponseWriter, r *http.Request) {
	// Enforce the query timeout while querying backends
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(q.cfg.Search.QueryTimeout))
	defer cancel()

	span, ctx := opentracing.StartSpanFromContext(ctx, "Querier.SearchTagValuesHandler")
	defer span.Finish()

	vars := mux.Vars(r)
	tagName, ok := vars["tagName"]
	if !ok {
		http.Error(w, "please provide a tagName", http.StatusBadRequest)
		return
	}

	req := &deeppb.SearchTagValuesRequest{
		TagName: tagName,
	}

	resp, err := q.SearchTagValuesV2(ctx, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	api.ParseMessageToHttp(w, r, span, resp)
}
