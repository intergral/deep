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

package local

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/intergral/deep/pkg/io"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/intergral/deep/pkg/deepdb/backend"
)

const objectName = "test"

func TestReadWrite(t *testing.T) {
	tempFile, err := os.CreateTemp("/tmp", "")
	defer func(name string) {
		_ = os.Remove(name)
	}(tempFile.Name())
	assert.NoError(t, err, "unexpected error creating temp file")

	r, w, _, err := New(&Config{
		Path: t.TempDir(),
	})
	assert.NoError(t, err, "unexpected error creating local backend")

	blockID := uuid.New()
	tenantIDs := []string{"fake"}

	for i := 0; i < 10; i++ {
		tenantIDs = append(tenantIDs, fmt.Sprintf("%d", rand.Int()))
	}

	fakeMeta := &backend.BlockMeta{
		BlockID: blockID,
	}

	fakeObject := make([]byte, 20)

	_, err = crand.Read(fakeObject)
	assert.NoError(t, err, "unexpected error creating fakeObject")

	ctx := context.Background()
	for _, id := range tenantIDs {
		fakeMeta.TenantID = id
		err = w.Write(ctx, objectName, backend.KeyPathForBlock(fakeMeta.BlockID, id), bytes.NewReader(fakeObject), int64(len(fakeObject)), false)
		assert.NoError(t, err, "unexpected error writing")
	}

	actualObject, size, err := r.Read(ctx, objectName, backend.KeyPathForBlock(blockID, tenantIDs[0]), false)
	assert.NoError(t, err, "unexpected error reading")
	actualObjectBytes, err := io.ReadAllWithEstimate(actualObject, size)
	assert.NoError(t, err, "unexpected error reading")
	assert.Equal(t, fakeObject, actualObjectBytes)

	actualReadRange := make([]byte, 5)
	err = r.ReadRange(ctx, objectName, backend.KeyPathForBlock(blockID, tenantIDs[0]), 5, actualReadRange, false)
	assert.NoError(t, err, "unexpected error range")
	assert.Equal(t, fakeObject[5:10], actualReadRange)

	list, err := r.List(ctx, backend.KeyPath{tenantIDs[0]})
	assert.NoError(t, err, "unexpected error reading blocklist")
	assert.Len(t, list, 1)
	assert.Equal(t, blockID.String(), list[0])
}

func TestShutdownLeavesTenantsWithBlocks(t *testing.T) {
	r, w, _, err := New(&Config{
		Path: t.TempDir(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	blockID := uuid.New()
	contents := bytes.NewReader([]byte("test"))
	tenant := "fake"

	// write a "block"
	err = w.Write(ctx, "test", backend.KeyPathForBlock(blockID, tenant), contents, contents.Size(), false)
	require.NoError(t, err)

	tenantExists(t, tenant, r)
	blockExists(t, blockID, tenant, r)

	// shutdown the backend
	r.Shutdown()

	tenantExists(t, tenant, r)
	blockExists(t, blockID, tenant, r)
}

func TestShutdownRemovesTenantsWithoutBlocks(t *testing.T) {
	r, w, c, err := New(&Config{
		Path: t.TempDir(),
	})
	require.NoError(t, err)

	ctx := context.Background()
	blockID := uuid.New()
	contents := bytes.NewReader([]byte("test"))
	tenant := "tenant"

	// write a "block"
	err = w.Write(ctx, "test", backend.KeyPathForBlock(blockID, tenant), contents, contents.Size(), false)
	require.NoError(t, err)

	tenantExists(t, tenant, r)
	blockExists(t, blockID, tenant, r)

	// clear the block
	err = c.ClearBlock(blockID, tenant)
	require.NoError(t, err)

	tenantExists(t, tenant, r)

	// block should not exist
	blocks, err := r.List(ctx, backend.KeyPath{tenant})
	require.NoError(t, err)
	require.Len(t, blocks, 0)

	// shutdown the backend
	r.Shutdown()

	// tenant should not exist
	tenants, err := r.List(ctx, backend.KeyPath{})
	require.NoError(t, err)
	require.Len(t, tenants, 0)
}

func tenantExists(t *testing.T, tenant string, r backend.RawReader) {
	tenants, err := r.List(context.Background(), backend.KeyPath{})
	require.NoError(t, err)
	require.Len(t, tenants, 1)
	require.Equal(t, tenant, tenants[0])
}

func blockExists(t *testing.T, blockID uuid.UUID, tenant string, r backend.RawReader) {
	blocks, err := r.List(context.Background(), backend.KeyPath{tenant})
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, blockID.String(), blocks[0])
}
