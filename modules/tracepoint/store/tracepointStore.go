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
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/intergral/deep/pkg/deepql"

	"github.com/intergral/deep/modules/storage"
	"github.com/intergral/deep/modules/tracepoint/store/encoding"
	"github.com/intergral/deep/modules/tracepoint/store/encoding/types"
	v1 "github.com/intergral/deep/modules/tracepoint/store/encoding/v1"
	"github.com/intergral/deep/pkg/deeppb"
	cp "github.com/intergral/deep/pkg/deeppb/common/v1"
	pb "github.com/intergral/deep/pkg/deeppb/poll/v1"
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
)

type TPStore struct {
	orgStores map[string]*orgStore
	backend   types.TPBackend
}

type ResourceTPStore interface {
	ProcessRequest(req *deeppb.LoadTracepointRequest) (*deeppb.LoadTracepointResponse, error)
	AddTracepoint(tp *tp.TracePointConfig) error
	DeleteTracepoints(ids ...string) error
}

type OrgTPStore interface {
	DeleteTracepoints(ids ...string) error
	forResource(resource []*cp.KeyValue) (ResourceTPStore, error)
	AddTracepoint(tracepoint *tp.TracePointConfig) error
	FindTracepoints(request *deepql.CommandRequest) ([]*tp.TracePointConfig, error)
	LoadAll() ([]*tp.TracePointConfig, error)
}

// NewStore will create a new store to handle reading and writing to disk
func NewStore(store storage.Store) (*TPStore, error) {
	loadEncoding, err := encoding.LoadBackend(store)
	if err != nil {
		return nil, err
	}
	return &TPStore{
		orgStores: map[string]*orgStore{},
		backend:   loadEncoding,
	}, nil
}

func (s *TPStore) FlushAll(ctx context.Context) error {
	for _, store := range s.orgStores {
		err := s.Flush(ctx, store)
		if err != nil {
			return err
		}
	}
	return nil
}

// Flush will sync the in memory changes to disk
func (s *TPStore) Flush(ctx context.Context, store OrgTPStore) error {
	o := store.(*orgStore)
	o.mu.Lock()
	defer o.mu.Unlock()

	return s.backend.Flush(ctx, o.block)
}

// ForResource will find or create a new in memory store for the defined resource
// these stores are partitioned by org id
func (s *TPStore) ForResource(ctx context.Context, id string, resource []*cp.KeyValue) (ResourceTPStore, error) {
	org, err := s.ForOrg(ctx, id)
	if err != nil {
		return nil, err
	}

	return org.forResource(resource)
}

// ForOrg will find or create a in memory store for the given org id
// this will load the org block from storage, if we do not already have a copy
func (s *TPStore) ForOrg(ctx context.Context, id string) (OrgTPStore, error) {
	if s.orgStores[id] != nil {
		return s.orgStores[id], nil
	}
	block, err := s.backend.LoadBlock(ctx, id)
	if err != nil {
		return nil, err
	}

	s.orgStores[id] = &orgStore{tenantID: id, userStores: map[string]*resourceStore{}, block: block}

	return s.orgStores[id], nil
}

// orgStore is the link to the block in storage
// this is what is read and written to storage when needed
type orgStore struct {
	tenantID   string
	userStores map[string]*resourceStore
	block      types.TPBlock
	mu         sync.Mutex
}

var _ OrgTPStore = (*orgStore)(nil)

// AddTracepoint will add a tracepoint to the org and any matching resource stores
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

// DeleteTracepoints will remove a tracepoint from the org and any matching resource stores
func (os *orgStore) DeleteTracepoints(ids ...string) error {
	os.mu.Lock()
	defer os.mu.Unlock()
	for _, store := range os.userStores {
		err := store.DeleteTracepoints(ids...)
		if err != nil {
			return err
		}
	}
	os.block.DeleteTracepoints(ids...)
	return nil
}

// forResource will create a representation of the tracepoints based on the resource.
// this simple creates a sublist of the org tracepoints that have targeting that affect ths resource provided
// the resourceStore is not persisted to disk
func (os *orgStore) forResource(resource []*cp.KeyValue) (ResourceTPStore, error) {
	key := os.keyForResource(os.tenantID, resource)
	if os.userStores[key] != nil {
		return os.userStores[key], nil
	}
	tps, err := os.block.ForResource(resource)
	if err != nil {
		return nil, err
	}

	os.userStores[key] = &resourceStore{tenantID: os.tenantID, resource: resource, tps: tps, os: os}
	os.userStores[key].rehash()

	return os.userStores[key], nil
}

