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

package encoding

import (
	"fmt"
	"github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/modules/tracepoint/store/encoding/types"
	v1 "github.com/intergral/deep/modules/tracepoint/store/encoding/v1"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/azure"
	"github.com/intergral/deep/pkg/deepdb/backend/gcs"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/backend/s3"
)

func LoadBackend(cfg storage.Config) (types.TPBackend, error) {

	var err error
	var rawR backend.RawReader
	var rawW backend.RawWriter

	switch cfg.Tracepoint.Backend {
	case "local":
		rawR, rawW, _, err = local.New(cfg.Tracepoint.Local)
	case "gcs":
		rawR, rawW, _, err = gcs.New(cfg.Tracepoint.GCS)
	case "s3":
		rawR, rawW, _, err = s3.New(cfg.Tracepoint.S3)
	case "azure":
		rawR, rawW, _, err = azure.New(cfg.Tracepoint.Azure)
	default:
		err = fmt.Errorf("unknown backend %s", cfg.Tracepoint.Backend)
	}

	if err != nil {
		return nil, err
	}

	return &v1.TPEncoder{
		Reader: rawR,
		Writer: rawW,
	}, nil
}
