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

package vparquet

import (
	"context"

	"github.com/intergral/deep/pkg/deepql"

	"github.com/intergral/deep/pkg/parquetquery"
)

type snapshotMetadataIterator struct {
	iter parquetquery.Iterator
}

const (
	resEntriesIDOffset          = 6
	resEntriesTSOffset          = 5
	resEntriesDurationOffset    = 4
	resEntriesServiceNameOffset = 3
	resEntriesPathOffset        = 2
	resEntriesLineNoOffset      = 1
)

var _ deepql.SnapshotResultIterator = (*snapshotMetadataIterator)(nil)

func newSnapshotMetadataIterator(iter parquetquery.Iterator) *snapshotMetadataIterator {
	return &snapshotMetadataIterator{
		iter: iter,
	}
}

func (i *snapshotMetadataIterator) Next(context.Context) (*deepql.SnapshotResult, error) {
	res, err := i.iter.Next()
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	entries := len(res.Entries)
	return &deepql.SnapshotResult{
		SnapshotID:         res.Entries[entries-resEntriesIDOffset].Value.Bytes(),
		StartTimeUnixNanos: res.Entries[entries-resEntriesTSOffset].Value.Uint64(),
		DurationNanos:      res.Entries[entries-resEntriesDurationOffset].Value.Uint64(),
		ServiceName:        res.Entries[entries-resEntriesServiceNameOffset].Value.String(),
		FilePath:           res.Entries[entries-resEntriesPathOffset].Value.String(),
		LineNo:             res.Entries[entries-resEntriesLineNoOffset].Value.Uint32(),
	}, err
}

func (i *snapshotMetadataIterator) Close() {
	i.iter.Close()
}
