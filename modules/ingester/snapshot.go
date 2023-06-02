package ingester

import (
	"context"
	"fmt"
	"time"

	"github.com/intergral/deep/pkg/model"
)

type liveSnapshot struct {
	bytes      []byte
	lastAppend time.Time
	snapshotId []byte
	start      uint32
	decoder    model.SegmentDecoder

	// byte limits
	maxBytes     int
	currentBytes int
}

func newLiveSnapshot(id []byte, maxBytes int) *liveSnapshot {
	return &liveSnapshot{
		bytes:      make([]byte, 0, 10), // 10 for luck
		lastAppend: time.Now(),
		snapshotId: id,
		maxBytes:   maxBytes,
		decoder:    model.MustNewSegmentDecoder(model.CurrentEncoding),
	}
}

func (t *liveSnapshot) Push(_ context.Context, instanceID string, snapshot []byte) error {
	t.lastAppend = time.Now()
	if t.maxBytes != 0 {
		reqSize := len(snapshot)
		if t.currentBytes+reqSize > t.maxBytes {
			return newSnapshotTooLargeError(t.snapshotId, instanceID, t.maxBytes, reqSize)
		}

		t.currentBytes += reqSize
	}

	start, err := t.decoder.FastRange(snapshot)
	if err != nil {
		return fmt.Errorf("failed to get range while adding segment: %w", err)
	}
	t.bytes = snapshot
	t.start = start

	return nil
}
