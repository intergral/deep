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

package deepdb

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/intergral/deep/pkg/deeppb"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/deepql"
	"io"
	"time"

	gkLog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	pkg_cache "github.com/intergral/deep/pkg/cache"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/azure"
	"github.com/intergral/deep/pkg/deepdb/backend/cache"
	"github.com/intergral/deep/pkg/deepdb/backend/cache/memcached"
	"github.com/intergral/deep/pkg/deepdb/backend/cache/redis"
	"github.com/intergral/deep/pkg/deepdb/backend/gcs"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/backend/s3"
	"github.com/intergral/deep/pkg/deepdb/blocklist"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/pool"
	"github.com/intergral/deep/pkg/deepdb/wal"
	"github.com/intergral/deep/pkg/util/log"
)

const (
	// BlockIDMin is the minimum possible value for a block id as a string
	BlockIDMin = "00000000-0000-0000-0000-000000000000"
	// BlockIDMax is the maximum possible value for a block id as a string
	BlockIDMax = "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF"
)

var (
	metricRetentionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "deepdb",
		Name:      "retention_duration_seconds",
		Help:      "Records the amount of time to perform retention tasks.",
		Buckets:   prometheus.ExponentialBuckets(.25, 2, 6),
	})
	metricRetentionErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "deepdb",
		Name:      "retention_errors_total",
		Help:      "Total number of times an error occurred while performing retention tasks.",
	})
	metricMarkedForDeletion = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "deepdb",
		Name:      "retention_marked_for_deletion_total",
		Help:      "Total number of blocks marked for deletion.",
	})
	metricDeleted = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "deepdb",
		Name:      "retention_deleted_total",
		Help:      "Total number of blocks deleted.",
	})
)

type Writer interface {
	WriteTracepointBlock(ctx context.Context, orgId string, reader *bytes.Reader, size int64) error
	WriteBlock(ctx context.Context, block WriteableBlock) error
	CompleteBlock(ctx context.Context, block common.WALBlock) (common.BackendBlock, error)
	CompleteBlockWithBackend(ctx context.Context, block common.WALBlock, r backend.Reader, w backend.Writer) (common.BackendBlock, error)
	WAL() *wal.WAL
}

type IterateObjectCallback func(id common.ID, obj []byte) bool

type Reader interface {
	ReadTracepointBlock(ctx context.Context, orgId string) (io.ReadCloser, int64, error)
	FindSnapshot(ctx context.Context, tenantID string, id common.ID, blockStart string, blockEnd string, timeStart int64, timeEnd int64) (*deep_tp.Snapshot, []error, error)

	Search(ctx context.Context, meta *backend.BlockMeta, req *deeppb.SearchRequest, opts common.SearchOptions) (*deeppb.SearchResponse, error)
	Fetch(ctx context.Context, meta *backend.BlockMeta, req deepql.FetchSnapshotRequest, opts common.SearchOptions) (deepql.FetchSnapshotResponse, error)
	BlockMetas(tenantID string) []*backend.BlockMeta
	EnablePolling(sharder blocklist.JobSharder)

	Shutdown()
}

type Compactor interface {
	EnableCompaction(ctx context.Context, cfg *CompactorConfig, sharder CompactorSharder, overrides CompactorOverrides)
}

type CompactorSharder interface {
	Combine(dataEncoding string, tenantID string, objs ...[]byte) ([]byte, bool, error)
	Owns(hash string) bool
	RecordDiscardedSpans(count int, tenantID string, traceID string)
}

type CompactorOverrides interface {
	BlockRetentionForTenant(tenantID string) time.Duration
	MaxBytesPerTraceForTenant(tenantID string) int
}

type WriteableBlock interface {
	BlockMeta() *backend.BlockMeta
	Write(ctx context.Context, w backend.Writer) error
}

type readerWriter struct {
	r backend.Reader
	w backend.Writer
	c backend.Compactor

	uncachedReader backend.Reader
	uncachedWriter backend.Writer

	wal  *wal.WAL
	pool *pool.Pool

	logger gkLog.Logger
	cfg    *Config

	blocklistPoller *blocklist.Poller
	blocklist       *blocklist.List

	compactorCfg          *CompactorConfig
	compactorSharder      CompactorSharder
	compactorOverrides    CompactorOverrides
	compactorTenantOffset uint
}

