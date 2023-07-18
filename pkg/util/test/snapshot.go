package test

import (
	"crypto/rand"
	"fmt"
	"github.com/google/uuid"
	cp "github.com/intergral/deep/pkg/deeppb/common/v1"
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	"reflect"
	"strconv"
	"time"
)

type GenerateOptions struct {
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

func GenerateSnapshot(index int, options *GenerateOptions) *tp.Snapshot {
	if options == nil {
		options = &GenerateOptions{
			attrs:           nil,
			resource:        nil,
			allResource:     false,
			allAttrs:        false,
			allVarTypes:     false,
			asyncFrame:      false,
			transpiledFrame: false,
			columnFrame:     false,
			noVars:          false,
			serviceName:     "test-service",
		}
	}
	vars := make([]*tp.Variable, 0)

	point := GenerateTracePoint(index)
	snap := &tp.Snapshot{
		ID:            MakeSnapshotID(),
		Tracepoint:    point,
		VarLookup:     nil,
		TsNanos:       uint64(time.Now().UnixNano()),
		Frames:        generateFrames(*options, vars),
		Watches:       generateWatchResults(),
		Attributes:    generateAttributes(point, *options),
		DurationNanos: 1010101,
		Resource:      generateResource(*options),
	}
	if !options.noVars {
		snap.VarLookup = generateVarLookup(vars)
	}
	return snap
}

func MakeSnapshotID() []byte {
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	return id
}

func GenerateTracePoint(index int) *tp.TracePointConfig {
	newUUID, _ := uuid.NewUUID()
	return &tp.TracePointConfig{
		ID:         newUUID.String(),
		Path:       "/some/path/to/file_" + strconv.Itoa(index) + ".py",
		LineNumber: uint32(index),
		Args:       nil,
		Watches:    nil,
	}
}

func generateFrames(options GenerateOptions, vars []*tp.Variable) []*tp.StackFrame {
	className := "SimpleTest"
	trueBool := true
	frames := []*tp.StackFrame{
		{
			FileName:   "/usr/src/app/src/simple-app/simple_test.py",
			MethodName: "message",
			LineNumber: 31,
			ClassName:  &className,
			Variables: []*tp.VariableID{
				makeVariableWithType(vars, "uuid", "e071bd7c-2dc6-4e77-860e-54d48eabff42", "str", []string{}, false),
				makeVariable(vars, &tp.VariableID{
					Name: "self",
				}, &tp.Variable{
					Type:  "self",
					Value: "SimpleTest:This is a test:1682513610601",
					Hash:  "140425155304272",
					Children: []*tp.VariableID{
						makeSimpleVariable(vars, "max_executions", 31),
						makeSimpleVariable(vars, "cnt", 25),
						makeSimpleVariable(vars, "started_at", 1682513610601),
						makeSimpleVariable(vars, "test_name", "This is a test"),
						makeVariable(vars, &tp.VariableID{Name: "char_counter"}, &tp.Variable{Type: "dict", Value: "Size: 3", Hash: "140425155619776", Children: []*tp.VariableID{
							makeSimpleVariable(vars, "1", 23),
							makeSimpleVariable(vars, "a", 12),
							makeSimpleVariable(vars, "f", 9),
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
		frames = append(frames, &tp.StackFrame{
			FileName:   "/var/text/frame.go",
			MethodName: "varTest",
			LineNumber: 101,
			Variables: []*tp.VariableID{
				makeSimpleVariable(vars, "bool", true),
				makeSimpleVariable(vars, "int", 1),
				makeSimpleVariable(vars, "double", 1.34),
				makeSimpleVariable(vars, "str", "a string"),
				makeVariable(vars, &tp.VariableID{Name: "modified_name", OriginalName: &originalName}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "private", Modifiers: []string{"private"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "protected", Modifiers: []string{"protected"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "package", Modifiers: []string{"package"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "public", Modifiers: []string{"public"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "static", Modifiers: []string{"static"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "volatile", Modifiers: []string{"volatile"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "final", Modifiers: []string{"final"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "transient", Modifiers: []string{"transient"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "synchronized", Modifiers: []string{"synchronized"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "pri_static_sync", Modifiers: []string{"synchronized", "private", "static"}}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123"}),
				makeVariable(vars, &tp.VariableID{Name: "truncated"}, &tp.Variable{Type: "str", Value: "a thing", Hash: "123", Truncated: &trueBool}),
				makeVariable(vars, &tp.VariableID{Name: "no hash"}, &tp.Variable{Type: "str", Value: "a thing"}),
			},
		})
	}

	if options.asyncFrame {
		frames = append(frames, &tp.StackFrame{
			FileName:   "/async/frame/test.go",
			MethodName: "asyncFrame",
			LineNumber: 101,
			IsAsync:    &trueBool,
		})
		frames = append(frames, &tp.StackFrame{
			FileName:   "/async/appframe/test.go",
			MethodName: "asyncFrame",
			LineNumber: 101,
			IsAsync:    &trueBool,
			AppFrame:   &trueBool,
		})
	}

	if options.columnFrame {
		columnNumber := uint32(222)
		frames = append(frames, &tp.StackFrame{
			FileName:     "/column/frame/test.go",
			MethodName:   "columnFrame",
			LineNumber:   101,
			ColumnNumber: &columnNumber,
		})
	}

	if options.transpiledFrame {
		transpiledFileName := "/original/file.ts"
		transpiledLineNumber := uint32(202)
		frames = append(frames, &tp.StackFrame{
			FileName:             "/transpiled/frame/test.go",
			MethodName:           "transpiled",
			LineNumber:           101,
			TranspiledFileName:   &transpiledFileName,
			TranspiledLineNumber: &transpiledLineNumber,
		})

		if options.columnFrame {
			columnNumber := uint32(333)
			tpColumnNumber := uint32(111)
			frames = append(frames, &tp.StackFrame{
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

func generateResource(options GenerateOptions) []*cp.KeyValue {
	var serviceName = "deep-cli"
	if options.serviceName != "" {
		serviceName = options.serviceName
	}

	values := []*cp.KeyValue{
		{
			Key: "service.name",
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{
				StringValue: serviceName,
			}},
		},
	}

	if options.allResource {
		allTypes := []*cp.KeyValue{
			{
				Key:   "str_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "StringValue"}},
			},
			{
				Key:   "int_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_IntValue{IntValue: 101}},
			},
			{
				Key:   "double_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_DoubleValue{DoubleValue: 3.14}},
			},
			{
				Key:   "bool_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_BoolValue{BoolValue: false}},
			},
			{
				Key: "arr_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_ArrayValue{ArrayValue: &cp.ArrayValue{
					Values: []*cp.AnyValue{
						{Value: &cp.AnyValue_StringValue{StringValue: "StringValue"}},
						{Value: &cp.AnyValue_IntValue{IntValue: 101}},
						{Value: &cp.AnyValue_DoubleValue{DoubleValue: 3.14}},
						{Value: &cp.AnyValue_BoolValue{BoolValue: false}},
						{Value: &cp.AnyValue_BytesValue{BytesValue: []byte("some bytes")}},
					},
				}}},
			},
			{
				Key: "kv_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_KvlistValue{KvlistValue: &cp.KeyValueList{Values: []*cp.KeyValue{
					{
						Key:   "str_type",
						Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "StringValue"}},
					},
					{
						Key:   "int_type",
						Value: &cp.AnyValue{Value: &cp.AnyValue_IntValue{IntValue: 101}},
					},
					{
						Key:   "double_type",
						Value: &cp.AnyValue{Value: &cp.AnyValue_DoubleValue{DoubleValue: 3.14}},
					},
					{
						Key:   "bool_type",
						Value: &cp.AnyValue{Value: &cp.AnyValue_BoolValue{BoolValue: false}},
					},
				}}}},
			},
		}
		values = append(values, allTypes...)
	}

	if options.resource != nil {
		for k, v := range options.resource {
			values = append(values, &cp.KeyValue{
				Key:   k,
				Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: v}},
			})
		}
	}
	return values
}

func generateAttributes(tp *tp.TracePointConfig, options GenerateOptions) []*cp.KeyValue {
	keyValues := []*cp.KeyValue{
		{
			Key:   "path",
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: tp.Path}},
		},
		{
			Key:   "line",
			Value: &cp.AnyValue{Value: &cp.AnyValue_IntValue{IntValue: int64(tp.LineNumber)}},
		},
		{
			Key:   "frame",
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "single_frame"}},
		},
	}

	if options.allAttrs {
		allTypes := []*cp.KeyValue{
			{
				Key:   "str_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "StringValue"}},
			},
			{
				Key:   "int_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_IntValue{IntValue: 101}},
			},
			{
				Key:   "double_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_DoubleValue{DoubleValue: 3.14}},
			},
			{
				Key:   "bool_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_BoolValue{BoolValue: false}},
			},
			{
				Key: "arr_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_ArrayValue{ArrayValue: &cp.ArrayValue{
					Values: []*cp.AnyValue{
						{Value: &cp.AnyValue_StringValue{StringValue: "StringValue"}},
						{Value: &cp.AnyValue_IntValue{IntValue: 101}},
						{Value: &cp.AnyValue_DoubleValue{DoubleValue: 3.14}},
						{Value: &cp.AnyValue_BoolValue{BoolValue: false}},
						{Value: &cp.AnyValue_BytesValue{BytesValue: []byte("some bytes")}},
					},
				}}},
			},
			{
				Key: "kv_type",
				Value: &cp.AnyValue{Value: &cp.AnyValue_KvlistValue{KvlistValue: &cp.KeyValueList{Values: []*cp.KeyValue{
					{
						Key:   "str_type",
						Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: "StringValue"}},
					},
					{
						Key:   "int_type",
						Value: &cp.AnyValue{Value: &cp.AnyValue_IntValue{IntValue: 101}},
					},
					{
						Key:   "double_type",
						Value: &cp.AnyValue{Value: &cp.AnyValue_DoubleValue{DoubleValue: 3.14}},
					},
					{
						Key:   "bool_type",
						Value: &cp.AnyValue{Value: &cp.AnyValue_BoolValue{BoolValue: false}},
					},
				}}}},
			},
		}
		keyValues = append(keyValues, allTypes...)
	}

	if options.attrs != nil {
		for k, v := range options.attrs {
			keyValues = append(keyValues, &cp.KeyValue{
				Key:   k,
				Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: v}},
			})
		}
	}

	return keyValues
}

