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

package store

import (
	"context"
	"github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/pkg/deepdb"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/deepdb/wal"
	"github.com/intergral/deep/pkg/deeppb"
	deeppb_poll "github.com/intergral/deep/pkg/deeppb/poll/v1"
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/stretchr/testify/assert"
	"testing"
)

func createStore(t *testing.T, tempDir string) *TPStore {
	if tempDir == "" {
		tempDir = t.TempDir()
	}
	store, err := storage.NewStore(storage.Config{
		TracePoint: deepdb.Config{
			Backend: "local",
			Local: &local.Config{
				Path: tempDir,
			},
			Block: &common.BlockConfig{
				BloomFP:             0.01,
				BloomShardSizeBytes: 100_000,
				Version:             encoding.DefaultEncoding().Version(),
				Encoding:            backend.EncLZ4_1M,
			},
			WAL: &wal.Config{
				Filepath: tempDir,
			},
		},
	}, nil)
	assert.NoError(t, err, "cannot load store")

	newStore, err := NewStore(store)
	assert.NoError(t, err, "cannot load tp store")

	return newStore
}

func TestNewOrgIsHandled(t *testing.T) {

	tpStore := createStore(t, "")

	org, err := tpStore.ForOrg(context.Background(), "test-org")

	assert.NoError(t, err, "cannot load org store")

	assert.Equal(t, 1, len(tpStore.orgStores))

	resource, _ := org.forResource(nil)

	request, _ := resource.ProcessRequest(&deeppb.LoadTracepointRequest{
		Request: &deeppb_poll.PollRequest{},
	})

	assert.Equal(t, 0, len(request.Response.Response))
}

func TestCreatingDeleteTracepoint(t *testing.T) {
	tpStore := createStore(t, "")

	org, _ := tpStore.ForOrg(context.Background(), "test-org")

	err := org.AddTracepoint(&tp.TracePointConfig{
		ID:        "1",
		Targeting: nil,
	})

	assert.NoError(t, err, "cannot create tp")
	// we create a new resource each go to simulate requests
	{
		resource, _ := org.forResource(nil)
		response, _ := resource.ProcessRequest(&deeppb.LoadTracepointRequest{
			Request: &deeppb_poll.PollRequest{},
		})

		assert.Equal(t, "1", response.Response.Response[0].ID)
	}
	{
		resource, _ := org.forResource(nil)
		err = resource.DeleteTracepoint("1")
		assert.NoError(t, err, "can not delete tracepoint")
	}
	{
		resource, _ := org.forResource(nil)
		response, _ := resource.ProcessRequest(&deeppb.LoadTracepointRequest{
			Request: &deeppb_poll.PollRequest{},
		})

		assert.Equal(t, 0, len(response.Response.Response))
	}
}

func TestCanLoadAfterFlush(t *testing.T) {
	dir := t.TempDir()
	{
		tpStore := createStore(t, dir)

		org, _ := tpStore.ForOrg(context.Background(), "test-org")

		err := org.AddTracepoint(&tp.TracePointConfig{
			ID:        "1",
			Targeting: nil,
		})

		assert.NoError(t, err, "cannot create tp")

		err = tpStore.FlushAll(context.Background())
		assert.NoError(t, err, "failed to flush")
	}
	// no create a new store with the same dir (simulate restart)
	{
		tpStore := createStore(t, dir)
		org, _ := tpStore.ForOrg(context.Background(), "test-org")
		resource, _ := org.forResource(nil)
		response, _ := resource.ProcessRequest(&deeppb.LoadTracepointRequest{
			Request: &deeppb_poll.PollRequest{},
		})

		assert.Equal(t, "1", response.Response.Response[0].ID)
	}
}

func TestShouldGetCorrectTypeWhenProcessingRequests(t *testing.T) {
	tpStore := createStore(t, "")

	org, _ := tpStore.ForOrg(context.Background(), "test-org")

	_ = org.AddTracepoint(&tp.TracePointConfig{
		ID:        "1",
		Targeting: nil,
	})

	resource, _ := org.forResource(nil)

	response, _ := resource.ProcessRequest(&deeppb.LoadTracepointRequest{
		Request: &deeppb_poll.PollRequest{},
	})

	assert.Equal(t, response.Response.ResponseType, deeppb_poll.ResponseType_UPDATE)
	assert.NotEqual(t, "", response.Response.CurrentHash)
	resource2, _ := org.forResource(nil)

	response2, _ := resource2.ProcessRequest(&deeppb.LoadTracepointRequest{
		Request: &deeppb_poll.PollRequest{
			CurrentHash: response.Response.CurrentHash,
		},
	})

	assert.Equal(t, response2.Response.ResponseType, deeppb_poll.ResponseType_NO_CHANGE)
}

func TestGetNewTracepointsOnPoll(t *testing.T) {
	tpStore := createStore(t, "")

	{
		org, _ := tpStore.ForOrg(context.Background(), "test-org")

		_ = org.AddTracepoint(&tp.TracePointConfig{
			ID:        "1",
			Targeting: nil,
		})
	}
	var currentHash string
	{
		org, _ := tpStore.ForOrg(context.Background(), "test-org")
		resource, _ := org.forResource(nil)
		response, _ := resource.ProcessRequest(&deeppb.LoadTracepointRequest{
			Request: &deeppb_poll.PollRequest{
				CurrentHash: currentHash,
			},
		})
		currentHash = response.Response.CurrentHash
		assert.Equal(t, "1", response.Response.Response[0].ID)
		assert.Equal(t, response.Response.ResponseType, deeppb_poll.ResponseType_UPDATE)
	}
	{
		org, _ := tpStore.ForOrg(context.Background(), "test-org")
		resource, _ := org.forResource(nil)
		response, _ := resource.ProcessRequest(&deeppb.LoadTracepointRequest{
			Request: &deeppb_poll.PollRequest{
				CurrentHash: currentHash,
			},
		})
		assert.Equal(t, "1", response.Response.Response[0].ID)
		assert.Equal(t, response.Response.CurrentHash, currentHash)
		assert.Equal(t, response.Response.ResponseType, deeppb_poll.ResponseType_NO_CHANGE)
	}

	{
		org, _ := tpStore.ForOrg(context.Background(), "test-org")

		_ = org.AddTracepoint(&tp.TracePointConfig{
			ID:        "2",
			Targeting: nil,
		})
	}

	{
		org, _ := tpStore.ForOrg(context.Background(), "test-org")
		resource, _ := org.forResource(nil)
		response, _ := resource.ProcessRequest(&deeppb.LoadTracepointRequest{
			Request: &deeppb_poll.PollRequest{
				CurrentHash: currentHash,
			},
		})
		currentHash = response.Response.CurrentHash
		assert.Equal(t, 2, len(response.Response.Response))
		assert.Equal(t, response.Response.ResponseType, deeppb_poll.ResponseType_UPDATE)
	}

	{
		org, _ := tpStore.ForOrg(context.Background(), "test-org")
		resource, _ := org.forResource(nil)
		response, _ := resource.ProcessRequest(&deeppb.LoadTracepointRequest{
			Request: &deeppb_poll.PollRequest{
				CurrentHash: currentHash,
			},
		})
		assert.Equal(t, 2, len(response.Response.Response))
		assert.Equal(t, response.Response.CurrentHash, currentHash)
		assert.Equal(t, response.Response.ResponseType, deeppb_poll.ResponseType_NO_CHANGE)
	}

}