// New creates a new deepdb
func New(cfg *Config, logger gkLog.Logger) (Reader, Writer, Compactor, error) {
	var rawR backend.RawReader
	var rawW backend.RawWriter
	var c backend.Compactor

	err := validateConfig(cfg)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid config while creating deepdb: %w", err)
	}

	switch cfg.Backend {
	case "local":
		rawR, rawW, c, err = local.New(cfg.Local)
	case "gcs":
		rawR, rawW, c, err = gcs.New(cfg.GCS)
	case "s3":
		rawR, rawW, c, err = s3.New(cfg.S3)
	case "azure":
		rawR, rawW, c, err = azure.New(cfg.Azure)
	default:
		err = fmt.Errorf("unknown backend %s", cfg.Backend)
	}

	if err != nil {
		return nil, nil, nil, err
	}

	uncachedReader := backend.NewReader(rawR)
	uncachedWriter := backend.NewWriter(rawW)

	var cacheBackend pkg_cache.Cache

	switch cfg.Cache {
	case "redis":
		cacheBackend = redis.NewClient(cfg.Redis, cfg.BackgroundCache, logger)
	case "memcached":
		cacheBackend = memcached.NewClient(cfg.Memcached, cfg.BackgroundCache, logger)
	}

	if cacheBackend != nil {
		rawR, rawW, err = cache.NewCache(rawR, rawW, cacheBackend)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	r := backend.NewReader(rawR)
	w := backend.NewWriter(rawW)
	rw := &readerWriter{
		c:              c,
		r:              r,
		uncachedReader: uncachedReader,
		uncachedWriter: uncachedWriter,
		w:              w,
		cfg:            cfg,
		logger:         logger,
		pool:           pool.NewPool(cfg.Pool),
		blocklist:      blocklist.New(),
	}

	rw.wal, err = wal.New(rw.cfg.WAL)
	if err != nil {
		return nil, nil, nil, err
	}

	return rw, rw, rw, nil
}

func (rw *readerWriter) WriteTracepointBlock(ctx context.Context, orgId string, data *bytes.Reader, size int64) error {
	return rw.w.WriteTracepointBlock(ctx, orgId, data, size)
}

func (rw *readerWriter) WriteBlock(ctx context.Context, c WriteableBlock) error {
	w := rw.getWriterForBlock(c.BlockMeta(), time.Now())
	return c.Write(ctx, w)
}

// CompleteBlock iterates the given WAL block and flushes it to the deepdb backend.
func (rw *readerWriter) CompleteBlock(ctx context.Context, block common.WALBlock) (common.BackendBlock, error) {
	return rw.CompleteBlockWithBackend(ctx, block, rw.r, rw.w)
}

// CompleteBlock iterates the given WAL block but flushes it to the given backend instead of the default deepdb backend. The
// new block will have the same ID as the input block.
func (rw *readerWriter) CompleteBlockWithBackend(ctx context.Context, block common.WALBlock, r backend.Reader, w backend.Writer) (common.BackendBlock, error) {

	// The destination block format:
	vers, err := encoding.FromVersion(rw.cfg.Block.Version)
	if err != nil {
		return nil, err
	}

	// force flush anything left in the wal
	err = block.Flush()
	if err != nil {
		return nil, fmt.Errorf("error flushing wal block: %w", err)
	}

	iter, err := block.Iterator()
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	walMeta := block.BlockMeta()

	inMeta := &backend.BlockMeta{
		// From the wal block
		TenantID:     walMeta.TenantID,
		BlockID:      walMeta.BlockID,
		TotalObjects: walMeta.TotalObjects,
		StartTime:    walMeta.StartTime,
		EndTime:      walMeta.EndTime,
		DataEncoding: walMeta.DataEncoding,

		// Other
		Encoding: rw.cfg.Block.Encoding,
	}

	newMeta, err := vers.CreateBlock(ctx, rw.cfg.Block, inMeta, iter, r, w)
	if err != nil {
		return nil, errors.Wrap(err, "error creating block")
	}

	backendBlock, err := encoding.OpenBlock(newMeta, r)
	if err != nil {
		return nil, errors.Wrap(err, "error opening new block")
	}

	return backendBlock, nil
}

func (rw *readerWriter) WAL() *wal.WAL {
	return rw.wal
}

func (rw *readerWriter) BlockMetas(tenantID string) []*backend.BlockMeta {
	return rw.blocklist.Metas(tenantID)
}

func (rw *readerWriter) ReadTracepointBlock(ctx context.Context, orgId string) (io.ReadCloser, int64, error) {
	return rw.r.ReadTracepointBlock(ctx, orgId)
}

