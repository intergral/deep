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

var _ deepql.SnapshotResultIterator = (*snapshotMetadataIterator)(nil)

func newSnapshotMetadataIterator(iter parquetquery.Iterator) *snapshotMetadataIterator {
	return &snapshotMetadataIterator{
		iter: iter,
	}
}

func (i *snapshotMetadataIterator) Next(ctx context.Context) (*deepql.SnapshotResult, error) {
	//res, err := i.iter.Next()
	//if err != nil {
	//	return nil, err
	//}
	//if res == nil {
	//	return nil, nil
	//}
	//
	//// The spanset is in the OtherEntries
	//iface := res.OtherValueFromKey(otherEntrySpansetKey)
	//if iface == nil {
	//	return nil, fmt.Errorf("engine assumption broken: spanset not found in other entries")
	//}
	//ss, ok := iface.(*deepql.SnapshotResult)
	//if !ok {
	//	return nil, fmt.Errorf("engine assumption broken: spanset is not of type *traceql.Spanset")
	//}
	//
	//return ss, nil
	return nil, nil
}

func (i *snapshotMetadataIterator) Close() {
	i.iter.Close()
}
