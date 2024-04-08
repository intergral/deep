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
	"time"

	"github.com/intergral/deep/pkg/util"
	"google.golang.org/grpc"

	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"github.com/intergral/deep/pkg/util/test"
	deep "github.com/intergral/go-deep-proto/tracepoint/v1"
)

type generateSnapshotCmd struct {
	GRCPClient

	Test       bool `help:"Create a known set of test snapshots" default:"False"`
	Count      int  `help:"The number snapshots to generate" default:"1"`
	Sleep      int  `help:"Sleep for this many seconds between iterations" default:"10"`
	Iterations int  `help:"The number of times to repeat the generation. (-1 forever)" default:"1"`

	AllResource     bool   `help:"Include all resource types" default:"False"`
	AllAttrs        bool   `help:"Include all attribute types" default:"False"`
	AllVarTypes     bool   `help:"Include all var types" default:"False"`
	AsyncFrame      bool   `help:"Include async frames" default:"False"`
	TranspiledFrame bool   `help:"Include transpiled frames" default:"False"`
	ColumnFrame     bool   `help:"Include frames with column numbers" default:"False"`
	NoVars          bool   `help:"Generate snapshot with no vars" default:"False"`
	RandomString    bool   `help:"Use random strings in the snapshot" default:"False"`
	ServiceName     string `help:"The service name to use" default:"test-service"`
	DurationNanos   uint64 `help:"The duration to set" default:"1010101"`
	RandomDuration  bool   `help:"Use a random duration. If set duration-nanos is used as the max." default:"False"`
	LogMsg          bool   `help:"Include a random log message" default:"False"`

	vars []*deep.Variable
}

func (cmd *generateSnapshotCmd) Run(*globalOptions) error {
	client := cmd.connectGrpc()
	defer func(connection *grpc.ClientConn) {
		_ = connection.Close()
	}(cmd.connection)

	count := 0
	for {
		if cmd.Iterations > 0 && count >= cmd.Iterations {
			return nil
		}

		if cmd.Test {
			for i := 0; i < cmd.Count; i++ {
				err := cmd.generateTestSnapshots(client)
				if err != nil {
					return err
				}
			}
			continue
		}

		for i := 0; i < cmd.Count; i++ {
			snap := cmd.generateSnapshot(i, &test.GenerateOptions{
				Attrs:           map[string]string{"test_id": "good_snap"},
				AllResource:     cmd.AllResource,
				AllAttrs:        cmd.AllAttrs,
				AllVarTypes:     cmd.AllVarTypes,
				AsyncFrame:      cmd.AsyncFrame,
				TranspiledFrame: cmd.TranspiledFrame,
				ColumnFrame:     cmd.ColumnFrame,
				NoVars:          cmd.NoVars,
				RandomStrings:   cmd.RandomString,
				ServiceName:     cmd.ServiceName,
				DurationNanos:   cmd.DurationNanos,
				RandomDuration:  cmd.RandomDuration,
				LogMsg:          cmd.LogMsg,
			})
			fmt.Printf("Pushing Snapshot: %s\n", util.SnapshotIDToHexString(snap.ID))
			publicType, _ := test.ConvertToPublicType(snap)
			_, err := client.Send(context.TODO(), publicType)
			if err != nil {
				return err
			}
		}
		count++
		duration := time.Duration(cmd.Sleep) * time.Second
		time.Sleep(duration)
	}
}

func (cmd *generateSnapshotCmd) generateSnapshot(index int, options *test.GenerateOptions) *tp.Snapshot {
	return test.GenerateSnapshot(index, options)
}

func (cmd *generateSnapshotCmd) generateTestSnapshots(client deep.SnapshotServiceClient) error {
	snapshots := GenerateTestSnapshots()

	for _, snapshot := range snapshots {
		fmt.Printf("Sending snapshot %s duration %d\n", util.SnapshotIDToHexString(snapshot.ID), snapshot.DurationNanos)
		publicType, _ := test.ConvertToPublicType(snapshot)
		_, _ = client.Send(context.Background(), publicType)
	}

	return nil
}

func GenerateTestSnapshots() []*tp.Snapshot {
	// Create snapshot with no var lookup
	snapshotNoVars := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "no_vars"}, NoVars: true, RandomDuration: true})

	// create good snapshot
	snapshot := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "good_snap"}, RandomDuration: true})

	// create resource snapshot
	resourceSnap := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "resource_snap"}, AllResource: true, RandomDuration: true})

	// create var test snap
	varTest := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "var_test"}, AllVarTypes: true, RandomDuration: true})

	// create frame test snap
	frameTest := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "frame_test"}, AsyncFrame: true, ColumnFrame: true, TranspiledFrame: true, RandomDuration: true})

	// create all tags snapshot
	tagsSnapshot := test.GenerateSnapshot(0, &test.GenerateOptions{Attrs: map[string]string{"test_id": "all_tags"}, AllAttrs: true, RandomDuration: true})

	return []*tp.Snapshot{snapshotNoVars, snapshot, resourceSnap, varTest, frameTest, tagsSnapshot}
}
