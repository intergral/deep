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
	"encoding/json"
	"github.com/google/uuid"
	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
	"github.com/olekukonko/tablewriter"
	"github.com/segmentio/parquet-go"
	"os"
	"strconv"
	"time"
)

type listObjectsCmd struct {
	backendOptions

	TenantID string `arg:"" help:"tenant-id within the bucket"`
	BlockID  string `arg:"" help:"block ID to list"`
}

func (cmd *listObjectsCmd) Run(ctx *globalOptions) error {
	r, _, _, err := loadBackend(&cmd.backendOptions, ctx)
	if err != nil {
		return err
	}

	meta, err := r.BlockMeta(context.TODO(), uuid.MustParse(cmd.BlockID), cmd.TenantID)
	if err != nil {
		return err
	}

	rr := vparquet.NewBackendReaderAt(context.Background(), r, vparquet.DataFileName, meta.BlockID, meta.TenantID)
	pf, err := parquet.OpenFile(rr, int64(meta.Size))
	if err != nil {
		return err
	}

	rows, err := parquet.Read[vparquet.Snapshot](pf, int64(meta.Size))

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"IDText", "Ts", "Path", "LineNo", "Resources", "Attributes"})

	for _, row := range rows {
		recourseAsJson, _ := json.Marshal(row.Resource)
		attributesAsJson, _ := json.Marshal(row.Attributes)
		unix := time.UnixMilli(row.Ts)
		table.Append([]string{row.IDText, unix.String(), row.Tracepoint.Path, strconv.Itoa(int(row.Tracepoint.LineNo)), string(recourseAsJson), string(attributesAsJson)})
	}

	table.Render()

	return nil
}
