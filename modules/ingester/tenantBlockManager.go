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

package ingester

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/fnv"
	"sort"
	"sync"
	"time"

	"github.com/intergral/deep/pkg/deeppb"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"google.golang.org/grpc/status"

	"github.com/go-kit/log/level"
	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/atomic"
	"google.golang.org/grpc/codes"

	"github.com/intergral/deep/modules/overrides"
	"github.com/intergral/deep/pkg/deepdb"
	"github.com/intergral/deep/pkg/deepdb/backend"
	"github.com/intergral/deep/pkg/deepdb/backend/local"
	"github.com/intergral/deep/pkg/deepdb/encoding"
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/model"
	"github.com/intergral/deep/pkg/util/log"
	"github.com/intergral/deep/pkg/validation"
)

type snapshotTooLargeError struct {
	snapshotID        common.ID
	instanceID        string
	maxBytes, reqSize int
}

func newSnapshotTooLargeError(snapshotID common.ID, instanceID string, maxBytes, reqSize int) *snapshotTooLargeError {
	return &snapshotTooLargeError{
		snapshotID: snapshotID,
		instanceID: instanceID,
		maxBytes:   maxBytes,
		reqSize:    reqSize,
	}
}

func (e snapshotTooLargeError) Error() string {
	return fmt.Sprintf(
		"%s max size of snapshot (%d) exceeded while adding %d bytes to snapshot %s for tenant %s",
		overrides.ErrorPrefixSnapshotTooLarge, e.maxBytes, e.reqSize, hex.EncodeToString(e.snapshotID), e.instanceID)
}

const (
	snapshotDataType = "snapshot"
)

var (
	metricSnapshotsCreatedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "snapshots_created_total",
		Help:      "The total number of snapshots created per tenant.",
	}, []string{"tenant"})
	metricLiveSnapshots = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "live_snapshots",
		Help:      "The current number of lives snapshots per tenant.",
	}, []string{"tenant"})
	metricBlocksClearedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "blocks_cleared_total",
		Help:      "The total number of blocks cleared.",
	})
	metricBytesReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "bytes_received_total",
		Help:      "The total bytes received per tenant.",
	}, []string{"tenant", "data_type"})
	metricReplayErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "deep",
		Subsystem: "ingester",
		Name:      "replay_errors_total",
		Help:      "The total number of replay errors received per tenant.",
	}, []string{"tenant"})
)

// tenantBlockManager brings together all the parts that are needed to handle data for a tenant. This will store
// the liveSnapshots, WALBlocks and backend blocks. There should be one of these per tenant that is being managed.
type tenantBlockManager struct {
	snapshotMtx   sync.Mutex
	liveSnapshots map[uint32]*liveSnapshot
	snapshotSizes map[uint32]uint32
	snapshotCount atomic.Int32

	blocksMtx        sync.RWMutex
	headBlock        common.WALBlock
	completingBlocks []common.WALBlock
	completeBlocks   []*localBlock

	lastBlockCut time.Time

	tenantID              string
	snapshotsCreatedTotal prometheus.Counter
	bytesReceivedTotal    *prometheus.CounterVec
	limiter               *Limiter
	writer                deepdb.Writer

	local       *local.Backend
	localReader backend.Reader
	localWriter backend.Writer

	hash hash.Hash32
}

// newTenantBlockManager create a new manager for the provided tenantID
func newTenantBlockManager(tenantID string, limiter *Limiter, writer deepdb.Writer, l *local.Backend) (*tenantBlockManager, error) {
	i := &tenantBlockManager{
		liveSnapshots: map[uint32]*liveSnapshot{},
		snapshotSizes: map[uint32]uint32{},

		tenantID:              tenantID,
		snapshotsCreatedTotal: metricSnapshotsCreatedTotal.WithLabelValues(tenantID),
		bytesReceivedTotal:    metricBytesReceivedTotal,
		limiter:               limiter,
		writer:                writer,

		local:       l,
		localReader: backend.NewReader(l),
		localWriter: backend.NewWriter(l),

		hash: fnv.New32(),
	}
	err := i.resetHeadBlock()
	if err != nil {
		return nil, err
	}
	return i, nil
}

