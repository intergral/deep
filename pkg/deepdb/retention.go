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

package deepdb

import (
	"context"
	"time"

	"github.com/opentracing/opentracing-go"

	"github.com/go-kit/log/level"

	"github.com/intergral/deep/pkg/boundedwaitgroup"
	"github.com/intergral/deep/pkg/deepdb/backend"
)

// retentionLoop watches a timer to clean up blocks that are past retention.
func (rw *readerWriter) retentionLoop(ctx context.Context) {
	ticker := time.NewTicker(rw.cfg.BlocklistPoll)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ticker.C:
			rw.doRetention(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (rw *readerWriter) doRetention(ctx context.Context) {
	tenants := rw.blocklist.Tenants()

	bg := boundedwaitgroup.New(rw.compactorCfg.RetentionConcurrency)

	for _, tenantID := range tenants {
		bg.Add(1)
		go func(t string) {
			defer bg.Done()
			rw.retainTenant(ctx, t)
		}(tenantID)
	}

	bg.Wait()
}

func (rw *readerWriter) retainTenant(ctx context.Context, tenantID string) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "rw.retainTenant")
	span.SetTag("tenantID", tenantID)
	defer span.Finish()

	start := time.Now()
	defer func() { metricRetentionDuration.Observe(time.Since(start).Seconds()) }()

	// Check for overrides
	retention := rw.compactorCfg.BlockRetention // Default
	if r := rw.compactorOverrides.BlockRetentionForTenant(tenantID); r != 0 {
		retention = r
	}
	level.Debug(rw.logger).Log("msg", "Performing block retention", "tenantID", tenantID, "retention", retention)

	// iterate through block list.  make compacted anything that is past retention.
	cutoff := time.Now().Add(-retention)
	blocklist := rw.blocklist.Metas(tenantID)
	for _, b := range blocklist {
		select {
		case <-ctx.Done():
			return
		default:
			if b.EndTime.Before(cutoff) && rw.compactorSharder.Owns(b.BlockID.String()) {
				level.Info(rw.logger).Log("msg", "marking block for deletion", "blockID", b.BlockID, "tenantID", tenantID)
				err := rw.c.MarkBlockCompacted(b.BlockID, tenantID)
				if err != nil {
					level.Error(rw.logger).Log("msg", "failed to mark block compacted during retention", "blockID", b.BlockID, "tenantID", tenantID, "err", err)
					metricRetentionErrors.Inc()
				} else {
					metricMarkedForDeletion.Inc()

					rw.blocklist.Update(tenantID, nil, []*backend.BlockMeta{b}, []*backend.CompactedBlockMeta{
						{
							BlockMeta:     *b,
							CompactedTime: time.Now(),
						},
					}, nil)
				}
			}
		}
	}

	// iterate through compacted list looking for blocks ready to be cleared
	cutoff = time.Now().Add(-rw.compactorCfg.CompactedBlockRetention)
	compactedBlocklist := rw.blocklist.CompactedMetas(tenantID)
	for _, b := range compactedBlocklist {
		select {
		case <-ctx.Done():
			return
		default:
			level.Info(rw.logger).Log("owns", rw.compactorSharder.Owns(b.BlockID.String()), "blockID", b.BlockID, "tenantID", tenantID, "cutoff", cutoff, "compacted", b.CompactedTime, "iscut", b.CompactedTime.Before(cutoff))
			if b.CompactedTime.Before(cutoff) && rw.compactorSharder.Owns(b.BlockID.String()) {
				level.Info(rw.logger).Log("msg", "deleting block", "blockID", b.BlockID, "tenantID", tenantID)
				err := rw.c.ClearBlock(b.BlockID, tenantID)
				if err != nil {
					level.Error(rw.logger).Log("msg", "failed to clear compacted block during retention", "blockID", b.BlockID, "tenantID", tenantID, "err", err)
					metricRetentionErrors.Inc()
				} else {
					metricDeleted.Inc()

					rw.blocklist.Update(tenantID, nil, nil, nil, []*backend.CompactedBlockMeta{b})
				}
			}
		}
	}
}
