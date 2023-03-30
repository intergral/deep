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
	Trace deepdb.Config `yaml:"trace"`
}

// RegisterFlagsAndApplyDefaults registers the flags.
func (cfg *Config) RegisterFlagsAndApplyDefaults(prefix string, f *flag.FlagSet) {

	cfg.Trace.BlocklistPollFallback = true
	cfg.Trace.BlocklistPollConcurrency = deepdb.DefaultBlocklistPollConcurrency
	cfg.Trace.BlocklistPollTenantIndexBuilders = deepdb.DefaultTenantIndexBuilders

	f.StringVar(&cfg.Trace.Backend, util.PrefixConfig(prefix, "trace.backend"), "", "Trace backend (s3, azure, gcs, local)")
	f.DurationVar(&cfg.Trace.BlocklistPoll, util.PrefixConfig(prefix, "trace.blocklist_poll"), deepdb.DefaultBlocklistPoll, "Period at which to run the maintenance cycle.")

	cfg.Trace.WAL = &wal.Config{}
	f.StringVar(&cfg.Trace.WAL.Filepath, util.PrefixConfig(prefix, "trace.wal.path"), "/var/deep/wal", "Path at which store WAL blocks.")
	cfg.Trace.WAL.Encoding = backend.EncSnappy
	cfg.Trace.WAL.SearchEncoding = backend.EncNone
	cfg.Trace.WAL.IngestionSlack = 2 * time.Minute

	cfg.Trace.Search = &deepdb.SearchConfig{}
	cfg.Trace.Search.ChunkSizeBytes = deepdb.DefaultSearchChunkSizeBytes
	cfg.Trace.Search.PrefetchTraceCount = deepdb.DefaultPrefetchTraceCount
	cfg.Trace.Search.ReadBufferCount = deepdb.DefaultReadBufferCount
	cfg.Trace.Search.ReadBufferSizeBytes = deepdb.DefaultReadBufferSize

	cfg.Trace.Block = &common.BlockConfig{}
	f.Float64Var(&cfg.Trace.Block.BloomFP, util.PrefixConfig(prefix, "trace.block.v2-bloom-filter-false-positive"), DefaultBloomFP, "Bloom Filter False Positive.")
	f.IntVar(&cfg.Trace.Block.BloomShardSizeBytes, util.PrefixConfig(prefix, "trace.block.v2-bloom-filter-shard-size-bytes"), DefaultBloomShardSizeBytes, "Bloom Filter Shard Size in bytes.")
	f.IntVar(&cfg.Trace.Block.IndexDownsampleBytes, util.PrefixConfig(prefix, "trace.block.v2-index-downsample-bytes"), DefaultIndexDownSampleBytes, "Number of bytes (before compression) per index record.")
	f.IntVar(&cfg.Trace.Block.IndexPageSizeBytes, util.PrefixConfig(prefix, "trace.block.v2-index-page-size-bytes"), DefaultIndexPageSizeBytes, "Number of bytes per index page.")
	cfg.Trace.Block.Version = encoding.DefaultEncoding().Version()
	cfg.Trace.Block.Encoding = backend.EncZstd
	cfg.Trace.Block.SearchEncoding = backend.EncSnappy
	cfg.Trace.Block.SearchPageSizeBytes = 1024 * 1024 // 1 MB
	cfg.Trace.Block.RowGroupSizeBytes = 100_000_000   // 100 MB

	cfg.Trace.Azure = &azure.Config{}
	f.StringVar(&cfg.Trace.Azure.StorageAccountName, util.PrefixConfig(prefix, "trace.azure.storage_account_name"), "", "Azure storage account name.")
	f.Var(&cfg.Trace.Azure.StorageAccountKey, util.PrefixConfig(prefix, "trace.azure.storage_account_key"), "Azure storage access key.")
	f.StringVar(&cfg.Trace.Azure.ContainerName, util.PrefixConfig(prefix, "trace.azure.container_name"), "", "Azure container name to store blocks in.")
	f.StringVar(&cfg.Trace.Azure.Endpoint, util.PrefixConfig(prefix, "trace.azure.endpoint"), "blob.core.windows.net", "Azure endpoint to push blocks to.")
	f.IntVar(&cfg.Trace.Azure.MaxBuffers, util.PrefixConfig(prefix, "trace.azure.max_buffers"), 4, "Number of simultaneous uploads.")
	cfg.Trace.Azure.BufferSize = 3 * 1024 * 1024
	cfg.Trace.Azure.HedgeRequestsUpTo = 2

	cfg.Trace.S3 = &s3.Config{}
	f.StringVar(&cfg.Trace.S3.Bucket, util.PrefixConfig(prefix, "trace.s3.bucket"), "", "s3 bucket to store blocks in.")
	f.StringVar(&cfg.Trace.S3.Endpoint, util.PrefixConfig(prefix, "trace.s3.endpoint"), "", "s3 endpoint to push blocks to.")
	f.StringVar(&cfg.Trace.S3.AccessKey, util.PrefixConfig(prefix, "trace.s3.access_key"), "", "s3 access key.")
	f.Var(&cfg.Trace.S3.SecretKey, util.PrefixConfig(prefix, "trace.s3.secret_key"), "s3 secret key.")
	f.Var(&cfg.Trace.S3.SessionToken, util.PrefixConfig(prefix, "trace.s3.session_token"), "s3 session token.")
	cfg.Trace.S3.HedgeRequestsUpTo = 2

	cfg.Trace.GCS = &gcs.Config{}
	f.StringVar(&cfg.Trace.GCS.BucketName, util.PrefixConfig(prefix, "trace.gcs.bucket"), "", "gcs bucket to store traces in.")
	cfg.Trace.GCS.ChunkBufferSize = 10 * 1024 * 1024
	cfg.Trace.GCS.HedgeRequestsUpTo = 2

	cfg.Trace.Local = &local.Config{}
	f.StringVar(&cfg.Trace.Local.Path, util.PrefixConfig(prefix, "trace.local.path"), "", "path to store traces at.")

	cfg.Trace.BackgroundCache = &cache.BackgroundConfig{}
	cfg.Trace.BackgroundCache.WriteBackBuffer = 10000
	cfg.Trace.BackgroundCache.WriteBackGoroutines = 10

	cfg.Trace.Pool = &pool.Config{}
	f.IntVar(&cfg.Trace.Pool.MaxWorkers, util.PrefixConfig(prefix, "trace.pool.max-workers"), 400, "Workers in the worker pool.")
	f.IntVar(&cfg.Trace.Pool.QueueDepth, util.PrefixConfig(prefix, "trace.pool.queue-depth"), 20000, "Work item queue depth.")
}