// PushBytesRequest accepts a push request to write new data to this tenant
func (i *tenantBlockManager) PushBytesRequest(ctx context.Context, req *deeppb.PushBytesRequest) error {
	return i.PushBytes(ctx, req.ID, req.Snapshot)
}

// PushBytes is used to push an unmarshalled deeppb.Snapshot to the tenantBlockManager
func (i *tenantBlockManager) PushBytes(ctx context.Context, id []byte, snapshotBytes []byte) error {
	i.measureReceivedBytes(snapshotBytes)

	if !validation.ValidSnapshotID(id) {
		return status.Errorf(codes.InvalidArgument, "%s is not a valid snapshot id", hex.EncodeToString(id))
	}

	// check for max snapshots before grabbing the lock to better load shed
	err := i.limiter.AssertMaxSnapshotsPerTenant(i.tenantID, int(i.snapshotCount.Load()))
	if err != nil {
		return status.Errorf(codes.FailedPrecondition, "%s max live snapshots exceeded for tenant %s: %v", overrides.ErrorPrefixLiveSnapshotsExceeded, i.tenantID, err)
	}

	return i.push(ctx, id, snapshotBytes)
}

func (i *tenantBlockManager) push(ctx context.Context, id, snapshotBytes []byte) error {
	i.snapshotMtx.Lock()
	defer i.snapshotMtx.Unlock()

	tkn := i.tokenForSnapshotID(id)
	maxBytes := i.limiter.limits.MaxBytesPerSnapshot(i.tenantID)

	if maxBytes > 0 {
		prevSize := int(i.snapshotSizes[tkn])
		reqSize := len(snapshotBytes)
		if prevSize+reqSize > maxBytes {
			return status.Errorf(codes.FailedPrecondition, newSnapshotTooLargeError(id, i.tenantID, maxBytes, reqSize).Error())
		}
	}

	snapshot := i.getOrCreateSnapshot(id, tkn, maxBytes)

	err := snapshot.Push(ctx, i.tenantID, snapshotBytes)
	if err != nil {
		if e, ok := err.(*snapshotTooLargeError); ok {
			return status.Errorf(codes.FailedPrecondition, e.Error())
		}
		return err
	}

	if maxBytes > 0 {
		i.snapshotSizes[tkn] += uint32(len(snapshotBytes))
	}

	return nil
}

func (i *tenantBlockManager) measureReceivedBytes(snapshotBytes []byte) {
	// measure received bytes as sum of slice lengths
	// type byte is guaranteed to be 1 byte in size
	// ref: https://golang.org/ref/spec#Size_and_alignment_guarantees
	i.bytesReceivedTotal.WithLabelValues(i.tenantID, snapshotDataType).Add(float64(len(snapshotBytes)))
}

// CutSnapshots moves any complete snapshots from liveSnapshots into a flushed wal block
func (i *tenantBlockManager) CutSnapshots(cutoff time.Duration, immediate bool) error {
	snapshotsToCut := i.snapshotsToCut(cutoff, immediate)
	segmentDecoder := model.MustNewSegmentDecoder(model.CurrentEncoding)

	// Sort by ID
	sort.Slice(snapshotsToCut, func(i, j int) bool {
		return bytes.Compare(snapshotsToCut[i].snapshotId, snapshotsToCut[j].snapshotId) == -1
	})

	for _, t := range snapshotsToCut {

		out, err := segmentDecoder.ToObject(t.bytes)
		if err != nil {
			return err
		}

		err = i.writeSnapshotToHeadBlock(t.snapshotId, out, t.start)
		if err != nil {
			return err
		}
	}

	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()
	return i.headBlock.Flush()
}

