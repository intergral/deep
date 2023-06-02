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
