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

package storage

import (
	"flag"
	"time"

	"github.com/intergral/deep/pkg/cache"

	"github.com/intergral/deep/pkg/deepdb"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/azure"
	"github.com/intergral/deep/pkg/deepdb/backend/gcs"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/backend/s3"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/pool"
	"github.com/intergral/deep/pkg/deepdb/wal"
	"github.com/intergral/deep/pkg/util"
)

const (
	DefaultBloomFP              = .01
	DefaultBloomShardSizeBytes  = 100 * 1024
	DefaultIndexDownSampleBytes = 1024 * 1024
	DefaultIndexPageSizeBytes   = 250 * 1024
)

// Config is the Tempo storage configuration
type Config struct {
	TracePoint deepdb.Config `yaml:"tracepoint"`
}

// RegisterFlagsAndApplyDefaults registers the flags.
func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {

	cfg.TracePoint.BlocklistPollFallback = true
	cfg.TracePoint.BlocklistPollConcurrency = deepdb.DefaultBlocklistPollConcurrency
	cfg.TracePoint.BlocklistPollTenantIndexBuilders = deepdb.DefaultTenantIndexBuilders

	f.StringVar(&cfg.TracePoint.Backend, util.PrefixConfig(prefix, "tracepoint.backend"), "", "Trace backend (s3, azure, gcs, local)")
	f.DurationVar(&cfg.TracePoint.BlocklistPoll, util.PrefixConfig(prefix, "tracepoint.blocklist_poll"), deepdb.DefaultBlocklistPoll, "Period at which to run the maintenance cycle.")

	cfg.TracePoint.WAL = &wal.Config{}
	f.StringVar(&cfg.TracePoint.WAL.Filepath, util.PrefixConfig(prefix, "tracepoint.wal.path"), "/var/deep/wal", "Path at which store WAL blocks.")
	cfg.TracePoint.WAL.Encoding = backend.EncSnappy
	cfg.TracePoint.WAL.SearchEncoding = backend.EncNone
	cfg.TracePoint.WAL.IngestionSlack = 2 * time.Minute

	cfg.TracePoint.Search = &deepdb.SearchConfig{}
	cfg.TracePoint.Search.ChunkSizeBytes = deepdb.DefaultSearchChunkSizeBytes
	cfg.TracePoint.Search.PrefetchTraceCount = deepdb.DefaultPrefetchTraceCount
	cfg.TracePoint.Search.ReadBufferCount = deepdb.DefaultReadBufferCount
	cfg.TracePoint.Search.ReadBufferSizeBytes = deepdb.DefaultReadBufferSize

	cfg.TracePoint.Block = &common.BlockConfig{}
	f.Float64Var(&cfg.TracePoint.Block.BloomFP, util.PrefixConfig(prefix, "tracepoint.block.v2-bloom-filter-false-positive"), DefaultBloomFP, "Bloom Filter False Positive.")
	f.IntVar(&cfg.TracePoint.Block.BloomShardSizeBytes, util.PrefixConfig(prefix, "tracepoint.block.v2-bloom-filter-shard-size-bytes"), DefaultBloomShardSizeBytes, "Bloom Filter Shard Size in bytes.")
	f.IntVar(&cfg.TracePoint.Block.IndexDownsampleBytes, util.PrefixConfig(prefix, "tracepoint.block.v2-index-downsample-bytes"), DefaultIndexDownSampleBytes, "Number of bytes (before compression) per index record.")
	f.IntVar(&cfg.TracePoint.Block.IndexPageSizeBytes, util.PrefixConfig(prefix, "tracepoint.block.v2-index-page-size-bytes"), DefaultIndexPageSizeBytes, "Number of bytes per index page.")
	cfg.TracePoint.Block.Version = encoding.DefaultEncoding().Version()
	cfg.TracePoint.Block.Encoding = backend.EncZstd
	cfg.TracePoint.Block.SearchEncoding = backend.EncSnappy
	cfg.TracePoint.Block.SearchPageSizeBytes = 1024 * 1024 // 1 MB
	cfg.TracePoint.Block.RowGroupSizeBytes = 100_000_000   // 100 MB

	cfg.TracePoint.Azure = &azure.Config{}
	f.StringVar(&cfg.TracePoint.Azure.StorageAccountName, util.PrefixConfig(prefix, "tracepoint.azure.storage_account_name"), "", "Azure storage account name.")
	f.Var(&cfg.TracePoint.Azure.StorageAccountKey, util.PrefixConfig(prefix, "tracepoint.azure.storage_account_key"), "Azure storage access key.")
	f.StringVar(&cfg.TracePoint.Azure.ContainerName, util.PrefixConfig(prefix, "tracepoint.azure.container_name"), "", "Azure container name to store blocks in.")
	f.StringVar(&cfg.TracePoint.Azure.Endpoint, util.PrefixConfig(prefix, "tracepoint.azure.endpoint"), "blob.core.windows.net", "Azure endpoint to push blocks to.")
	f.IntVar(&cfg.TracePoint.Azure.MaxBuffers, util.PrefixConfig(prefix, "tracepoint.azure.max_buffers"), 4, "Number of simultaneous uploads.")
	cfg.TracePoint.Azure.BufferSize = 3 * 1024 * 1024
	cfg.TracePoint.Azure.HedgeRequestsUpTo = 2

	cfg.TracePoint.S3 = &s3.Config{}
	f.StringVar(&cfg.TracePoint.S3.Bucket, util.PrefixConfig(prefix, "tracepoint.s3.bucket"), "", "s3 bucket to store blocks in.")
	f.StringVar(&cfg.TracePoint.S3.Endpoint, util.PrefixConfig(prefix, "tracepoint.s3.endpoint"), "", "s3 endpoint to push blocks to.")
	f.StringVar(&cfg.TracePoint.S3.AccessKey, util.PrefixConfig(prefix, "tracepoint.s3.access_key"), "", "s3 access key.")
	f.Var(&cfg.TracePoint.S3.SecretKey, util.PrefixConfig(prefix, "tracepoint.s3.secret_key"), "s3 secret key.")
	f.Var(&cfg.TracePoint.S3.SessionToken, util.PrefixConfig(prefix, "tracepoint.s3.session_token"), "s3 session token.")
	cfg.TracePoint.S3.HedgeRequestsUpTo = 2

	cfg.TracePoint.GCS = &gcs.Config{}
	f.StringVar(&cfg.TracePoint.GCS.BucketName, util.PrefixConfig(prefix, "tracepoint.gcs.bucket"), "", "gcs bucket to store traces in.")
	cfg.TracePoint.GCS.ChunkBufferSize = 10 * 1024 * 1024
	cfg.TracePoint.GCS.HedgeRequestsUpTo = 2

	cfg.TracePoint.Local = &local.Config{}
	f.StringVar(&cfg.TracePoint.Local.Path, util.PrefixConfig(prefix, "tracepoint.local.path"), "", "path to store traces at.")

	cfg.TracePoint.BackgroundCache = &cache.BackgroundConfig{}
	cfg.TracePoint.BackgroundCache.WriteBackBuffer = 10000
	cfg.TracePoint.BackgroundCache.WriteBackGoroutines = 10

	cfg.TracePoint.Pool = &pool.Config{}
	f.IntVar(&cfg.TracePoint.Pool.MaxWorkers, util.PrefixConfig(prefix, "tracepoint.pool.max-workers"), 400, "Workers in the worker pool.")
	f.IntVar(&cfg.TracePoint.Pool.QueueDepth, util.PrefixConfig(prefix, "tracepoint.pool.queue-depth"), 20000, "Work item queue depth.")
}
