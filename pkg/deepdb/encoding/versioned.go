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

package encoding

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/google/uuid"

	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
)

// VersionedEncoding represents a backend block version, and the methods to
// read/write them.
type VersionedEncoding interface {
	Version() string

	// OpenBlock for reading
	OpenBlock(meta *backend.BlockMeta, r backend.Reader) (common.BackendBlock, error)

	// NewCompactor creates a Compactor that can be used to combine blocks of this
	// encoding. It is expected to use internal details for efficiency.
	NewCompactor(common.CompactionOptions) common.Compactor

	// CreateBlock with the given attributes and trace contents.
	// BlockMeta is used as a container for many options. Required fields:
	// * BlockID
	// * TenantID
	// * Encoding
	// * DataEncoding
	// * StartTime
	// * EndTime
	// * TotalObjects
	CreateBlock(ctx context.Context, cfg *common.BlockConfig, meta *backend.BlockMeta, i common.Iterator, r backend.Reader, to backend.Writer) (*backend.BlockMeta, error)

	// CopyBlock from one backend to another.
	CopyBlock(ctx context.Context, meta *backend.BlockMeta, from backend.Reader, to backend.Writer) error

	// MigrateBlock from one backend and tenant to another.
	MigrateBlock(ctx context.Context, fromMeta, toMeta *backend.BlockMeta, from backend.Reader, to backend.Writer) error

	// OpenWALBlock opens an existing appendable block for the WAL
	OpenWALBlock(filename string, path string, ingestionSlack time.Duration, additionalStartSlack time.Duration) (common.WALBlock, error, error)

	// CreateWALBlock creates a new appendable block for the WAL
	CreateWALBlock(id uuid.UUID, tenantID string, filepath string, e backend.Encoding, dataEncoding string, ingestionSlack time.Duration) (common.WALBlock, error)

	// OwnsWALBlock indicates if this encoding owns the WAL block
	OwnsWALBlock(entry fs.DirEntry) bool
}

// FromVersion returns a versioned encoding for the provided string
func FromVersion(v string) (VersionedEncoding, error) {
	switch v {
	case vparquet.VersionString:
		return vparquet.Encoding{}, nil
	}

	return nil, fmt.Errorf("%s is not a valid block version", v)
}

// DefaultEncoding for newly written blocks.
func DefaultEncoding() VersionedEncoding {
	return vparquet.Encoding{}
}

// AllEncodings returns all encodings
func AllEncodings() []VersionedEncoding {
	return []VersionedEncoding{
		vparquet.Encoding{},
	}
}

// OpenBlock for reading in the backend. It automatically chooes the encoding for the given block.
func OpenBlock(meta *backend.BlockMeta, r backend.Reader) (common.BackendBlock, error) {
	v, err := FromVersion(meta.Version)
	if err != nil {
		return nil, err
	}
	return v.OpenBlock(meta, r)
}

// CopyBlock from one backend to another. It automatically chooses the encoding for the given block.
func CopyBlock(ctx context.Context, meta *backend.BlockMeta, from backend.Reader, to backend.Writer) error {
	v, err := FromVersion(meta.Version)
	if err != nil {
		return err
	}
	return v.CopyBlock(ctx, meta, from, to)
}
