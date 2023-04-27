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
	"crypto/rand"
	"fmt"
	"github.com/google/uuid"
	deep_common "github.com/intergral/go-deep-proto/common/v1"
	deep "github.com/intergral/go-deep-proto/tracepoint/v1"
	"google.golang.org/grpc"
	"reflect"
	"strconv"
	"time"
)

type generateSnapshotCmd struct {
	GRCPClient

	Test  bool `help:"Create a known set of test snapshots" default:"False"`
	Count int  `help:"The number snapshots to generate" default:"1"`

	vars []*deep.Variable
}

type generateOptions struct {
	attrs           map[string]string
	resource        map[string]string
	allResource     bool
	allAttrs        bool
	allVarTypes     bool
	asyncFrame      bool
	transpiledFrame bool
	columnFrame     bool
	noVars          bool
	serviceName     string
}

func (cmd *generateSnapshotCmd) Run(opts *globalOptions) error {
	if cmd.Test {
		return cmd.generateTestSnapshots()
	}
	client := cmd.connectGrpc()
	defer func(connection *grpc.ClientConn) {
		_ = connection.Close()
	}(cmd.connection)

	for i := 0; i < cmd.Count; i++ {
		snap := cmd.generateSnapshot(i, generateOptions{attrs: map[string]string{"test_id": "good_snap"}})
		fmt.Printf("%+v\n", snap)
		_, _ = client.Send(context.TODO(), snap)
	}
	return nil
}

func (cmd *generateSnapshotCmd) generateSnapshot(index int, options generateOptions) *deep.Snapshot {
	point := cmd.generateTracePoint(index)
	snap := &deep.Snapshot{
		ID:            makeSnapshotID(),
		Tracepoint:    point,
		VarLookup:     nil,
		TsNanos:       uint64(time.Now().UnixNano()),
		Frames:        cmd.generateFrames(index, options),
		Watches:       cmd.generateWatchResults(),
		Attributes:    cmd.generateAttributes(point, options),
		DurationNanos: 1010101,
		Resource:      cmd.generateResource(options),
	}
	if !options.noVars {
		snap.VarLookup = cmd.generateVarLookup()
	}
	cmd.vars = make([]*deep.Variable, 0)
	return snap
}

func (cmd *generateSnapshotCmd) generateTracePoint(index int) *deep.TracePointConfig {
	newUUID, _ := uuid.NewUUID()
	return &deep.TracePointConfig{
		ID:         newUUID.String(),
		Path:       "/some/path/to/file_" + strconv.Itoa(index) + ".py",
		LineNumber: uint32(index),
		Args:       nil,
		Watches:    nil,
	}
}