// keyForResource creates a key from the tenantID and the client resources. This identifies all resource with
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

func (os *orgStore) FindTracepoints(request *deepql.CommandRequest) ([]*tp.TracePointConfig, error) {
	os.mu.Lock()
	defer os.mu.Unlock()

	all := os.block.Tps()

	var found []*tp.TracePointConfig

	for _, config := range all {
		if os.matched(config, request) {
			found = append(found, config)
		}
	}

	return found, nil
}

func (os *orgStore) LoadAll() ([]*tp.TracePointConfig, error) {
	os.mu.Lock()
	defer os.mu.Unlock()
	return os.block.Tps(), nil
}

func (os *orgStore) matched(config *tp.TracePointConfig, request *deepql.CommandRequest) bool {
	for _, condition := range request.Conditions {
		switch condition.Attribute {
		case "path":
			if !condition.MatchesString(config.Path) {
				return false
			}
		case "method":
			if !condition.MatchesString(config.Args["method"]) {
				return false
			}
		case "id":
			if !condition.MatchesString(config.ID) {
				return false
			}
		case "line":
			if !condition.MatchesInt(int(config.LineNumber)) {
				return false
			}
		}
	}
	return true
}

// resourceStore is the in memory filtered list of the resource config
// e.g. this is the list of tracepoints that will affect a give client
// these are updated when clients connect, or tracepoint configs change
// they are not always kept in memory and will be recreated from storage
// when needed
type resourceStore struct {
	tenantID    string
	tps         []*tp.TracePointConfig
	currentHash string
	resource    []*cp.KeyValue
	os          *orgStore
}

var _ ResourceTPStore = (*resourceStore)(nil)

// ProcessRequest will process a request to load the tracepoints for a resource
func (us *resourceStore) ProcessRequest(req *deeppb.LoadTracepointRequest) (*deeppb.LoadTracepointResponse, error) {
	responseType := pb.ResponseType_UPDATE
	// if the incoming hash is the same has the hash we have then there is no change between the client and us
	if req.Request.CurrentHash != "" && req.Request.CurrentHash == us.currentHash {
		responseType = pb.ResponseType_NO_CHANGE
	}

	return &deeppb.LoadTracepointResponse{Response: &pb.PollResponse{
		TsNanos:      uint64(time.Now().UnixNano()),
		CurrentHash:  us.currentHash,
		Response:     us.tps,
		ResponseType: responseType,
	}}, nil
}

// AddTracepoint to this resource
func (us *resourceStore) AddTracepoint(tp *tp.TracePointConfig) error {
	us.tps = append(us.tps, tp)
	us.rehash()
	return nil
}

// DeleteTracepoints from this resource
func (us *resourceStore) DeleteTracepoints(ids ...string) error {
	var tpToRemoveIndex []int
	for i, config := range us.tps {
		for _, id := range ids {
			if config.ID == id {
				tpToRemoveIndex = append(tpToRemoveIndex, i)
			}
		}
	}

	if len(tpToRemoveIndex) == 0 {
		return nil
	}

	us.tps = us.removeAll(us.tps, tpToRemoveIndex...)
	us.rehash()
	return nil
}

// rehash the resource and set our currentHash
func (us *resourceStore) rehash() {
	h := fnv.New32()
	for _, config := range us.tps {
		_, _ = h.Write([]byte(config.ID))
	}
	us.currentHash = strconv.Itoa(int(h.Sum32()))
}

// remove element at index from array then return the new array
func (us *resourceStore) remove(array []*tp.TracePointConfig, index int) []*tp.TracePointConfig {
	array[index] = array[len(array)-1]
	array[len(array)-1] = nil
	return array[:len(array)-1]
}

func (us *resourceStore) removeAll(tps []*tp.TracePointConfig, idxs ...int) []*tp.TracePointConfig {
	for _, i := range idxs {
		tps[i] = nil
	}
	return us.removeNils(tps)
}

func (us *resourceStore) removeNils(things []*tp.TracePointConfig) []*tp.TracePointConfig {
	for i := 0; i < len(things); {
		if things[i] != nil {
			i++
			continue
		}

		if i < len(things)-1 {
			copy(things[i:], things[i+1:])
		}

		things[len(things)-1] = nil
		things = things[:len(things)-1]
	}

	things = things[:len(things):len(things)]
	return things
}
