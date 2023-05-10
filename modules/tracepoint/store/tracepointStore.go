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
	"github.com/intergral/deep/modules/tracepoint/store/encoding"
	"github.com/intergral/deep/modules/tracepoint/store/encoding/types"
	v1 "github.com/intergral/deep/modules/tracepoint/store/encoding/v1"
	"github.com/intergral/deep/pkg/deeppb"
	cp "github.com/intergral/deep/pkg/deeppb/common/v1"
	pb "github.com/intergral/deep/pkg/deeppb/poll/v1"
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
	"time"
)

type TPStore struct {
	orgStores map[string]*orgStore
	backend   types.TPBackend
}

type ResourceTPStore interface {
	ProcessRequest(req *deeppb.LoadTracepointRequest) (*deeppb.LoadTracepointResponse, error)
	AddTracepoint(tp *tp.TracePointConfig) error
	DeleteTracepoint(tpID string) error
}

type OrgTPStore interface {
	DeleteTracepoint(tpID string) error
	forResource(resource []*cp.KeyValue) (ResourceTPStore, error)
	AddTracepoint(tracepoint *tp.TracePointConfig) error
}

func NewStore(cfg storage.Config) (*TPStore, error) {
	loadEncoding, err := encoding.LoadBackend(cfg)
	if err != nil {
		return nil, err
	}
	return &TPStore{
		orgStores: map[string]*orgStore{},
		backend:   loadEncoding,
	}, nil
}

func (s *TPStore) Flush(ctx context.Context, store OrgTPStore) error {
	o := store.(*orgStore)
	o.mu.Lock()
	defer o.mu.Unlock()

	return s.backend.Flush(ctx, o.block)
}

func (s *TPStore) ForResource(ctx context.Context, id string, resource []*cp.KeyValue) (ResourceTPStore, error) {

	org, err := s.ForOrg(ctx, id)
	if err != nil {
		return nil, err
	}

	return org.forResource(resource)
}

func (s *TPStore) ForOrg(ctx context.Context, id string) (OrgTPStore, error) {

	if s.orgStores[id] != nil {
		return s.orgStores[id], nil
	}
	block, err := s.backend.LoadBlock(ctx, id)
	if err != nil {
		return nil, err
	}

	s.orgStores[id] = &orgStore{id: id, userStores: map[string]*resourceStore{}, block: block}

	return s.orgStores[id], nil
}

type orgStore struct {
	id         string
	userStores map[string]*resourceStore
	block      types.TPBlock
	mu         sync.Mutex
}

func (os *orgStore) AddTracepoint(tp *tp.TracePointConfig) error {
	os.mu.Lock()
	defer os.mu.Unlock()

	os.block.AddTracepoint(tp)

	for _, store := range os.userStores {
		if v1.ResourceMatches(tp, store.resource) {
			err := store.AddTracepoint(tp)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (os *orgStore) DeleteTracepoint(tpID string) error {
	os.mu.Lock()
	defer os.mu.Unlock()
	for _, store := range os.userStores {
		_ = store.DeleteTracepoint(tpID)
	}
	return nil
}

func (os *orgStore) forResource(resource []*cp.KeyValue) (ResourceTPStore, error) {
	key := os.keyForResource(os.id, resource)
	if os.userStores[key] != nil {
		return os.userStores[key], nil
	}
	tps, err := os.block.ForResource(resource)
	if err != nil {
		return nil, err
	}

	os.userStores[key] = &resourceStore{orgId: os.id, resource: resource, tps: tps, os: os}

	return os.userStores[key], nil
}

// keyForResource creates a key from the user/org id and the client resources. This identifies all resource with
// the same tags as the same thing. Allowing us to cache the values easier.
func (os *orgStore) keyForResource(id string, resource []*cp.KeyValue) string {
	h := fnv.New32()

	_, _ = h.Write([]byte(id))

	if resource != nil {
		keys := make([]string, len(resource))
		values := make(map[string]string, len(resource))
		for i, attr := range resource {
			keys[i] = attr.Key
			values[attr.Key] = attr.Value.GetStringValue()
		}

		// we need to sort the keys to ensure we always hash the same way
		sort.Strings(keys)

		for _, key := range keys {
			_, _ = h.Write([]byte(key))
			_, _ = h.Write([]byte(values[key]))
		}
	}

	return strconv.Itoa(int(h.Sum32()))
}

type resourceStore struct {
	orgId       string
	tps         []*tp.TracePointConfig
	currentHash string
	resource    []*cp.KeyValue
	os          *orgStore
}

func (us *resourceStore) ProcessRequest(req *deeppb.LoadTracepointRequest) (*deeppb.LoadTracepointResponse, error) {
	// we are now in the scalable nodes for the tracepoints. here we need to load from disk/mem
	var responseType = pb.ResponseType_UPDATE
	if req.Request.CurrentHash == us.currentHash {
		responseType = pb.ResponseType_NO_CHANGE
	}

	return &deeppb.LoadTracepointResponse{Response: &pb.PollResponse{
		TsNanos:      uint64(time.Now().UnixNano()),
		CurrentHash:  us.currentHash,
		Response:     us.tps,
		ResponseType: responseType,
	}}, nil
}

func (us *resourceStore) AddTracepoint(tp *tp.TracePointConfig) error {
	us.tps = append(us.tps, tp)
	us.rehash()
	return nil
}

func (us *resourceStore) DeleteTracepoint(tpID string) error {
	var tpToRemoveIndex int
	for i, config := range us.tps {
		if config.ID == tpID {
			tpToRemoveIndex = i
			break
		}
	}

	us.tps = us.remove(us.tps, tpToRemoveIndex)
	us.rehash()
	return nil
}

func (us *resourceStore) rehash() {
	h := fnv.New32()
	for _, config := range us.tps {
		_, _ = h.Write([]byte(config.ID))
	}
	us.currentHash = strconv.Itoa(int(h.Sum32()))
}

func (us *resourceStore) remove(s []*tp.TracePointConfig, i int) []*tp.TracePointConfig {
	s[i] = s[len(s)-1]
	s[len(s)-1] = nil
	return s[:len(s)-1]
}
