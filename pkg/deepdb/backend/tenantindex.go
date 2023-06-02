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

package backend

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/klauspost/compress/gzip"
)

const (
	internalFilename = "index.json"
)

// TenantIndex holds a list of all metas and compacted metas for a given tenant
// it is probably stored in /<tenantid>/blockindex.json.gz as a gzipped json file
type TenantIndex struct {
	CreatedAt     time.Time             `json:"created_at"`
	Meta          []*BlockMeta          `json:"meta"`
	CompactedMeta []*CompactedBlockMeta `json:"compacted"`
}

func newTenantIndex(meta []*BlockMeta, compactedMeta []*CompactedBlockMeta) *TenantIndex {
	return &TenantIndex{
		CreatedAt:     time.Now(),
		Meta:          meta,
		CompactedMeta: compactedMeta,
	}
}

// marshal converts to json and compresses the bucketindex
func (b *TenantIndex) marshal() ([]byte, error) {
	buffer := &bytes.Buffer{}

	gzip := gzip.NewWriter(buffer)
	gzip.Name = internalFilename

	jsonBytes, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}

	if _, err = gzip.Write(jsonBytes); err != nil {
		return nil, err
	}
	if err = gzip.Flush(); err != nil {
		return nil, err
	}
	if err = gzip.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// unmarshal decompresses and unmarshals the results from json
func (b *TenantIndex) unmarshal(buffer []byte) error {
	gzipReader, err := gzip.NewReader(bytes.NewReader(buffer))
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	d := json.NewDecoder(gzipReader)
	return d.Decode(b)
}
