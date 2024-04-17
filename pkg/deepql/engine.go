/*
 * Copyright (C) 2024  Intergral GmbH
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
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deeppb"
	v1 "github.com/intergral/deep/pkg/deeppb/common/v1"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/util"
	"github.com/opentracing/opentracing-go"
)

type Engine struct{}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) ExecuteSearch(ctx context.Context, searchReq *deeppb.SearchRequest, snapshotResultFetcher SnapshotResultFetcher) (*deeppb.SearchResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "deepql.Engine.ExecuteSearch")
	defer span.Finish()

	span.SetTag("deepql", searchReq.Query)

	rootExpr, err := ParseString(searchReq.Query)
	span.LogKV("msg", "failed to parse deepql statement", "deepql", searchReq.Query, "err", err)
	if err != nil {
		return nil, err
	}

	if rootExpr.search != nil {
		span.SetTag("ql_type", "search")
		snapshotRequest, err := doSearchSnapshotRequest(ctx, searchReq, rootExpr.search, snapshotResultFetcher)
		return snapshotRequest, err
	}

	return nil, errors.New("invalid deepql search query")
}

func (e *Engine) ExecuteTriggerQuery(ctx context.Context, expr *RootExpr, handler TriggerHandler) (*deeptp.TracePointConfig, []*deeptp.TracePointConfig, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "deepql.Engine.ExecuteTriggerQuery")
	defer span.Finish()
	span.SetTag("ql_type", "trigger")

	request, err := createTriggerRequest(expr.trigger)
	if err != nil {
		return nil, nil, err
	}

	return handler(ctx, request)
}

func createTriggerRequest(t *trigger) (*deeppb.CreateTracepointRequest, error) {
	// todo this is a temporary migration to old request type until new API is defined
	req := &deeppb.CreateTracepointRequest{
		Tracepoint: &deeptp.TracePointConfig{
			ID:         uuid.New().String(),
			Path:       t.file,
			LineNumber: uint32(t.line),
			Args: map[string]string{
				"fire_count":  strconv.Itoa(t.fireCount),
				"fire_period": strconv.Itoa(int(t.rateLimit)),
				"condition":   t.condition,
			},
			Watches:   t.watch,
			Targeting: parseTargeting(t.targeting),
			Metrics:   nil,
		},
	}
	if t.windowStart != "" {
		startTime, err := parseTimeWindow(t.windowStart)
		if err != nil {
			return nil, err
		}
		req.Tracepoint.Args["window_start"] = strconv.FormatInt(startTime.Unix(), 10)
	}

	if t.windowEnd != "" {
		endDur, err := parseTimeWindow(t.windowEnd)
		if err != nil {
			return nil, err
		}
		req.Tracepoint.Args["window_end"] = strconv.FormatInt(endDur.Unix(), 10)
	}

	if t.method != "" {
		req.Tracepoint.Args["method_name"] = t.method
	}

	if t.log != "" {
		req.Tracepoint.Args["log_msg"] = t.log
	}

	if t.span != "" {
		req.Tracepoint.Args["span"] = t.span
	}

	if t.spanName != "" {
		req.Tracepoint.Args["span_name"] = t.spanName
	}

	if !t.snapshot {
		req.Tracepoint.Args["snapshot"] = "no_collect"
	}

	if t.target != "" {
		if t.method == "" {
			req.Tracepoint.Args["stage"] = fmt.Sprintf("line_%s", t.target)
		} else {
			req.Tracepoint.Args["stage"] = fmt.Sprintf("method_%s", t.target)
		}
	}

	if t.metric {
		req.Tracepoint.Metrics = []*deeptp.Metric{
			{
				Name:             metricNameOrDefault(t.metricName, t.file, t.line, t.method),
				LabelExpressions: parseLabels(t.metricLabels),
				Type:             t.metricType,
				Expression:       &t.metricExpression,
				Namespace:        &t.metricNamespace,
				Help:             &t.metricHelp,
				Unit:             &t.metricUnit,
			},
		}
	}

	processLabels("span_", req, t.spanLabels)
	processLabels("log_", req, t.logLabels)
	processLabels("snapshot_", req, t.snapshotLabels)

	return req, nil
}

func parseLabels(labels []label) []*deeptp.LabelExpression {
	ret := make([]*deeptp.LabelExpression, len(labels))
	for i, l := range labels {
		ret[i] = &deeptp.LabelExpression{
			Key: l.key,
			Value: &deeptp.LabelExpression_Expression{
				Expression: l.value,
			},
		}
	}
	return ret
}

func metricNameOrDefault(name string, file string, l uint, m string) string {
	if name != "" {
		return name
	}
	if m == "" {
		return fmt.Sprintf("%s_%d", file, l)
	}
	return fmt.Sprintf("%s_%s", file, m)
}

func parseTargeting(targeting map[string]string) []*v1.KeyValue {
	ret := make([]*v1.KeyValue, 0, len(targeting))
	indx := 0
	for k, v := range targeting {
		ret[indx] = &v1.KeyValue{
			Key:   k,
			Value: &v1.AnyValue{Value: &v1.AnyValue_StringValue{StringValue: v}},
		}
		indx++
	}
	return ret
}

func processLabels(prefix string, req *deeppb.CreateTracepointRequest, labels []label) {
	if labels == nil {
		return
	}
	for _, l := range labels {
		req.Tracepoint.Args[fmt.Sprintf("%s_lable_%s", prefix, l.key)] = l.value
	}
}

func (e *Engine) ExecuteCommandQuery(ctx context.Context, expr *RootExpr, handler CommandHandler) ([]*deeptp.TracePointConfig, []*deeptp.TracePointConfig, string, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "deepql.Engine.ExecuteCommandQuery")
	defer span.Finish()
	span.SetTag("ql_type", "command")

	request, err := createCommandRequest(expr.command)
	if err != nil {
		return nil, nil, "", err
	}

	return handler(ctx, request)
}

func createCommandRequest(c *command) (*CommandRequest, error) {
	return &CommandRequest{
		Command:    c.command,
		Conditions: c.buildConditions(),
	}, nil
}

func doSearchSnapshotRequest(ctx context.Context, req *deeppb.SearchRequest, s *search, fetcher SnapshotResultFetcher) (*deeppb.SearchResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "deepql.Engine.doSearchSnapshotRequest")
	defer span.Finish()

	snapReq, err := createSearchRequest(req, s)
	if err != nil {
		return nil, err
	}

	fetch, err := fetcher(ctx, snapReq)
	if err != nil {
		return nil, err
	}
	iterator := fetch.Results
	defer iterator.Close()

	response := &deeppb.SearchResponse{
		Snapshots: nil,
		// todo add metrics
		Metrics: &deeppb.SearchMetrics{},
	}

	for {
		next, err := iterator.Next(ctx)
		if err != nil && err != io.EOF {
			span.LogKV("msg", "iterator.Next", "err", err)
			return nil, err
		}

		if next == nil {
			break
		}
		response.Snapshots = append(response.Snapshots, asSnapshotSearchMetadata(next))

		if len(response.Snapshots) >= int(req.Limit) && req.Limit > 0 {
			break
		}
	}

	span.SetTag("snapshots_found", len(response.Snapshots))

	return response, nil
}

func asSnapshotSearchMetadata(result *SnapshotResult) *deeppb.SnapshotSearchMetadata {
	return &deeppb.SnapshotSearchMetadata{
		SnapshotID:        util.SnapshotIDToHexString(result.SnapshotID),
		ServiceName:       result.ServiceName,
		FilePath:          result.FilePath,
		LineNo:            result.LineNo,
		StartTimeUnixNano: result.StartTimeUnixNanos,
		DurationNano:      result.DurationNanos,
	}
}

func createSearchRequest(req *deeppb.SearchRequest, s *search) (FetchSnapshotRequest, error) {
	return FetchSnapshotRequest{
		StartTimeUnixNanos: unixSecToNano(req.Start),
		EndTimeUnixNanos:   unixSecToNano(req.End),
		Conditions:         s.buildConditions(),
	}, nil
}

func unixSecToNano(ts uint32) uint64 {
	return uint64(ts) * uint64(time.Second/time.Nanosecond)
}