func (cmd *generateSnapshotCmd) generateFrames(index int, options generateOptions) []*deep.StackFrame {
	className := "SimpleTest"
	trueBool := true
	frames := []*deep.StackFrame{
		{
			FileName:   "/usr/src/app/src/simple-app/simple_test.py",
			MethodName: "message",
			LineNumber: 31,
			ClassName:  &className,
			Variables: []*deep.VariableID{
				cmd.makeVariableWithType("uuid", "e071bd7c-2dc6-4e77-860e-54d48eabff42", "str", []string{}, false),
				cmd.makeVariable(&deep.VariableID{
					Name: "self",
				}, &deep.Variable{
					Type:  "self",
					Value: "SimpleTest:This is a test:1682513610601",
					Hash:  "140425155304272",
					Children: []*deep.VariableID{
						cmd.makeSimpleVariable("max_executions", 31),
						cmd.makeSimpleVariable("cnt", 25),
						cmd.makeSimpleVariable("started_at", 1682513610601),
						cmd.makeSimpleVariable("test_name", "This is a test"),
						cmd.makeVariable(&deep.VariableID{Name: "char_counter"}, &deep.Variable{Type: "dict", Value: "Size: 3", Hash: "140425155619776", Children: []*deep.VariableID{
							cmd.makeSimpleVariable("1", 23),
							cmd.makeSimpleVariable("a", 12),
							cmd.makeSimpleVariable("f", 9),
						}}),
					},
				}),
			},
			AppFrame: &trueBool,
		},
		{
			FileName:   "/usr/src/app/src/simple-app/main.py",
			MethodName: "main",
			LineNumber: 37,
		},
		{
			FileName:   "/usr/src/app/src/simple-app/main.py",
			MethodName: "<module>",
			LineNumber: 49,
		},
	}

	if options.allVarTypes {
		originalName := "original name"
		frames = append(frames, &deep.StackFrame{
			FileName:   "/var/text/frame.go",
			MethodName: "varTest",
			LineNumber: 101,
			Variables: []*deep.VariableID{
				cmd.makeSimpleVariable("bool", true),
				cmd.makeSimpleVariable("int", 1),
				cmd.makeSimpleVariable("double", 1.34),
				cmd.makeSimpleVariable("str", "a string"),
				cmd.makeVariable(&deep.VariableID{Name: "modified_name", OriginalName: &originalName}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "private", Modifiers: []string{"private"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "protected", Modifiers: []string{"protected"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "package", Modifiers: []string{"package"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "public", Modifiers: []string{"public"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "static", Modifiers: []string{"static"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "volatile", Modifiers: []string{"volatile"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "final", Modifiers: []string{"final"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "transient", Modifiers: []string{"transient"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "synchronized", Modifiers: []string{"synchronized"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "pri_static_sync", Modifiers: []string{"synchronized", "private", "static"}}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				cmd.makeVariable(&deep.VariableID{Name: "truncated"}, &deep.Variable{Type: "str", Value: "a thing", Hash: "123", Truncated: &trueBool}),
				cmd.makeVariable(&deep.VariableID{Name: "no hash"}, &deep.Variable{Type: "str", Value: "a thing"}),
			},
		})
	}

	if options.asyncFrame {
		frames = append(frames, &deep.StackFrame{
			FileName:   "/async/frame/test.go",
			MethodName: "asyncFrame",
			LineNumber: 101,
			IsAsync:    &trueBool,
		})
		frames = append(frames, &deep.StackFrame{
			FileName:   "/async/appframe/test.go",
			MethodName: "asyncFrame",
			LineNumber: 101,
			IsAsync:    &trueBool,
			AppFrame:   &trueBool,
		})
	}

	if options.columnFrame {
		columnNumber := uint32(222)
		frames = append(frames, &deep.StackFrame{
			FileName:     "/column/frame/test.go",
			MethodName:   "columnFrame",
			LineNumber:   101,
			ColumnNumber: &columnNumber,
		})
	}

	if options.transpiledFrame {
		transpiledFileName := "/original/file.ts"
		transpiledLineNumber := uint32(202)
		frames = append(frames, &deep.StackFrame{
			FileName:             "/transpiled/frame/test.go",
			MethodName:           "transpiled",
			LineNumber:           101,
			TranspiledFileName:   &transpiledFileName,
			TranspiledLineNumber: &transpiledLineNumber,
		})

		if options.columnFrame {
			columnNumber := uint32(333)
			tpColumnNumber := uint32(111)
			frames = append(frames, &deep.StackFrame{
				FileName:               "/transpiled/column/test.go",
				MethodName:             "transpiled",
				LineNumber:             101,
				ColumnNumber:           &columnNumber,
				TranspiledFileName:     &transpiledFileName,
				TranspiledLineNumber:   &transpiledLineNumber,
				TranspiledColumnNumber: &tpColumnNumber,
			})
		}
	}

	return frames
}

func (cmd *generateSnapshotCmd) generateResource(options generateOptions) []*deep_common.KeyValue {
	var serviceName = "deep-cli"
	if options.serviceName != "" {
		serviceName = options.serviceName
	}

	values := []*deep_common.KeyValue{
		{
			Key: "service.name",
			Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_StringValue{
				StringValue: serviceName,
			}},
		},
	}

	if options.allResource {
		allTypes := []*deep_common.KeyValue{
			{
				Key:   "str_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_StringValue{StringValue: "StringValue"}},
			},
			{
				Key:   "int_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_IntValue{IntValue: 101}},
			},
			{
				Key:   "double_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_DoubleValue{DoubleValue: 3.14}},
			},
			{
				Key:   "bool_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_BoolValue{BoolValue: false}},
			},
			{
				Key: "arr_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_ArrayValue{ArrayValue: &deep_common.ArrayValue{
					Values: []*deep_common.AnyValue{
						{Value: &deep_common.AnyValue_StringValue{StringValue: "StringValue"}},
						{Value: &deep_common.AnyValue_IntValue{IntValue: 101}},
						{Value: &deep_common.AnyValue_DoubleValue{DoubleValue: 3.14}},
						{Value: &deep_common.AnyValue_BoolValue{BoolValue: false}},
						{Value: &deep_common.AnyValue_BytesValue{BytesValue: []byte("some bytes")}},
					},
				}}},
			},
			{
				Key: "kv_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_KvlistValue{KvlistValue: &deep_common.KeyValueList{Values: []*deep_common.KeyValue{
					{
						Key:   "str_type",
						Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_StringValue{StringValue: "StringValue"}},
					},
					{
						Key:   "int_type",
						Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_IntValue{IntValue: 101}},
					},
					{
						Key:   "double_type",
						Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_DoubleValue{DoubleValue: 3.14}},
					},
					{
						Key:   "bool_type",
						Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_BoolValue{BoolValue: false}},
					},
				}}}},
			},
		}
		values = append(values, allTypes...)
	}

	if options.resource != nil {
		for k, v := range options.resource {
			values = append(values, &deep_common.KeyValue{
				Key:   k,
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_StringValue{StringValue: v}},
			})
		}
	}
	return values
}

