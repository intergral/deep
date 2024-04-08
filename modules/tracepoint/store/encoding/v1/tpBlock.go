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
	"github.com/intergral/deep/modules/tracepoint/store/encoding/types"
	"time"

	cp "github.com/intergral/deep/pkg/deeppb/common/v1"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
)

type tpBlock struct {
	tps       []*deep_tp.TracePointConfig
	tenantID  string
	lastFlush int64
}

var _ types.TPBlock = (*tpBlock)(nil)

func (t *tpBlock) ForResource(resource []*cp.KeyValue) ([]*deep_tp.TracePointConfig, error) {
	var tps []*deep_tp.TracePointConfig

	for _, tp := range t.tps {
		if t.matches(tp, resource) {
			tps = append(tps, tp)
		}
	}

	return tps, nil
}

func (t *tpBlock) TenantID() string {
	return t.tenantID
}

func (t *tpBlock) Tps() []*deep_tp.TracePointConfig {
	return t.tps
}

func (t *tpBlock) Flushed() {
	t.lastFlush = time.Now().UnixMilli()
}

func (t *tpBlock) AddTracepoint(tp *deep_tp.TracePointConfig) {
	t.tps = append(t.tps, tp)
}

func (t *tpBlock) DeleteTracepoints(ids ...string) {
	var tpToRemoveIndex []int
	for i, config := range t.tps {
		for _, s := range ids {
			if config.ID == s {
				tpToRemoveIndex = append(tpToRemoveIndex, i)
				break
			}
		}
	}

	if len(tpToRemoveIndex) == 0 {
		// todo return error?
		return
	}

	t.tps = t.removeAll(t.tps, tpToRemoveIndex...)
}

func (t *tpBlock) matches(tp *deep_tp.TracePointConfig, resource []*cp.KeyValue) bool {
	return ResourceMatches(tp, resource)
}

func (t *tpBlock) remove(tps []*deep_tp.TracePointConfig, i int) []*deep_tp.TracePointConfig {
	tps[i] = tps[len(tps)-1]
	tps[len(tps)-1] = nil
	return tps[:len(tps)-1]
}

func (t *tpBlock) removeAll(tps []*deep_tp.TracePointConfig, idxs ...int) []*deep_tp.TracePointConfig {
	for _, i := range idxs {
		tps[i] = nil
	}
	return t.removeNils(tps)
}

func (t *tpBlock) removeNils(things []*deep_tp.TracePointConfig) []*deep_tp.TracePointConfig {
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

	return things[:len(things):len(things)]
}

func ResourceMatches(tp *deep_tp.TracePointConfig, resource []*cp.KeyValue) bool {
	// if the resource targeting is empty then we match all
	if len(resource) == 0 {
		return true
	}

	// the tp targeting is not set so match all
	if tp.Targeting == nil || len(tp.Targeting) == 0 {
		return true
	}

	targeting := make(map[string]string, len(tp.Targeting))

	for _, value := range resource {
		key := value.Key
		val := value.Value.GetStringValue()
		targeting[key] = val
	}

	for _, value := range tp.Targeting {
		key := value.Key
		val, ok := targeting[key]
		if !ok {
			return false
		}
		if val != value.Value.GetStringValue() {
			return false
		}
	}

	return true
}
