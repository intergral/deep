package ingester

import (
	"context"
	"fmt"
	"sort"

	"github.com/go-kit/log/level"
	"github.com/intergral/deep/pkg/api"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deeppb"
	"github.com/intergral/deep/pkg/deepql"
	"github.com/intergral/deep/pkg/search"
	"github.com/intergral/deep/pkg/util"
	"github.com/intergral/deep/pkg/util/log"
	"github.com/opentracing/opentracing-go"
	ot_log "github.com/opentracing/opentracing-go/log"
	"github.com/weaveworks/common/user"
)

func (i *instance) Search(ctx context.Context, req *deeppb.SearchRequest) (*deeppb.SearchResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "instance.Search")
	defer span.Finish()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	maxResults := int(req.Limit)
	// if limit is not set, use a safe default
	if maxResults == 0 {
		maxResults = 20
	}

	span.LogFields(ot_log.String("SearchRequest", req.String()))

	sr := search.NewResults()
	defer sr.Close() // signal all running workers to quit

	// Lock blocks mutex until all search tasks have been created. This avoids
	// deadlocking with other activity (ingest, flushing), caused by releasing
	// and then attempting to retake the lock.
	i.blocksMtx.RLock()
	i.searchWAL(ctx, req, sr)
	i.searchLocalBlocks(ctx, req, sr)
	i.blocksMtx.RUnlock()

	sr.AllWorkersStarted()

	// read and combine search results
	resultsMap := map[string]*deeppb.SnapshotSearchMetadata{}

	// collect results from all the goroutines via sr.Results channel.
	// range loop will exit when sr.Results channel is closed.
	for result := range sr.Results() {
		// exit early and Propagate error upstream
		if sr.Error() != nil {
			return nil, sr.Error()
		}

		// Dedupe/combine results
		if existing := resultsMap[result.SnapshotID]; existing != nil {
			search.CombineSearchResults(existing, result)
		} else {
			resultsMap[result.SnapshotID] = result
		}

		if len(resultsMap) >= maxResults {
			sr.Close() // signal pending workers to exit
			break
		}
	}

	// can happen when we have only error, and no results
	if sr.Error() != nil {
		return nil, sr.Error()
	}

	results := make([]*deeppb.SnapshotSearchMetadata, 0, len(resultsMap))
	for _, result := range resultsMap {
		results = append(results, result)
	}

	// Sort
	sort.Slice(results, func(i, j int) bool {
		return results[i].StartTimeUnixNano > results[j].StartTimeUnixNano
	})

	return &deeppb.SearchResponse{
		Snapshots: results,
		Metrics: &deeppb.SearchMetrics{
			InspectedTraces: sr.TracesInspected(),
			InspectedBytes:  sr.BytesInspected(),
			InspectedBlocks: sr.BlocksInspected(),
			SkippedBlocks:   sr.BlocksSkipped(),
		},
	}, nil
}

// searchWAL starts a search task for every WAL block. Must be called under lock.
func (i *instance) searchWAL(ctx context.Context, req *deeppb.SearchRequest, sr *search.Results) {
	searchWalBlock := func(b common.WALBlock) {
		blockID := b.BlockMeta().BlockID.String()
		span, ctx := opentracing.StartSpanFromContext(ctx, "instance.searchWALBlock", opentracing.Tags{
			"blockID": blockID,
		})
		defer span.Finish()
		defer sr.FinishWorker()

		var resp *deeppb.SearchResponse
		var err error

		opts := common.DefaultSearchOptions()
		if api.IsTraceQLQuery(req) {
			// note: we are creating new engine for each wal block,
			// and engine.Execute is parsing the query for each block
			resp, err = deepql.NewEngine().Execute(ctx, req, deepql.NewSnapshotResultFetcherWrapper(func(ctx context.Context, req deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
				return b.Fetch(ctx, req, opts)
			}))
		} else {
			resp, err = b.Search(ctx, req, opts)
		}

		if err == common.ErrUnsupported {
			level.Warn(log.Logger).Log("msg", "wal block does not support search", "blockID", b.BlockMeta().BlockID)
			return
		}
		if err != nil {
			level.Error(log.Logger).Log("msg", "error searching local block", "blockID", blockID, "block_version", b.BlockMeta().Version, "err", err)
			sr.SetError(err)
			return
		}

		sr.AddBlockInspected()
		sr.AddBytesInspected(resp.Metrics.InspectedBytes)
		sr.AddTraceInspected(resp.Metrics.InspectedTraces)
		for _, r := range resp.Snapshots {
			sr.AddResult(ctx, r)
		}
	}

	// head block
	if i.headBlock != nil {
		sr.StartWorker()
		go searchWalBlock(i.headBlock)
	}

	// completing blocks
	for _, b := range i.completingBlocks {
		sr.StartWorker()
		go searchWalBlock(b)
	}
}

