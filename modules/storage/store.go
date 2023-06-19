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

	deepdb.TracepointWriter
	deepdb.TracepointReader
}

type store struct {
	services.Service

	cfg Config

	deepdb.Reader
	deepdb.Writer
	deepdb.Compactor

	deepdb.TracepointWriter
	deepdb.TracepointReader
}

// NewStore creates a new Tempo Store using configuration supplied.
func NewStore(cfg Config, logger log.Logger) (Store, error) {

	statCache.Set(cfg.TracePoint.Cache)
	statBackend.Set(cfg.TracePoint.Backend)
	statWalEncoding.Set(cfg.TracePoint.WAL.Encoding.String())
	statWalSearchEncoding.Set(cfg.TracePoint.WAL.SearchEncoding.String())
	statBlockEncoding.Set(cfg.TracePoint.Block.Encoding.String())
	statBlockSearchEncoding.Set(cfg.TracePoint.Block.SearchEncoding.String())

	r, w, c, err := deepdb.New(&cfg.TracePoint, logger)
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
