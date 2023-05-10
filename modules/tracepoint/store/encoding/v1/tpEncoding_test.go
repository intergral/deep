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
	"context"
	"fmt"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	deeptp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestFlush(t *testing.T) {
	fakeTracesFile, err := os.CreateTemp("/tmp", "")
	defer os.Remove(fakeTracesFile.Name())
	assert.NoError(t, err, "unexpected error creating temp file")

	r, w, _, err := local.New(&local.Config{
		Path: t.TempDir(),
	})

	encoder := TPEncoder{
		Reader: r,
		Writer: w,
	}

	block := tpBlock{
		orgID: "test-id",
		tps: []*deeptp.TracePointConfig{
			{
				ID:         "iamatest",
				Path:       "/some/test/file.path",
				LineNumber: 10,
			},
		},
	}

	err = encoder.Flush(context.Background(), &block)
	if err != nil {
		fmt.Printf("%v", err)
		t.Fail()
	}

	blockOut, err := encoder.LoadBlock(context.Background(), "test-id")
	assert.Equal(t, blockOut.OrgId(), "test-id")
	assert.Equal(t, len(blockOut.Tps()), 1)

	assert.Equal(t, blockOut.Tps()[0].ID, "iamatest")
	assert.Equal(t, blockOut.Tps()[0].Path, "/some/test/file.path")
	assert.Equal(t, blockOut.Tps()[0].LineNumber, uint32(10))
}
