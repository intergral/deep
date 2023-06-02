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

package s3

import (
	"time"

	"github.com/grafana/dskit/flagext"
)

type Config struct {
	Bucket             string         `yaml:"bucket"`
	Endpoint           string         `yaml:"endpoint"`
	Region             string         `yaml:"region"`
	AccessKey          string         `yaml:"access_key"`
	SecretKey          flagext.Secret `yaml:"secret_key"`
	SessionToken       flagext.Secret `yaml:"session_token"`
	Insecure           bool           `yaml:"insecure"`
	InsecureSkipVerify bool           `yaml:"insecure_skip_verify"`
	PartSize           uint64         `yaml:"part_size"`
	HedgeRequestsAt    time.Duration  `yaml:"hedge_requests_at"`
	HedgeRequestsUpTo  int            `yaml:"hedge_requests_up_to"`
	// SignatureV2 configures the object storage to use V2 signing instead of V4
	SignatureV2      bool              `yaml:"signature_v2"`
	ForcePathStyle   bool              `yaml:"forcepathstyle"`
	BucketLookupType int               `yaml:"bucket_lookup_type"`
	Tags             map[string]string `yaml:"tags"`
	StorageClass     string            `yaml:"storage_class"`
	Metadata         map[string]string `yaml:"metadata"`
}