// CutBlockIfReady cuts a completingBlock from the HeadBlock if ready.
// Returns the ID of a block if one was cut or a nil ID if one was not cut, along with the error (if any).
func (i *tenantBlockManager) CutBlockIfReady(maxBlockLifetime time.Duration, maxBlockBytes uint64, immediate bool) (uuid.UUID, error) {
	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()

	if i.headBlock == nil || i.headBlock.DataLength() == 0 {
		return uuid.Nil, nil
	}

	now := time.Now()
	if i.lastBlockCut.Add(maxBlockLifetime).Before(now) || i.headBlock.DataLength() >= maxBlockBytes || immediate {

		// Final flush
		err := i.headBlock.Flush()
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to flush head block: %w", err)
		}

		completingBlock := i.headBlock

		i.completingBlocks = append(i.completingBlocks, completingBlock)

		err = i.resetHeadBlock()
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to resetHeadBlock: %w", err)
		}

		return completingBlock.BlockMeta().BlockID, nil
	}

	return uuid.Nil, nil
}

// CompleteBlock moves a completingBlock to a completeBlock. The new completeBlock has the same ID.
func (i *tenantBlockManager) CompleteBlock(blockID uuid.UUID) error {
	i.blocksMtx.Lock()
	// find local WAL block
	var completingBlock common.WALBlock
	for _, iterBlock := range i.completingBlocks {
		if iterBlock.BlockMeta().BlockID == blockID {
			completingBlock = iterBlock
			break
		}
	}
	i.blocksMtx.Unlock()

	if completingBlock == nil {
		return fmt.Errorf("error finding completingBlock")
	}

	ctx := context.Background()

	// write WAL block to local disk as block '/path/to/WAL/blocks'
	backendBlock, err := i.writer.CompleteBlockWithBackend(ctx, completingBlock, i.localReader, i.localWriter)
	if err != nil {
		return errors.Wrap(err, "error completing wal block with local backend")
	}
	// create local block and append to completed blocks
	ingesterBlock := newLocalBlock(ctx, backendBlock, i.local)

	i.blocksMtx.Lock()
	i.completeBlocks = append(i.completeBlocks, ingesterBlock)
	i.blocksMtx.Unlock()

	return nil
}

// ClearCompletingBlock will find the WAL block with the blockID and delete it from disk
func (i *tenantBlockManager) ClearCompletingBlock(blockID uuid.UUID) error {
	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()

	var completingBlock common.WALBlock
	for j, iterBlock := range i.completingBlocks {
		if iterBlock.BlockMeta().BlockID == blockID {
			completingBlock = iterBlock
			i.completingBlocks = append(i.completingBlocks[:j], i.completingBlocks[j+1:]...)
			break
		}
	}

	if completingBlock != nil {
		return completingBlock.Clear()
	}

	return errors.New("Error finding wal completingBlock to clear")
}

// GetBlockToBeFlushed gets a list of blocks that can be flushed to the backend.
func (i *tenantBlockManager) GetBlockToBeFlushed(blockID uuid.UUID) *localBlock {
	i.blocksMtx.RLock()
	defer i.blocksMtx.RUnlock()

	for _, c := range i.completeBlocks {
		if c.BlockMeta().BlockID == blockID && c.FlushedTime().IsZero() {
			return c
		}
	}

	return nil
}

// ClearFlushedBlocks will delete the local backend blocks that have been flushed to storage
func (i *tenantBlockManager) ClearFlushedBlocks(completeBlockTimeout time.Duration) error {
	var err error

	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()

	for idx, b := range i.completeBlocks {
		flushedTime := b.FlushedTime()
		if flushedTime.IsZero() {
			continue
		}

		if flushedTime.Add(completeBlockTimeout).Before(time.Now()) {
			i.completeBlocks = append(i.completeBlocks[:idx], i.completeBlocks[idx+1:]...)

			err = i.local.ClearBlock(b.BlockMeta().BlockID, i.tenantID)
			if err == nil {
				metricBlocksClearedTotal.Inc()
			}
			break
		}
	}

	return err
}