func (cmd *generateSnapshotCmd) generateAttributes(tp *deep.TracePointConfig, options generateOptions) []*deep_common.KeyValue {
	keyValues := []*deep_common.KeyValue{
		{
			Key:   "path",
			Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_StringValue{StringValue: tp.Path}},
		},
		{
			Key:   "line",
			Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_IntValue{IntValue: int64(tp.LineNumber)}},
		},
		{
			Key:   "frame",
			Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_StringValue{StringValue: "single_frame"}},
		},
	}

	if options.allAttrs {
		allTypes := []*deep_common.KeyValue{
			{
				Key:   "str_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_StringValue{StringValue: "StringValue"}},
			},
			{
				Key:   "int_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_IntValue{IntValue: 101}},
			},
			{
				Key:   "double_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_DoubleValue{DoubleValue: 3.14}},
			},
			{
				Key:   "bool_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_BoolValue{BoolValue: false}},
			},
			{
				Key: "arr_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_ArrayValue{ArrayValue: &deep_common.ArrayValue{
					Values: []*deep_common.AnyValue{
						{Value: &deep_common.AnyValue_StringValue{StringValue: "StringValue"}},
						{Value: &deep_common.AnyValue_IntValue{IntValue: 101}},
						{Value: &deep_common.AnyValue_DoubleValue{DoubleValue: 3.14}},
						{Value: &deep_common.AnyValue_BoolValue{BoolValue: false}},
						{Value: &deep_common.AnyValue_BytesValue{BytesValue: []byte("some bytes")}},
					},
				}}},
			},
			{
				Key: "kv_type",
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_KvlistValue{KvlistValue: &deep_common.KeyValueList{Values: []*deep_common.KeyValue{
					{
						Key:   "str_type",
						Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_StringValue{StringValue: "StringValue"}},
					},
					{
						Key:   "int_type",
						Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_IntValue{IntValue: 101}},
					},
					{
						Key:   "double_type",
						Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_DoubleValue{DoubleValue: 3.14}},
					},
					{
						Key:   "bool_type",
						Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_BoolValue{BoolValue: false}},
					},
				}}}},
			},
		}
		keyValues = append(keyValues, allTypes...)
	}

	if options.attrs != nil {
		for k, v := range options.attrs {
			keyValues = append(keyValues, &deep_common.KeyValue{
				Key:   k,
				Value: &deep_common.AnyValue{Value: &deep_common.AnyValue_StringValue{StringValue: v}},
			})
		}
	}

	return keyValues
}