// searchLocalBlocks starts a search task for every local block. Must be called under lock.
func (i *instance) searchLocalBlocks(ctx context.Context, req *deeppb.SearchRequest, sr *search.Results) {
	// next check all complete blocks to see if they were not searched, if they weren't then attempt to search them
	for _, e := range i.completeBlocks {
		sr.StartWorker()
		go func(e *localBlock) {
			defer sr.FinishWorker()

			span, ctx := opentracing.StartSpanFromContext(ctx, "instance.searchLocalBlocks")
			defer span.Finish()

			blockID := e.BlockMeta().BlockID.String()

			span.LogFields(ot_log.Event("local block entry mtx acquired"))
			span.SetTag("blockID", blockID)

			var resp *deeppb.SearchResponse
			var err error

			opts := common.DefaultSearchOptions()
			if api.IsTraceQLQuery(req) {
				// note: we are creating new engine for each wal block,
				// and engine.Execute is parsing the query for each block
				resp, err = deepql.NewEngine().Execute(ctx, req, deepql.NewSnapshotResultFetcherWrapper(func(ctx context.Context, req deepql.FetchSnapshotRequest) (deepql.FetchSnapshotResponse, error) {
					return e.Fetch(ctx, req, opts)
				}))
			} else {
				resp, err = e.Search(ctx, req, opts)
			}

			if err == common.ErrUnsupported {
				level.Warn(log.Logger).Log("msg", "block does not support search", "blockID", e.BlockMeta().BlockID)
				return
			}
			if err != nil {
				level.Error(log.Logger).Log("msg", "error searching local block", "blockID", blockID, "err", err)
				sr.SetError(err)
				return
			}

			for _, t := range resp.Snapshots {
				sr.AddResult(ctx, t)
			}
			sr.AddBlockInspected()

			sr.AddBytesInspected(resp.Metrics.InspectedBytes)
			sr.AddTraceInspected(resp.Metrics.InspectedTraces)
		}(e)
	}
}

func (i *instance) SearchTags(ctx context.Context) (*deeppb.SearchTagsResponse, error) {
	userID, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, err
	}

	limit := i.limiter.limits.MaxBytesPerTagValuesQuery(userID)
	distinctValues := util.NewDistinctStringCollector(limit)

	search := func(s common.Searcher, dv *util.DistinctStringCollector) error {
		if s == nil {
			return nil
		}
		if dv.Exceeded() {
			return nil
		}
		err = s.SearchTags(ctx, dv.Collect, common.DefaultSearchOptions())
		if err != nil && err != common.ErrUnsupported {
			return fmt.Errorf("unexpected error searching tags: %w", err)
		}

		return nil
	}

	i.blocksMtx.RLock()
	defer i.blocksMtx.RUnlock()

	// search parquet wal/completing blocks/completed blocks
	if err = search(i.headBlock, distinctValues); err != nil {
		return nil, fmt.Errorf("unexpected error searching head block (%s): %w", i.headBlock.BlockMeta().BlockID, err)
	}
	for _, b := range i.completingBlocks {
		if err = search(b, distinctValues); err != nil {
			return nil, fmt.Errorf("unexpected error searching completing block (%s): %w", b.BlockMeta().BlockID, err)
		}
	}
	for _, b := range i.completeBlocks {
		if err = search(b, distinctValues); err != nil {
			return nil, fmt.Errorf("unexpected error searching complete block (%s): %w", b.BlockMeta().BlockID, err)
		}
	}

	if distinctValues.Exceeded() {
		level.Warn(log.Logger).Log("msg", "size of tags in instance exceeded limit, reduce cardinality or size of tags", "userID", userID, "limit", limit, "total", distinctValues.TotalDataSize())
	}

	return &deeppb.SearchTagsResponse{
		TagNames: distinctValues.Strings(),
	}, nil
}