func (i *tenantBlockManager) FindSnapshotByID(ctx context.Context, id []byte) (*deep_tp.Snapshot, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "tenantBlockManager.FindSnapshotByID")
	span.SetTag("tenantID", i.tenantID)
	defer span.Finish()

	var err error
	var foundSnapshot *deep_tp.Snapshot

	// live snapshots
	i.snapshotMtx.Lock()
	if liveSnapshot, ok := i.liveSnapshots[i.tokenForSnapshotID(id)]; ok {
		foundSnapshot, err = model.MustNewSegmentDecoder(model.CurrentEncoding).PrepareForRead(liveSnapshot.bytes)
		if err != nil {
			i.snapshotMtx.Unlock()
			return nil, fmt.Errorf("unable to unmarshal liveSnapshot: %w", err)
		}
	}
	i.snapshotMtx.Unlock()

	if foundSnapshot != nil {
		return foundSnapshot, nil
	}

	i.blocksMtx.RLock()
	defer i.blocksMtx.RUnlock()

	// headBlock
	tr, err := i.headBlock.FindSnapshotByID(ctx, id, common.DefaultSearchOptions())
	if err != nil {
		return nil, fmt.Errorf("headBlock.FindSnapshotByID failed: %w", err)
	}
	if tr != nil {
		return tr, nil
	}

	// completingBlock
	for _, c := range i.completingBlocks {
		tr, err = c.FindSnapshotByID(ctx, id, common.DefaultSearchOptions())
		if err != nil {
			return nil, fmt.Errorf("completingBlock.FindSnapshotByID failed: %w", err)
		}
		if tr != nil {
			return tr, nil
		}
	}

	// completeBlock
	for _, c := range i.completeBlocks {
		found, err := c.FindSnapshotByID(ctx, id, common.DefaultSearchOptions())
		if err != nil {
			return nil, fmt.Errorf("completeBlock.FindSnapshotByID failed: %w", err)
		}
		if found != nil {
			return found, nil
		}
	}
	return nil, nil
}

// AddCompletingBlock adds an AppendBlock directly to the slice of completing blocks.
// This is used during wal replay. It is expected that calling code will add the appropriate
// jobs to the queue to eventually flush these.
func (i *tenantBlockManager) AddCompletingBlock(b common.WALBlock) {
	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()

	i.completingBlocks = append(i.completingBlocks, b)
}

// getOrCreateSnapshot will return a new snapshot object for the given request
//
//	It must be called under the i.snapshotMtx lock
func (i *tenantBlockManager) getOrCreateSnapshot(snapshotID []byte, fp uint32, maxBytes int) *liveSnapshot {
	snapshot, ok := i.liveSnapshots[fp]
	if ok {
		return snapshot
	}

	snapshot = newLiveSnapshot(snapshotID, maxBytes)
	i.liveSnapshots[fp] = snapshot
	i.snapshotsCreatedTotal.Inc()
	i.snapshotCount.Inc()

	return snapshot
}

// tokenForSnapshotID hash snapshot ID, should be called under lock
func (i *tenantBlockManager) tokenForSnapshotID(id []byte) uint32 {
	i.hash.Reset()
	_, _ = i.hash.Write(id)
	return i.hash.Sum32()
}

// resetHeadBlock() should be called under lock
func (i *tenantBlockManager) resetHeadBlock() error {
	// Reset snapshot sizes when cutting block
	i.snapshotMtx.Lock()
	i.snapshotSizes = make(map[uint32]uint32, len(i.snapshotSizes))
	i.snapshotMtx.Unlock()

	newHeadBlock, err := i.writer.WAL().NewBlock(uuid.New(), i.tenantID, model.CurrentEncoding)
	if err != nil {
		return err
	}

	i.headBlock = newHeadBlock
	i.lastBlockCut = time.Now()

	return nil
}

