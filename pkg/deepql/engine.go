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

package deepql

import (
	"context"
	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/util"
	"github.com/opentracing/opentracing-go"
	"io"
	"time"
)

type Engine struct {
	spansPerSpanSet int
}

func NewEngine() *Engine {
	return &Engine{
		spansPerSpanSet: 3, // TODO make configurable
	}
}

func (e *Engine) Execute(ctx context.Context, searchReq *deeppb.SearchRequest, spanSetFetcher SnapshotResultFetcher) (*deeppb.SearchResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "traceql.Engine.Execute")
	defer span.Finish()

	rootExpr, err := e.parseQuery(searchReq)
	if err != nil {
		return nil, err
	}

	fetchSpansRequest := e.createFetchSnapshotRequest(searchReq, rootExpr.Pipeline)

	span.SetTag("pipeline", rootExpr.Pipeline)
	span.SetTag("fetchSpansRequest", fetchSpansRequest)

	spansetsEvaluated := 0
	// set up the expression evaluation as a filter to reduce data pulled
	fetchSpansRequest.Filter = func(inSS *SnapshotResult) ([]*SnapshotResult, error) {
		if inSS.Snapshot == nil {
			return nil, nil
		}

		evalSS, err := rootExpr.Pipeline.evaluate([]*SnapshotResult{inSS})
		if err != nil {
			span.LogKV("msg", "pipeline.evaluate", "err", err)
			return nil, err
		}

		spansetsEvaluated++
		if len(evalSS) == 0 {
			return nil, nil
		}

		return evalSS, nil
	}

	fetchSpansResponse, err := spanSetFetcher.Fetch(ctx, fetchSpansRequest)
	if err != nil {
		return nil, err
	}
	iterator := fetchSpansResponse.Results
	defer iterator.Close()

	res := &deeppb.SearchResponse{
		Snapshots: nil,
		// TODO capture and update metrics
		Metrics: &deeppb.SearchMetrics{},
	}
	for {
		snapshotResult, err := iterator.Next(ctx)
		if err != nil && err != io.EOF {
			span.LogKV("msg", "iterator.Next", "err", err)
			return nil, err
		}
		if snapshotResult == nil {
			break
		}
		res.Snapshots = append(res.Snapshots, e.asSnapshotSearchMetadata(snapshotResult))

		if len(res.Snapshots) >= int(searchReq.Limit) && searchReq.Limit > 0 {
			break
		}
	}

	span.SetTag("spansets_evaluated", spansetsEvaluated)
	span.SetTag("spansets_found", len(res.Snapshots))

	return res, nil
}

func (e *Engine) parseQuery(searchReq *deeppb.SearchRequest) (*RootExpr, error) {
	r, err := Parse(searchReq.Query)
	if err != nil {
		return nil, err
	}
	return r, r.validate()
}

// createFetchSpansRequest will flatten the SpansetFilter in simple conditions the storage layer
// can work with.
func (e *Engine) createFetchSnapshotRequest(searchReq *deeppb.SearchRequest, pipeline Pipeline) FetchSnapshotRequest {
	// TODO handle SearchRequest.MinDurationMs and MaxDurationMs, this refers to the trace level duration which is not the same as the intrinsic duration

	req := FetchSnapshotRequest{
		StartTimeUnixNanos: unixSecToNano(searchReq.Start),
		EndTimeUnixNanos:   unixSecToNano(searchReq.End),
		Conditions:         nil,
		AllConditions:      true,
	}

	pipeline.extractConditions(&req)
	return req
}

func (e *Engine) asSnapshotSearchMetadata(result *SnapshotResult) *deeppb.SnapshotSearchMetadata {
	return &deeppb.SnapshotSearchMetadata{
		SnapshotID:        util.SnapshotIDToHexString(result.SnapshotID),
		ServiceName:       result.ServiceName,
		FilePath:          result.FilePath,
		LineNo:            result.LineNo,
		StartTimeUnixNano: result.StartTimeUnixNanos,
		DurationNano:      result.DurationNanos,
	}
}

func unixSecToNano(ts uint32) uint64 {
	return uint64(ts) * uint64(time.Second/time.Nanosecond)
}