func (rw *readerWriter) FindSnapshot(ctx context.Context, tenantID string, id common.ID, blockStart string, blockEnd string, timeStart int64, timeEnd int64) (*deep_tp.Snapshot, []error, error) {
	// tracing instrumentation
	logger := log.WithContext(ctx, log.Logger)
	span, ctx := opentracing.StartSpanFromContext(ctx, "store.FindSnapshot")
	defer span.Finish()

	blockStartUUID, err := uuid.Parse(blockStart)
	if err != nil {
		return nil, nil, err
	}
	blockStartBytes, err := blockStartUUID.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}
	blockEndUUID, err := uuid.Parse(blockEnd)
	if err != nil {
		return nil, nil, err
	}
	blockEndBytes, err := blockEndUUID.MarshalBinary()
	if err != nil {
		return nil, nil, err
	}

	// gather appropriate blocks
	blocklist := rw.blocklist.Metas(tenantID)
	compactedBlocklist := rw.blocklist.CompactedMetas(tenantID)
	copiedBlocklist := make([]interface{}, 0, len(blocklist))
	blocksSearched := 0
	compactedBlocksSearched := 0

	for _, b := range blocklist {
		if includeBlock(b, id, blockStartBytes, blockEndBytes, timeStart, timeEnd) {
			copiedBlocklist = append(copiedBlocklist, b)
			blocksSearched++
		}
	}
	for _, c := range compactedBlocklist {
		if includeCompactedBlock(c, id, blockStartBytes, blockEndBytes, rw.cfg.BlocklistPoll, timeStart, timeEnd) {
			copiedBlocklist = append(copiedBlocklist, &c.BlockMeta)
			compactedBlocksSearched++
		}
	}
	if len(copiedBlocklist) == 0 {
		return nil, nil, nil
	}

	opts := common.DefaultSearchOptions()
	if rw.cfg != nil && rw.cfg.Search != nil {
		rw.cfg.Search.ApplyToOptions(&opts)
	}

	curTime := time.Now()
	jobsResults, funcErrs, err := rw.pool.RunJobs(ctx, copiedBlocklist, func(ctx context.Context, payload interface{}) (interface{}, error) {
		meta := payload.(*backend.BlockMeta)
		r := rw.getReaderForBlock(meta, curTime)
		block, err := encoding.OpenBlock(meta, r)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("error opening block for reading, blockID: %s", meta.BlockID.String()))
		}

		foundObject, err := block.FindSnapshotByID(ctx, id, opts)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("error finding trace by id, blockID: %s", meta.BlockID.String()))
		}

		level.Info(logger).Log("msg", "searching for trace in block", "findTraceID", hex.EncodeToString(id), "block", meta.BlockID, "found", foundObject != nil)
		return foundObject, nil
	})

	var result *deep_tp.Snapshot
	for i := range jobsResults {
		res := jobsResults[i].(*deep_tp.Snapshot)
		if res != nil {
			result = res
		}
	}

	span.SetTag("blockErrs", len(funcErrs))
	span.SetTag("liveBlocks", len(blocklist))
	span.SetTag("liveBlocksSearched", blocksSearched)
	span.SetTag("compactedBlocks", len(compactedBlocklist))
	span.SetTag("compactedBlocksSearched", compactedBlocksSearched)

	return result, funcErrs, err
}

// Search the given block.  This method takes the pre-loaded block meta instead of a block ID, which
// eliminates a read per search request.
func (rw *readerWriter) Search(ctx context.Context, meta *backend.BlockMeta, req *deeppb.SearchRequest, opts common.SearchOptions) (*deeppb.SearchResponse, error) {
	block, err := encoding.OpenBlock(meta, rw.r)
	if err != nil {
		return nil, err
	}

	rw.cfg.Search.ApplyToOptions(&opts)
	return block.Search(ctx, req, opts)
}

func (rw *readerWriter) Fetch(ctx context.Context, meta *backend.BlockMeta, req deepql.FetchSnapshotRequest, opts common.SearchOptions) (deepql.FetchSnapshotResponse, error) {
	block, err := encoding.OpenBlock(meta, rw.r)
	if err != nil {
		return deepql.FetchSnapshotResponse{}, err
	}

	rw.cfg.Search.ApplyToOptions(&opts)
	return block.Fetch(ctx, req, opts)
}

func (rw *readerWriter) Shutdown() {
	// todo: stop blocklist poll
	rw.pool.Shutdown()
	rw.r.Shutdown()
}

// EnableCompaction activates the compaction/retention loops
func (rw *readerWriter) EnableCompaction(ctx context.Context, cfg *CompactorConfig, c CompactorSharder, overrides CompactorOverrides) {
	// Set default if needed. This is mainly for tests.
	if cfg.RetentionConcurrency == 0 {
		cfg.RetentionConcurrency = DefaultRetentionConcurrency
	}

	rw.compactorCfg = cfg
	rw.compactorSharder = c
	rw.compactorOverrides = overrides

	if rw.cfg.BlocklistPoll == 0 {
		level.Info(rw.logger).Log("msg", "polling cycle unset. compaction and retention disabled")
		return
	}

	if cfg != nil {
		level.Info(rw.logger).Log("msg", "compaction and retention enabled.")
		go rw.compactionLoop(ctx)
		go rw.retentionLoop(ctx)
	}
}

