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
	"bytes"
	"github.com/intergral/deep/pkg/deepdb/encoding/vparquet"
	deep_io "github.com/intergral/deep/pkg/io"
	"github.com/intergral/deep/pkg/util"
	"github.com/segmentio/parquet-go"
	"github.com/willf/bloom"
	"os"
)

func main() {
	file := "/home/bdonnell/repo/github/intergral/deep/examples/docker-compose/debug/deep-data/blocks/single-tenant/a7e03070-9f3a-4461-9673-86e98fa8981d/data.parquet"
	//open, err := os.Open(file)

	//schema := parquet.SchemaOf(new(vparquet.Snapshot))
	//
	//reader := parquet.NewReader(open, schema)

	//
	rows, err := parquet.ReadFile[vparquet.Snapshot](file)
	if err != nil {
		print(err)
		return
	}
	snapId := "14c3aa7d185ed3719c79c78e8246681f"
	f, err := os.OpenFile("/home/bdonnell/repo/github/intergral/deep/examples/docker-compose/debug/deep-data/blocks/single-tenant/a7e03070-9f3a-4461-9673-86e98fa8981d/bloom-0", os.O_RDONLY, 0644)
	stat, err := f.Stat()

	estimate, err := deep_io.ReadAllWithEstimate(f, stat.Size())

	filter := &bloom.BloomFilter{}
	_, err = filter.ReadFrom(bytes.NewReader(estimate))
	id, err := util.HexStringToTraceID(snapId)
	println(snapId, id, err, filter.Test(id))

	noId := "dc1b672ac7394b7a53433107fc3f012"
	traceID, err := util.HexStringToTraceID(noId)
	println(noId, traceID, err, filter.Test(traceID))

	for _, c := range rows {
		println(c.ID, c.IDText, filter.Test(c.ID))
	}

	//f, err := parquet.OpenFile(open, 1010101010101010)
	//if err != nil {
	//	print(err)
	//	return
	//}
	//
	//for _, rowGroup := range f.RowGroups() {
	//	for _, c := range rowGroup.ColumnChunks() {
	//		fmt.Printf("%+v : %+v \n", c.Type(), c.Column())
	//	}
	//}
}
