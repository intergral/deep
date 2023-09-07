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

package common

import (
	"fmt"

	"github.com/intergral/deep/pkg/deepdb/backend"
)

// BlockConfig holds configuration options for newly created blocks
type BlockConfig struct {
	BloomFP             float64          `yaml:"bloom_filter_false_positive"`
	BloomShardSizeBytes int              `yaml:"bloom_filter_shard_size_bytes"`
	Version             string           `yaml:"version"`
	SearchEncoding      backend.Encoding `yaml:"search_encoding"`

	// v2 fields
	Encoding backend.Encoding `yaml:"v2_encoding"`

	// parquet fields
	RowGroupSizeBytes int `yaml:"parquet_row_group_size_bytes"`
}

// ValidateConfig returns true if the config is valid
func ValidateConfig(b *BlockConfig) error {
	if b.BloomFP <= 0.0 {
		return fmt.Errorf("invalid bloom filter fp rate %v", b.BloomFP)
	}

	if b.BloomShardSizeBytes <= 0 {
		return fmt.Errorf("positive value required for bloom-filter shard size")
	}

	return nil
}
