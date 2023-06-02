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

package gcs

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/googleapis/gax-go/v2"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"google.golang.org/api/iterator"
)

func (rw *readerWriter) MarkBlockCompacted(blockID uuid.UUID, tenantID string) error {
	// move meta file to a new location
	metaFilename := backend.MetaFileName(blockID, tenantID)
	compactedMetaFilename := backend.CompactedMetaFileName(blockID, tenantID)

	src := rw.bucket.Object(metaFilename)
	dst := rw.bucket.Object(compactedMetaFilename).Retryer(
		storage.WithBackoff(gax.Backoff{}),
		storage.WithPolicy(storage.RetryAlways),
	)

	ctx := context.TODO()
	_, err := dst.CopierFrom(src).Run(ctx)
	if err != nil {
		return err
	}

	return src.Delete(ctx)
}

func (rw *readerWriter) ClearBlock(blockID uuid.UUID, tenantID string) error {
	if len(tenantID) == 0 {
		return fmt.Errorf("empty tenant id")
	}

	if blockID == uuid.Nil {
		return fmt.Errorf("empty block id")
	}

	ctx := context.TODO()
	iter := rw.bucket.Objects(ctx, &storage.Query{
		Prefix:   backend.RootPath(blockID, tenantID),
		Versions: false,
	})

	for {
		attrs, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		o := rw.bucket.Object(attrs.Name)
		err = o.Delete(ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (rw *readerWriter) CompactedBlockMeta(blockID uuid.UUID, tenantID string) (*backend.CompactedBlockMeta, error) {
	name := backend.CompactedMetaFileName(blockID, tenantID)

	bytes, modTime, err := rw.readAllWithModTime(context.Background(), name)
	if err != nil {
		return nil, readError(err)
	}

	out := &backend.CompactedBlockMeta{}
	err = json.Unmarshal(bytes, out)
	if err != nil {
		return nil, err
	}
	out.CompactedTime = modTime

	return out, nil
}