func generateWatchResults() []*tp.WatchResult {
	return []*tp.WatchResult{
		{
			Expression: "some.thing",
			Result: &tp.WatchResult_GoodResult{
				GoodResult: makeSimpleVariable(nil, "some.thing", "i am the result"),
			},
		},
		{
			Expression: "some.bad",
			Result:     &tp.WatchResult_ErrorResult{ErrorResult: "Cannot find 'bad' on object 'some'"},
		},
	}
}

func makeVariable(vars []*tp.Variable, varId *tp.VariableID, variable *tp.Variable) *tp.VariableID {
	if varId.ID == "" {
		varId.ID = strconv.Itoa(len(vars))
	}
	vars = append(vars, variable)
	return varId
}

func makeVariableWithType(vars []*tp.Variable, name string, value string, typeStr string, modifiers []string, truncated bool) *tp.VariableID {
	varId := &tp.VariableID{
		ID:           strconv.Itoa(len(vars)),
		Name:         name,
		Modifiers:    modifiers,
		OriginalName: nil,
	}

	variable := &tp.Variable{
		Type:      typeStr,
		Value:     value,
		Hash:      fmt.Sprint(&value),
		Children:  nil,
		Truncated: &truncated,
	}

	return makeVariable(nil, varId, variable)
}

func makeSimpleVariable(vars []*tp.Variable, name string, value interface{}) *tp.VariableID {
	typeOf := reflect.TypeOf(value)
	valStr := fmt.Sprintf("%v", value)
	return makeVariableWithType(vars, name, valStr, typeOf.Name(), []string{}, false)
}

func generateVarLookup(vars []*tp.Variable) map[string]*tp.Variable {
	var varLookup = make(map[string]*tp.Variable, len(vars))

	for i, variable := range vars {
		varLookup[strconv.Itoa(i)] = variable
	}

	return varLookup
}