func (cmd *generateSnapshotCmd) generateWatchResults() []*deep.WatchResult {
	return []*deep.WatchResult{
		{
			Expression: "some.thing",
			Result: &deep.WatchResult_GoodResult{
				GoodResult: cmd.makeSimpleVariable("some.thing", "i am the result"),
			},
		},
		{
			Expression: "some.bad",
			Result:     &deep.WatchResult_ErrorResult{ErrorResult: "Cannot find 'bad' on object 'some'"},
		},
	}
}

func (cmd *generateSnapshotCmd) makeVariable(varId *deep.VariableID, variable *deep.Variable) *deep.VariableID {
	if varId.ID == "" {
		varId.ID = strconv.Itoa(len(cmd.vars))
	}
	cmd.vars = append(cmd.vars, variable)
	return varId
}

func (cmd *generateSnapshotCmd) makeVariableWithType(name string, value string, typeStr string, modifiers []string, truncated bool) *deep.VariableID {
	varId := &deep.VariableID{
		ID:           strconv.Itoa(len(cmd.vars)),
		Name:         name,
		Modifiers:    modifiers,
		OriginalName: nil,
	}

	variable := &deep.Variable{
		Type:      typeStr,
		Value:     value,
		Hash:      fmt.Sprint(&value),
		Children:  nil,
		Truncated: &truncated,
	}

	return cmd.makeVariable(varId, variable)
}

func (cmd *generateSnapshotCmd) makeSimpleVariable(name string, value interface{}) *deep.VariableID {
	typeOf := reflect.TypeOf(value)
	valStr := fmt.Sprintf("%v", value)
	return cmd.makeVariableWithType(name, valStr, typeOf.Name(), []string{}, false)
}

func (cmd *generateSnapshotCmd) generateVarLookup() map[string]*deep.Variable {
	var varLookup = make(map[string]*deep.Variable, len(cmd.vars))

	for i, variable := range cmd.vars {
		varLookup[strconv.Itoa(i)] = variable
	}

	return varLookup
}

func (cmd *generateSnapshotCmd) generateTestSnapshots() error {

	client := cmd.connectGrpc()
	defer func(connection *grpc.ClientConn) {
		_ = connection.Close()
	}(cmd.connection)

	// Create snapshot with no var lookup
	snapshotNoVars := cmd.generateSnapshot(0, generateOptions{attrs: map[string]string{"test_id": "no_vars"}, noVars: true})
	_, _ = client.Send(context.TODO(), snapshotNoVars)

	// create good snapshot
	snapshot := cmd.generateSnapshot(0, generateOptions{attrs: map[string]string{"test_id": "good_snap"}})
	_, _ = client.Send(context.TODO(), snapshot)

	// create resource snapshot
	resourceSnap := cmd.generateSnapshot(0, generateOptions{attrs: map[string]string{"test_id": "resource_snap"}, allResource: true})
	_, _ = client.Send(context.TODO(), resourceSnap)

	// create var test snap
	varTest := cmd.generateSnapshot(0, generateOptions{attrs: map[string]string{"test_id": "var_test"}, allVarTypes: true})
	_, _ = client.Send(context.TODO(), varTest)

	// create frame test snap
	frameTest := cmd.generateSnapshot(0, generateOptions{attrs: map[string]string{"test_id": "frame_test"}, asyncFrame: true, columnFrame: true, transpiledFrame: true})
	_, _ = client.Send(context.TODO(), frameTest)

	// create all tags snapshot
	tagsSnapshot := cmd.generateSnapshot(0, generateOptions{attrs: map[string]string{"test_id": "all_tags"}, allAttrs: true})
	_, _ = client.Send(context.TODO(), tagsSnapshot)

	return nil
}

func makeSnapshotID() []byte {
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	return id
}
