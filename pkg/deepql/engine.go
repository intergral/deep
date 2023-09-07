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
	"io"
	"time"

	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/util"
	"github.com/opentracing/opentracing-go"
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Execute(ctx context.Context, searchReq *deeppb.SearchRequest, snapshotResultFetcher SnapshotResultFetcher) (*deeppb.SearchResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "deepql.Engine.Execute")
	defer span.Finish()

	rootExpr, err := e.parseQuery(searchReq)
	if err != nil {
		return nil, err
	}

	snapshotRequest := e.createFetchSnapshotRequest(searchReq, rootExpr.Pipeline)

	span.SetTag("pipeline", rootExpr.Pipeline)
	span.SetTag("fetchSnapshotRequest", snapshotRequest)

	evaluated := 0
	// set up the expression evaluation as a filter to reduce data pulled
	snapshotRequest.Filter = func(inSS *SnapshotResult) ([]*SnapshotResult, error) {
		if inSS.Snapshot == nil {
			return nil, nil
		}

		evalSS, err := rootExpr.Pipeline.evaluate([]*SnapshotResult{inSS})
		if err != nil {
			span.LogKV("msg", "pipeline.evaluate", "err", err)
			return nil, err
		}

		evaluated++
		if len(evalSS) == 0 {
			return nil, nil
		}

		return evalSS, nil
	}

	fetchSnapshotResponse, err := snapshotResultFetcher.Fetch(ctx, snapshotRequest)
	if err != nil {
		return nil, err
	}
	iterator := fetchSnapshotResponse.Results
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

	span.SetTag("snapshots_evaluated", evaluated)
	span.SetTag("snapshots_found", len(res.Snapshots))

	return res, nil
}

func (e *Engine) parseQuery(searchReq *deeppb.SearchRequest) (*RootExpr, error) {
	r, err := Parse(searchReq.Query)
	if err != nil {
		return nil, err
	}
	return r, r.validate()
}

// createFetchSnapshotRequest will flatten the SearchRequest in simple conditions the storage layer
// can work with.
func (e *Engine) createFetchSnapshotRequest(searchReq *deeppb.SearchRequest, pipeline Pipeline) FetchSnapshotRequest {
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
