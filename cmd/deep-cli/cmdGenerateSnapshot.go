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

package main

import (
	"context"
	"fmt"

	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/util/test"
	deep "github.com/intergral/go-deep-proto/tracepoint/v1"
	"google.golang.org/grpc"
)

type generateSnapshotCmd struct {
	GRCPClient

	Test  bool `help:"Create a known set of test snapshots" default:"False"`
	Count int  `help:"The number snapshots to generate" default:"1"`

	vars []*deep.Variable
}

func (cmd *generateSnapshotCmd) Run(*globalOptions) error {
	if cmd.Test {
		return cmd.generateTestSnapshots()
	}
	client := cmd.connectGrpc()
	defer func(connection *grpc.ClientConn) {
		_ = connection.Close()
	}(cmd.connection)

	for i := 0; i < cmd.Count; i++ {
		snap := cmd.generateSnapshot(i, &test.GenerateOptions{Attrs: map[string]string{"test_id": "good_snap"}})
		fmt.Printf("%+v\n", snap)
		_, _ = client.Send(context.TODO(), snap)
	}
	return nil
}

func (cmd *generateSnapshotCmd) generateSnapshot(index int, options *test.GenerateOptions) *tp.Snapshot {
	return test.GenerateSnapshot(index, options)
}

func (cmd *generateSnapshotCmd) generateTestSnapshots() error {
	client := cmd.connectGrpc()
	defer func(connection *grpc.ClientConn) {
		_ = connection.Close()
	}(cmd.connection)

	snapshots := GenerateTestSnapshots()

	for _, snapshot := range snapshots {
		_, _ = client.Send(context.Background(), snapshot)
	}

	return nil
}

func GenerateTestSnapshots() []*tp.Snapshot {
	// Create snapshot with no var lookup
	snapshotNoVars := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "no_vars"}, NoVars: true})

	// create good snapshot
	snapshot := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "good_snap"}})

	// create resource snapshot
	resourceSnap := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "resource_snap"}, AllResource: true})

	// create var test snap
	varTest := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "var_test"}, AllVarTypes: true})

	// create frame test snap
	frameTest := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "frame_test"}, AsyncFrame: true, ColumnFrame: true, TranspiledFrame: true})

	// create all tags snapshot
	tagsSnapshot := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "all_tags"}, AllAttrs: true})

	return []*tp.Snapshot{snapshotNoVars, snapshot, resourceSnap, varTest, frameTest, tagsSnapshot}
}
