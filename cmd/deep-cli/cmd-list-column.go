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
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
	pq "github.com/intergral/deep/pkg/parquetquery"
	"github.com/segmentio/parquet-go"
)

type listColumnCmd struct {
	backendOptions

	TenantID string `arg:"" help:"tenant-id within the bucket"`
	BlockID  string `arg:"" help:"block ID to list"`
	Column   string `arg:"ID" help:"column name to list values of"`
}

func (cmd *listColumnCmd) Run(ctx *globalOptions) error {
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

	colIndex, _ := pq.GetColumnIndexByPath(pf, cmd.Column)

	for i, rg := range pf.RowGroups() {

		// choose the column mentioned in the cli param
		cc := rg.ColumnChunks()[colIndex]

		fmt.Printf("\n***************       rowgroup %d      ********************\n\n\n", i)

		pages := cc.Pages()
		numPages := cc.ColumnIndex().NumPages()
		fmt.Println("Min Value of rowgroup", cc.ColumnIndex().MinValue(0).Bytes())
		fmt.Println("Max Value of rowgroup", cc.ColumnIndex().MaxValue(numPages-1).Bytes())

		buffer := make([]parquet.Value, 10000)
		for {
			pg, err := pages.ReadPage()
			if pg == nil || err == io.EOF {
				break
			}

			vr := pg.Values()
			for {
				x, err := vr.ReadValues(buffer)
				for y := 0; y < x; y++ {
					fmt.Println(buffer[y].Bytes())
				}

				// check for EOF after processing any returned data
				if err == io.EOF {
					break
				}
				// todo: better error handling
				if err != nil {
					break
				}
			}
		}
	}

	return nil
}
