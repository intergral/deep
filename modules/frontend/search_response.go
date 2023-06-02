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

package frontend

import (
	"context"
	"github.com/intergral/deep/pkg/deeppb"
	"net/http"
	"sort"
	"sync"

	"github.com/intergral/deep/pkg/search"
)

// searchResponse is a thread safe struct used to aggregate the responses from all downstream
// queriers
type searchResponse struct {
	err        error
	statusCode int
	statusMsg  string
	ctx        context.Context

	resultsMap       map[string]*deeppb.SnapshotSearchMetadata
	resultsMetrics   *deeppb.SearchMetrics
	cancelFunc       context.CancelFunc
	finishedRequests int

	limit int
	mtx   sync.Mutex
}

func newSearchResponse(ctx context.Context, limit int, cancelFunc context.CancelFunc) *searchResponse {
	return &searchResponse{
		ctx:              ctx,
		statusCode:       http.StatusOK,
		limit:            limit,
		cancelFunc:       cancelFunc,
		resultsMetrics:   &deeppb.SearchMetrics{},
		finishedRequests: 0,
		resultsMap:       map[string]*deeppb.SnapshotSearchMetadata{},
	}
}

func (r *searchResponse) setStatus(statusCode int, statusMsg string) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.statusCode = statusCode
	r.statusMsg = statusMsg

	if r.internalShouldQuit() {
		// cancel currently running requests, and bail
		r.cancelFunc()
	}
}

func (r *searchResponse) setError(err error) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.err = err

	if r.internalShouldQuit() {
		// cancel currently running requests, and bail
		r.cancelFunc()
	}
}

func (r *searchResponse) addResponse(res *deeppb.SearchResponse) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	for _, t := range res.Snapshots {
		if _, ok := r.resultsMap[t.SnapshotID]; !ok {
			r.resultsMap[t.SnapshotID] = t
		} else {
			search.CombineSearchResults(r.resultsMap[t.SnapshotID], t)
		}
	}

	// purposefully ignoring InspectedBlocks as that value is set by the sharder
	r.resultsMetrics.InspectedBytes += res.Metrics.InspectedBytes
	r.resultsMetrics.InspectedTraces += res.Metrics.InspectedTraces
	r.resultsMetrics.SkippedBlocks += res.Metrics.SkippedBlocks
	r.resultsMetrics.SkippedTraces += res.Metrics.SkippedTraces

	// count this request as finished
	r.finishedRequests++

	if r.internalShouldQuit() {
		// cancel currently running requests, and bail
		r.cancelFunc()
	}
}

// shouldQuit locks and checks if we should quit from current execution or not
func (r *searchResponse) shouldQuit() bool {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	quit := r.internalShouldQuit()
	if quit {
		// cancel currently running requests, and bail
		r.cancelFunc()
	}

	return quit
}

// internalShouldQuit check if we should quit but without locking,
// NOTE: only use internally where we already hold lock on searchResponse
func (r *searchResponse) internalShouldQuit() bool {
	if r.err != nil {
		return true
	}
	if r.ctx.Err() != nil {
		return true
	}
	if r.statusCode/100 != 2 {
		return true
	}
	if len(r.resultsMap) > r.limit {
		return true
	}

	return false
}

func (r *searchResponse) result() *deeppb.SearchResponse {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	res := &deeppb.SearchResponse{
		Metrics: r.resultsMetrics,
	}

	for _, t := range r.resultsMap {
		res.Snapshots = append(res.Snapshots, t)
	}
	sort.Slice(res.Snapshots, func(i, j int) bool {
		return res.Snapshots[i].StartTimeUnixNano > res.Snapshots[j].StartTimeUnixNano
	})

	return res
}