// EnablePolling activates the polling loop. Pass nil if this component
//
//	should never be a tenant index builder.
func (rw *readerWriter) EnablePolling(sharder blocklist.JobSharder) {
	if sharder == nil {
		sharder = blocklist.OwnsNothingSharder
	}

	if rw.cfg.BlocklistPoll == 0 {
		rw.cfg.BlocklistPoll = DefaultBlocklistPoll
	}

	if rw.cfg.BlocklistPollConcurrency == 0 {
		rw.cfg.BlocklistPollConcurrency = DefaultBlocklistPollConcurrency
	}

	if rw.cfg.BlocklistPollTenantIndexBuilders <= 0 {
		rw.cfg.BlocklistPollTenantIndexBuilders = DefaultTenantIndexBuilders
	}

	level.Info(rw.logger).Log("msg", "polling enabled", "interval", rw.cfg.BlocklistPoll, "concurrency", rw.cfg.BlocklistPollConcurrency)

	blocklistPoller := blocklist.NewPoller(&blocklist.PollerConfig{
		PollConcurrency:     rw.cfg.BlocklistPollConcurrency,
		PollFallback:        rw.cfg.BlocklistPollFallback,
		TenantIndexBuilders: rw.cfg.BlocklistPollTenantIndexBuilders,
		StaleTenantIndex:    rw.cfg.BlocklistPollStaleTenantIndex,
		PollJitterMs:        rw.cfg.BlocklistPollJitterMs,
	}, sharder, rw.r, rw.c, rw.w, rw.logger)

	rw.blocklistPoller = blocklistPoller

	// do the first poll cycle synchronously. this will allow the caller to know
	// that when this method returns the block list is updated
	rw.pollBlocklist()

	go rw.pollingLoop()
}

func (rw *readerWriter) pollingLoop() {
	ticker := time.NewTicker(rw.cfg.BlocklistPoll)
	for range ticker.C {
		rw.pollBlocklist()
	}
}

func (rw *readerWriter) pollBlocklist() {
	blocklist, compactedBlocklist, err := rw.blocklistPoller.Do()
	if err != nil {
		level.Error(rw.logger).Log("msg", "failed to poll blocklist. using previously polled lists", "err", err)
		return
	}

	rw.blocklist.ApplyPollResults(blocklist, compactedBlocklist)
}

func (rw *readerWriter) shouldCache(meta *backend.BlockMeta, curTime time.Time) bool {
	// compaction level is _atleast_ CacheMinCompactionLevel
	if rw.cfg.CacheMinCompactionLevel > 0 && meta.CompactionLevel < rw.cfg.CacheMinCompactionLevel {
		return false
	}

	// block is not older than CacheMaxBlockAge
	if rw.cfg.CacheMaxBlockAge > 0 && curTime.Sub(meta.StartTime) > rw.cfg.CacheMaxBlockAge {
		return false
	}

	return true
}

func (rw *readerWriter) getReaderForBlock(meta *backend.BlockMeta, curTime time.Time) backend.Reader {
	if rw.shouldCache(meta, curTime) {
		return rw.r
	}

	return rw.uncachedReader
}

func (rw *readerWriter) getWriterForBlock(meta *backend.BlockMeta, curTime time.Time) backend.Writer {
	if rw.shouldCache(meta, curTime) {
		return rw.w
	}

	return rw.uncachedWriter
}

// includeBlock indicates whether a given block should be included in a backend search
func includeBlock(b *backend.BlockMeta, _ common.ID, blockStart []byte, blockEnd []byte, timeStart int64, timeEnd int64) bool {
	// todo: restore this functionality once it works. min/max ids are currently not recorded
	//    https://github.com/intergral/deep/issues/1903
	//  correctly in a block
	// if bytes.Compare(id, b.MinID) == -1 || bytes.Compare(id, b.MaxID) == 1 {
	// 	return false
	// }

	if timeStart != 0 && timeEnd != 0 {
		if b.StartTime.Unix() >= timeEnd || b.EndTime.Unix() <= timeStart {
			return false
		}
	}

	blockIDBytes, _ := b.BlockID.MarshalBinary()
	// check block is in shard boundaries
	// blockStartBytes <= blockIDBytes <= blockEndBytes
	if bytes.Compare(blockIDBytes, blockStart) == -1 || bytes.Compare(blockIDBytes, blockEnd) == 1 {
		return false
	}

	return true
}

// if block is compacted within lookback period, and is within shard ranges, include it in search
func includeCompactedBlock(c *backend.CompactedBlockMeta, id common.ID, blockStart []byte, blockEnd []byte, poll time.Duration, timeStart int64, timeEnd int64) bool {
	lookback := time.Now().Add(-(2 * poll))
	if c.CompactedTime.Before(lookback) {
		return false
	}
	return includeBlock(&c.BlockMeta, id, blockStart, blockEnd, timeStart, timeEnd)
}
