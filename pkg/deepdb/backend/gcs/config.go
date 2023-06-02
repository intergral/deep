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
	"time"
)

type Config struct {
	BucketName         string            `yaml:"bucket_name"`
	ChunkBufferSize    int               `yaml:"chunk_buffer_size"`
	Endpoint           string            `yaml:"endpoint"`
	HedgeRequestsAt    time.Duration     `yaml:"hedge_requests_at"`
	HedgeRequestsUpTo  int               `yaml:"hedge_requests_up_to"`
	Insecure           bool              `yaml:"insecure"`
	ObjectCacheControl string            `yaml:"object_cache_control"`
	ObjectMetadata     map[string]string `yaml:"object_metadata"`
}
