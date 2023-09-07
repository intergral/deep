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

package main

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/model"
	v1 "github.com/intergral/deep/pkg/model/v1"
	"github.com/intergral/deep/pkg/util/test"
)

type cmdGenerateBlock struct {
	backendOptions
	TenantID string `arg:"" help:"tenant-id within the bucket"`
}

func (l *cmdGenerateBlock) Run(ctx *globalOptions) error {
	r, w, _, err := loadBackend(&l.backendOptions, ctx)
	if err != nil {
		return err
	}

	snapshots := generateTestSnapshots()

	version, _ := encoding.FromVersion(vparquet.VersionString)

	block, err := version.CreateWALBlock(uuid.New(), l.TenantID, l.Bucket, backend.EncSnappy, model.CurrentEncoding, time.Duration(0))

	for _, snapshot := range snapshots {
		decoder := v1.NewSegmentDecoder()
		write, _ := decoder.PrepareForWrite(snapshot, uint32(snapshot.TsNanos/1e9))
		err = block.Append(snapshot.ID, write, uint32(snapshot.TsNanos/1e9))
	}
	err = block.Flush()

	iter, _ := block.Iterator()
	defer iter.Close()
	_, _ = version.CreateBlock(context.Background(), &common.BlockConfig{
		BloomShardSizeBytes: storage.DefaultBloomShardSizeBytes,
		BloomFP:             storage.DefaultBloomFP,
	}, block.BlockMeta(), iter, r, w)

	return nil
}

func generateTestSnapshots() []*tp.Snapshot {
	snapshots := GenerateTestSnapshots()

	for i := 0; i < 10; i++ {
		snapshot := test.GenerateSnapshot(i, &test.GenerateOptions{RandomStrings: true})
		snapshots = append(snapshots, snapshot)
	}

	return snapshots
}
