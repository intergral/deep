package storage

import (
	"context"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/services"

	"github.com/intergral/deep/pkg/deepdb"
	"github.com/intergral/deep/pkg/usagestats"
)

var (
	statCache               = usagestats.NewString("storage_cache")
	statBackend             = usagestats.NewString("storage_backend")
	statWalEncoding         = usagestats.NewString("storage_wal_encoding")
	statWalSearchEncoding   = usagestats.NewString("storage_wal_search_encoding")
	statBlockEncoding       = usagestats.NewString("storage_block_encoding")
	statBlockSearchEncoding = usagestats.NewString("storage_block_search_encoding")
)

// Store wraps the deepdb storage layer
type Store interface {
	services.Service

	deepdb.Reader
	deepdb.Writer
	deepdb.Compactor
}

type store struct {
	services.Service

	cfg Config

	deepdb.Reader
	deepdb.Writer
	deepdb.Compactor
}

// NewStore creates a new Tempo Store using configuration supplied.
func NewStore(cfg Config, logger log.Logger) (Store, error) {

	statCache.Set(cfg.Trace.Cache)
	statBackend.Set(cfg.Trace.Backend)
	statWalEncoding.Set(cfg.Trace.WAL.Encoding.String())
	statWalSearchEncoding.Set(cfg.Trace.WAL.SearchEncoding.String())
	statBlockEncoding.Set(cfg.Trace.Block.Encoding.String())
	statBlockSearchEncoding.Set(cfg.Trace.Block.SearchEncoding.String())

	r, w, c, err := deepdb.New(&cfg.Trace, logger)
	if err != nil {
		return nil, err
	}

	s := &store{
		cfg:       cfg,
		Reader:    r,
		Writer:    w,
		Compactor: c,
	}

	s.Service = services.NewIdleService(s.starting, s.stopping)
	return s, nil
}

func (s *store) starting(_ context.Context) error {
	return nil
}

func (s *store) stopping(_ error) error {
	s.Reader.Shutdown()

	return nil
}
