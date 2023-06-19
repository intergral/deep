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

package v1

import (
	"bytes"
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/modules/tracepoint/store/encoding/types"
	"github.com/intergral/deep/pkg/deepdb/backend"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
)

type TPEncoder struct {
	Store storage.Store
}

func (t *TPEncoder) Flush(ctx context.Context, block types.TPBlock) error {

	var positons []byte
	var tpBytes []byte

	tps := block.Tps()

	for _, tp := range tps {
		marshal, err := proto.Marshal(tp)
		if err != nil {
			return err
		}
		// record the length of each tp in bytes
		positons = append(positons, byte(len(marshal)))
		// append tps bytes to list of all tps
		tpBytes = append(tpBytes, marshal...)
	}

	// create data block
	// first 2 bytes are the length of the above arrays
	data := []byte{byte(len(positons)), byte(len(tpBytes))}
	// append the position array
	data = append(data, positons...)
	// append the tps
	data = append(data, tpBytes...)

	reader := bytes.NewReader(data)

	err := t.Store.WriteTracepointBlock(ctx, block.OrgId(), reader, int64(len(data)))
	if err != nil {
		return err
	}
	block.Flushed()

	return nil
}

func (t *TPEncoder) LoadBlock(ctx context.Context, orgId string) (types.TPBlock, error) {
	read, i, err := t.Store.ReadTracepointBlock(ctx, orgId)

	if err != nil {
		if err == backend.ErrDoesNotExist {
			// block doesn't exist so create new
			return &tpBlock{
				tps:   []*deeptp.TracePointConfig{},
				orgID: orgId,
			}, nil
		}
		return nil, err
	}

	data := make([]byte, i)
	_, err = read.Read(data)
	if err != nil {
		return nil, err
	}

	reader := bytes.NewReader(data)
	// read the first byte as the length of the position array
	positionSize, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	// read the second byte as the length of all the tps (we do not use this atm)
	_, err = reader.ReadByte()
	if err != nil {
		return nil, err
	}

	// the block is empty
	if positionSize == 0 {
		return &tpBlock{
			tps:   []*deeptp.TracePointConfig{},
			orgID: orgId,
		}, nil
	}

	// create slice for positions and read the bytes
	positionBytes := make([]byte, positionSize)
	_, err = reader.Read(positionBytes)
	if err != nil {
		return nil, err
	}

	tps := make([]*deeptp.TracePointConfig, len(positionBytes))
	var start = 0
	for i, position := range positionBytes {
		// create slice of the length from start to next position (which is then length of the next tp to read)
		tpBytes := make([]byte, int(position)-start)
		_, err := reader.Read(tpBytes)
		if err != nil {
			return nil, err
		}

		// unmarshal the tp to a grpc message
		var tp = &deeptp.TracePointConfig{}

		err = proto.Unmarshal(tpBytes, tp)
		if err != nil {
			return nil, err
		}

		tps[i] = tp
	}

	return &tpBlock{
		orgID: orgId,
		tps:   tps,
	}, nil
}