func (i *tenantBlockManager) snapshotsToCut(cutoff time.Duration, immediate bool) []*liveSnapshot {
	i.snapshotMtx.Lock()
	defer i.snapshotMtx.Unlock()

	// Set this before cutting to give a more accurate number.
	metricLiveSnapshots.WithLabelValues(i.tenantID).Set(float64(len(i.liveSnapshots)))

	cutoffTime := time.Now().Add(cutoff)
	snapshotsToCut := make([]*liveSnapshot, 0, len(i.liveSnapshots))

	for key, snapshot := range i.liveSnapshots {
		if cutoffTime.After(snapshot.lastAppend) || immediate {
			snapshotsToCut = append(snapshotsToCut, snapshot)
			delete(i.liveSnapshots, key)
		}
	}
	i.snapshotCount.Store(int32(len(i.liveSnapshots)))

	return snapshotsToCut
}

func (i *tenantBlockManager) writeSnapshotToHeadBlock(id common.ID, b []byte, start uint32) error {
	i.blocksMtx.Lock()
	defer i.blocksMtx.Unlock()

	err := i.headBlock.Append(id, b, start)
	if err != nil {
		return err
	}

	return nil
}

// rediscoverLocalBlocks will look from local backend blocks that still exist. These are blocks that have been
// removed from WAL, but not flushed to storage. So we add them to completedBlocks, so they can be flushed again.
func (i *tenantBlockManager) rediscoverLocalBlocks(ctx context.Context) ([]*localBlock, error) {
	ids, err := i.localReader.Blocks(ctx, i.tenantID)
	if err != nil {
		return nil, err
	}

	hasWal := func(id uuid.UUID) bool {
		i.blocksMtx.RLock()
		defer i.blocksMtx.RUnlock()
		for _, b := range i.completingBlocks {
			if b.BlockMeta().BlockID == id {
				return true
			}
		}
		return false
	}

	var rediscoveredBlocks []*localBlock

	for _, id := range ids {

		// Ignore blocks that have a matching wal. The wal will be replayed and the local block recreated.
		// NOTE - Wal replay must be done beforehand.
		if hasWal(id) {
			continue
		}

		// See if block is intact by checking for meta, which is written last.
		// If meta missing then block was not successfully written.
		meta, err := i.localReader.BlockMeta(ctx, id, i.tenantID)
		if err != nil {
			if err == backend.ErrDoesNotExist {
				// Partial/incomplete block found, remove, it will be recreated from data in the wal.
				level.Warn(log.Logger).Log("msg", "Unable to reload meta for local block. This indicates an incomplete block and will be deleted", "tenant", i.tenantID, "block", id.String())
				err = i.local.ClearBlock(id, i.tenantID)
				if err != nil {
					return nil, errors.Wrapf(err, "deleting bad local block tenant %v block %v", i.tenantID, id.String())
				}
			} else {
				// Block with unknown error
				level.Error(log.Logger).Log("msg", "Unexpected error reloading meta for local block. Ignoring and continuing. This block should be investigated.", "tenant", i.tenantID, "block", id.String(), "error", err)
				metricReplayErrorsTotal.WithLabelValues(i.tenantID).Inc()
			}

			continue
		}

		b, err := encoding.OpenBlock(meta, i.localReader)
		if err != nil {
			return nil, err
		}

		ib := newLocalBlock(ctx, b, i.local)
		rediscoveredBlocks = append(rediscoveredBlocks, ib)

		level.Info(log.Logger).Log("msg", "reloaded local block", "tenantID", i.tenantID, "block", id.String(), "flushed", ib.FlushedTime())
	}

	i.blocksMtx.Lock()
	i.completeBlocks = append(i.completeBlocks, rediscoveredBlocks...)
	i.blocksMtx.Unlock()

	return rediscoveredBlocks, nil
}
