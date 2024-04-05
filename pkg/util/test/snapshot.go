package test

import (
	"crypto/rand"
	"fmt"
	"github.com/golang/protobuf/proto"
	rand2 "math/rand"
	"reflect"
	"strconv"
	"time"

	"github.com/google/uuid"
	cp "github.com/intergral/deep/pkg/deeppb/common/v1"
	tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	deeptp "github.com/intergral/go-deep-proto/tracepoint/v1"
)

type GenerateOptions struct {
	Id              []byte
	Attrs           map[string]string
	Resource        map[string]string
	AllResource     bool
	AllAttrs        bool
	AllVarTypes     bool
	AsyncFrame      bool
	TranspiledFrame bool
	ColumnFrame     bool
	NoVars          bool
	RandomStrings   bool
	ServiceName     string
	DurationNanos   uint64
	LogMsg          bool
	RandomDuration  bool
}

func AppendFrame(snapshot *tp.Snapshot) {
	variables := make([]*tp.Variable, len(snapshot.VarLookup))

	snapshot.Frames = append(snapshot.Frames, generateFrames(GenerateOptions{}, variables)...)

	lookup := generateVarLookup(variables)

	for key, variable := range lookup {
		snapshot.VarLookup[key] = variable
	}
}

func GenerateSnapshot(index int, options *GenerateOptions) *tp.Snapshot {
	if options == nil {
		options = &GenerateOptions{
			Id:              MakeSnapshotID(),
			Attrs:           nil,
			Resource:        nil,
			AllResource:     false,
			AllAttrs:        false,
			AllVarTypes:     false,
			AsyncFrame:      false,
			TranspiledFrame: false,
			ColumnFrame:     false,
			NoVars:          false,
			RandomStrings:   false,
			ServiceName:     "test-service",
			DurationNanos:   1010101,
			RandomDuration:  false,
			LogMsg:          false,
		}
	}

	if options.Id == nil {
		options.Id = MakeSnapshotID()
	}

	vars := make([]*tp.Variable, 0)

	watchResults, watches := generateWatchResults()
	point := GenerateTracePoint(index, watches, options)

	duration := options.DurationNanos
	if options.RandomDuration {
		if duration == 0 {
			duration = uint64(rand2.Int())
		} else {
			duration = uint64(rand2.Int63n(int64(duration)))
		}
	}

	snap := &tp.Snapshot{
		ID:            options.Id,
		Tracepoint:    point,
		VarLookup:     nil,
		TsNanos:       uint64(time.Now().UnixNano()),
		Frames:        generateFrames(*options, vars),
		Watches:       watchResults,
		Attributes:    generateAttributes(point, *options),
		DurationNanos: duration,
		Resource:      generateResource(*options),
		LogMsg:        generateLogMsg(options),
	}
	if !options.NoVars {
		snap.VarLookup = generateVarLookup(vars)
	}
	return snap
}

func generateLogMsg(options *GenerateOptions) *string {
	if options.LogMsg {
		stringLen := RandomStringLen(255)
		ret := &stringLen
		return ret
	}
	return nil
}

func MakeSnapshotID() []byte {
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	return id
}

func GenerateTracePoint(index int, watches []string, options *GenerateOptions) *tp.TracePointConfig {
	newUUID, _ := uuid.NewUUID()

	path := "/some/path/to/file_" + strconv.Itoa(index) + ".py"
	if options.RandomStrings {
		path = "/some/path/to/file_" + RandomStringLen(5) + ".py"
	}

	return &tp.TracePointConfig{
		ID:         newUUID.String(),
		Path:       path,
		LineNumber: uint32(index),
		Args:       nil,
		Watches:    watches,
	}
}

func randomString(options GenerateOptions) string {
	if options.RandomStrings {
		return RandomStringLen(5)
	}
	return ""
}

func generateFrames(options GenerateOptions, vars []*tp.Variable) []*tp.StackFrame {
	className := "SimpleTest"
	trueBool := true
	frames := []*tp.StackFrame{
		{
			FileName:   "/usr/src/app/src/simple-app/simple_test" + randomString(options) + ".py",
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
						makeSimpleVariable(vars, "started_at", int64(1682513610601)),
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
			FileName:   "/usr/src/app/src/simple-app/main" + randomString(options) + ".py",
			MethodName: "main",
			LineNumber: 37,
		},
		{
			FileName:   "/usr/src/app/src/simple-app/main.py",
			MethodName: "<module>",
			LineNumber: 49,
		},
	}

	if options.AllVarTypes {
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

	if options.AsyncFrame {
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

	if options.ColumnFrame {
		columnNumber := uint32(222)
		frames = append(frames, &tp.StackFrame{
			FileName:     "/column/frame/test.go",
			MethodName:   "columnFrame",
			LineNumber:   101,
			ColumnNumber: &columnNumber,
		})
	}

	if options.TranspiledFrame {
		transpiledFileName := "/original/file.ts"
		transpiledLineNumber := uint32(202)
		frames = append(frames, &tp.StackFrame{
			FileName:             "/transpiled/frame/test.go",
			MethodName:           "transpiled",
			LineNumber:           101,
			TranspiledFileName:   &transpiledFileName,
			TranspiledLineNumber: &transpiledLineNumber,
		})

		if options.ColumnFrame {
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
	serviceName := "deep-cli"
	if options.ServiceName != "" {
		serviceName = options.ServiceName
	}

	values := []*cp.KeyValue{
		{
			Key: "service.name",
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{
				StringValue: serviceName,
			}},
		},
	}

	if options.RandomStrings {
		values = append(values, &cp.KeyValue{
			Key:   RandomString(),
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: RandomString()}},
		}, &cp.KeyValue{
			Key:   RandomString(),
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: RandomString()}},
		})
	}

	if options.AllResource {
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

	if options.Resource != nil {
		for k, v := range options.Resource {
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

	if options.RandomStrings {
		keyValues = append(keyValues, &cp.KeyValue{
			Key:   RandomString(),
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: RandomString()}},
		}, &cp.KeyValue{
			Key:   RandomString(),
			Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: RandomString()}},
		})
	}

	if options.AllAttrs {
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

	if options.Attrs != nil {
		for k, v := range options.Attrs {
			keyValues = append(keyValues, &cp.KeyValue{
				Key:   k,
				Value: &cp.AnyValue{Value: &cp.AnyValue_StringValue{StringValue: v}},
			})
		}
	}

	return keyValues
}

func generateWatchResults() ([]*tp.WatchResult, []string) {
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
	}, []string{"some.thing", "some.bad"}
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
	varLookup := make(map[string]*tp.Variable, len(vars))

	for i, variable := range vars {
		varLookup[strconv.Itoa(i)] = variable
	}

	return varLookup
}

func ConvertToPublicType(in *tp.Snapshot) (*deeptp.Snapshot, error) {
	convert, err := proto.Marshal(in)
	if err != nil {
		return nil, err
	}

	// deeppb_tp.Snapshot is wire-compatible with go-deep-proto
	snapshot := &deeptp.Snapshot{}
	err = proto.Unmarshal(convert, snapshot)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}
