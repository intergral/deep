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

package receivers

import (
	"context"
	"github.com/intergral/deep/pkg/util/log"
	pb "github.com/intergral/go-deep-proto/poll/v1"
	tp "github.com/intergral/go-deep-proto/tracepoint/v1"
	"reflect"
	"testing"
)

func Snapshot(ctx context.Context, in *tp.Snapshot) (*tp.SnapshotResponse, error) {
	return nil, nil
}

func Poll(ctx context.Context, pollRequest *pb.PollRequest) (*pb.PollResponse, error) {
	return nil, nil
}

func TestLoadDeepReceiver(t *testing.T) {
	cfg := map[string]interface{}{
		"deep": nil,
	}

	receivers, err := ForConfig(cfg, Snapshot, Poll, log.Logger)
	if err != nil {
		t.Errorf("Error = %v", err)
		return
	}

	if len(receivers) != 1 {
		t.Errorf("Failed to load receviers")
		return
	}

	of := reflect.TypeOf(receivers[0].(interface{}))
	name := of.String()
	if name != "*deep.deepReceiver" {
		t.Errorf("Exepected deep got %s", name)
		return
	}
}