func (i *instance) SearchTagValues(ctx context.Context, tagName string) (*deeppb.SearchTagValuesResponse, error) {
	userID, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, err
	}

	limit := i.limiter.limits.MaxBytesPerTagValuesQuery(userID)
	distinctValues := util.NewDistinctStringCollector(limit)

	search := func(s common.Searcher, dv *util.DistinctStringCollector) error {
		if s == nil {
			return nil
		}
		if dv.Exceeded() {
			return nil
		}
		err = s.SearchTagValues(ctx, tagName, dv.Collect, common.DefaultSearchOptions())
		if err != nil && err != common.ErrUnsupported {
			return fmt.Errorf("unexpected error searching tag values (%s): %w", tagName, err)
		}

		return nil
	}

	i.blocksMtx.RLock()
	defer i.blocksMtx.RUnlock()

	// search parquet wal/completing blocks/completed blocks
	if err = search(i.headBlock, distinctValues); err != nil {
		return nil, fmt.Errorf("unexpected error searching head block (%s): %w", i.headBlock.BlockMeta().BlockID, err)
	}
	for _, b := range i.completingBlocks {
		if err = search(b, distinctValues); err != nil {
			return nil, fmt.Errorf("unexpected error searching completing block (%s): %w", b.BlockMeta().BlockID, err)
		}
	}
	for _, b := range i.completeBlocks {
		if err = search(b, distinctValues); err != nil {
			return nil, fmt.Errorf("unexpected error searching complete block (%s): %w", b.BlockMeta().BlockID, err)
		}
	}

	if distinctValues.Exceeded() {
		level.Warn(log.Logger).Log("msg", "size of tag values in instance exceeded limit, reduce cardinality or size of tags", "tag", tagName, "userID", userID, "limit", limit, "total", distinctValues.TotalDataSize())
	}

	return &deeppb.SearchTagValuesResponse{
		TagValues: distinctValues.Strings(),
	}, nil
}

func (i *instance) SearchTagValuesV2(ctx context.Context, req *deeppb.SearchTagValuesRequest) (*deeppb.SearchTagValuesV2Response, error) {
	userID, err := user.ExtractOrgID(ctx)
	if err != nil {
		return nil, err
	}

	tag, err := deepql.ParseIdentifier(req.TagName)
	if err != nil {
		return nil, err
	}

	limit := i.limiter.limits.MaxBytesPerTagValuesQuery(userID)
	distinctValues := util.NewDistinctValueCollector[*deeppb.TagValue](limit, func(v *deeppb.TagValue) int { return len(v.Type) + len(v.Value) })

	cb := func(v deepql.Static) bool {
		tv := deeppb.TagValue{}

		switch v.Type {
		case deepql.TypeString:
			tv.Type = "string"
			tv.Value = v.S // avoid formatting

		case deepql.TypeBoolean:
			tv.Type = "bool"
			tv.Value = v.String()

		case deepql.TypeInt:
			tv.Type = "int"
			tv.Value = v.String()

		case deepql.TypeFloat:
			tv.Type = "float"
			tv.Value = v.String()

		case deepql.TypeDuration:
			tv.Type = "duration"
			tv.Value = v.String()
		}

		return distinctValues.Collect(&tv)
	}

	search := func(s common.Searcher, dv *util.DistinctValueCollector[*deeppb.TagValue]) error {
		if s == nil || dv.Exceeded() {
			return nil
		}

		err = s.SearchTagValuesV2(ctx, tag, cb, common.DefaultSearchOptions())
		if err != nil && err != common.ErrUnsupported {
			return fmt.Errorf("unexpected error searching tag values v2 (%s): %w", tag, err)
		}
		return nil
	}

	i.blocksMtx.RLock()
	defer i.blocksMtx.RUnlock()
	// head block
	if err = search(i.headBlock, distinctValues); err != nil {
		return nil, fmt.Errorf("unexpected error searching head block (%s): %w", i.headBlock.BlockMeta().BlockID, err)
	}

	// completing blocks
	for _, b := range i.completingBlocks {
		if err = search(b, distinctValues); err != nil {
			return nil, fmt.Errorf("unexpected error searching completing block (%s): %w", b.BlockMeta().BlockID, err)
		}
	}

	// completed blocks
	for _, b := range i.completeBlocks {
		if err = search(b, distinctValues); err != nil {
			return nil, fmt.Errorf("unexpected error searching complete block (%s): %w", b.BlockMeta().BlockID, err)
		}
	}

	if distinctValues.Exceeded() {
		level.Warn(log.Logger).Log("msg", "size of tag values in instance exceeded limit, reduce cardinality or size of tags", "tag", req.TagName, "userID", userID, "limit", limit, "total", distinctValues.TotalDataSize())
	}

	resp := &deeppb.SearchTagValuesV2Response{}

	for _, v := range distinctValues.Values() {
		v2 := v
		resp.TagValues = append(resp.TagValues, v2)
	}

	return resp, nil
}
