package vparquet

import (
	"bytes"
	deep_common "github.com/intergral/deep/pkg/deeppb/common/v1"
	deep_tp "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"

	"github.com/golang/protobuf/jsonpb" //nolint:all //deprecated
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/util"
)

// Label names for conversion b/n Proto <> Parquet
const (
	LabelRootSpanName    = "root.name"
	LabelRootServiceName = "root.service.name"

	LabelServiceName = "service.name"
	LabelCluster     = "cluster"
	LabelNamespace   = "namespace"
	LabelPod         = "pod"
	LabelContainer   = "container"

	LabelK8sClusterName   = "k8s.cluster.name"
	LabelK8sNamespaceName = "k8s.namespace.name"
	LabelK8sPodName       = "k8s.pod.name"
	LabelK8sContainerName = "k8s.container.name"
)

// These definition levels match the schema below
const (
	DefinitionLevelTrace                     = 0
	DefinitionLevelResourceSpans             = 1
	DefinitionLevelResourceAttrs             = 2
	DefinitionLevelResourceSpansILSSpan      = 3
	DefinitionLevelResourceSpansILSSpanAttrs = 4

	FieldResourceAttrKey       = "rs.Resource.Attrs.Key"
	FieldResourceAttrVal       = "rs.Resource.Attrs.Value"
	FieldResourceAttrValInt    = "rs.Resource.Attrs.ValueInt"
	FieldResourceAttrValDouble = "rs.Resource.Attrs.ValueDouble"
	FieldResourceAttrValBool   = "rs.Resource.Attrs.ValueBool"

	FieldAttrKey       = "rs.Attributes.Key"
	FieldAttrVal       = "rs.Attributes.Value"
	FieldAttrValInt    = "rs.Attributes.ValueInt"
	FieldAttrValDouble = "rs.Attributes.ValueDouble"
	FieldAttrValBool   = "rs.Attributes.ValueBool"
)

var (
	jsonMarshaler = new(jsonpb.Marshaler)

	labelMappings = map[string]string{
		LabelServiceName:      "rs.Resource.ServiceName",
		LabelCluster:          "rs.Resource.Cluster",
		LabelNamespace:        "rs.Resource.Namespace",
		LabelPod:              "rs.Resource.Pod",
		LabelContainer:        "rs.Resource.Container",
		LabelK8sClusterName:   "rs.Resource.K8sClusterName",
		LabelK8sNamespaceName: "rs.Resource.K8sNamespaceName",
		LabelK8sPodName:       "rs.Resource.K8sPodName",
		LabelK8sContainerName: "rs.Resource.K8sContainerName",
	}
)

type Attribute struct {
	Key string `parquet:",snappy,dict"`

	// This is a bad design that leads to millions of null values. How can we fix this?
	Value       *string  `parquet:",dict,snappy,optional"`
	ValueInt    *int64   `parquet:",snappy,optional"`
	ValueDouble *float64 `parquet:",snappy,optional"`
	ValueBool   *bool    `parquet:",snappy,optional"`
	ValueKVList string   `parquet:",snappy,optional"`
	ValueArray  string   `parquet:",snappy,optional"`
}

type Resource struct {
	Attrs []Attribute

	// Known attributes
	ServiceName      string  `parquet:",snappy,dict"`
	Cluster          *string `parquet:",snappy,optional,dict"`
	Namespace        *string `parquet:",snappy,optional,dict"`
	Pod              *string `parquet:",snappy,optional,dict"`
	Container        *string `parquet:",snappy,optional,dict"`
	K8sClusterName   *string `parquet:",snappy,optional,dict"`
	K8sNamespaceName *string `parquet:",snappy,optional,dict"`
	K8sPodName       *string `parquet:",snappy,optional,dict"`
	K8sContainerName *string `parquet:",snappy,optional,dict"`

	Test string `parquet:",snappy,dict,optional"` // Always empty for testing
}

type TracePointConfig struct {
	ID      string            `parquet:",snappy,dict"`
	Path    string            `parquet:",snappy,dict"`
	LineNo  int32             `parquet:",delta"`
	Args    map[string]string `parquet:""`
	Watches []string          `parquet:""`
}

type VariableID struct {
	ID        string   `parquet:",snappy,dict"`
	Name      string   `parquet:",snappy,dict"`
	Modifiers []string `parquet:""`
}

type Variable struct {
	Type      string       `parquet:",snappy,dict"`
	Value     string       `parquet:",snappy,dict"`
	Hash      string       `parquet:",snappy,dict"`
	Children  []VariableID `parquet:""`
	Truncated bool         `parquet:""`
}

type StackFrame struct {
	FileName               string       `parquet:",snappy,dict"`
	MethodName             string       `parquet:",snappy,dict"`
	LineNumber             int32        `parquet:",delta"`
	ClassName              *string      `parquet:",snappy,optional,dict"`
	IsAsync                bool         `parquet:""`
	ColumnNumber           *int32       `parquet:",snappy,optional"`
	TranspiledFileName     *string      `parquet:",snappy,optional,dict"`
	TranspiledLineNumber   *int32       `parquet:",snappy,optional"`
	TranspiledColumnNumber *int32       `parquet:",snappy,optional"`
	Variables              []VariableID `parquet:""`
	AppFrame               bool         `parquet:""`
}

type WatchResult struct {
	Expression  string      `parquet:",snappy"`
	GoodResult  *VariableID `parquet:""`
	ErrorResult *string     `parquet:",snappy"`
}

type Snapshot struct {
	ID            []byte              `parquet:""`
	IDText        string              `parquet:",snappy"`
	Tracepoint    TracePointConfig    `parquet:"tp"`
	VarLookup     map[string]Variable `parquet:""`
	Ts            int64               `parquet:",delta"`
	Frames        []StackFrame        `parquet:""`
	Watches       []WatchResult       `parquet:""`
	Attributes    []Attribute         `parquet:"attr"`
	NanosDuration int64               `parquet:",delta"`
	Resource      Resource            `parquet:"rs"`
}

func attrToParquet(a *deep_common.KeyValue, p *Attribute) {
	p.Key = a.Key
	p.Value = nil
	p.ValueArray = ""
	p.ValueBool = nil
	p.ValueDouble = nil
	p.ValueInt = nil
	p.ValueKVList = ""

	switch v := a.GetValue().Value.(type) {
	case *deep_common.AnyValue_StringValue:
		p.Value = &v.StringValue
	case *deep_common.AnyValue_IntValue:
		p.ValueInt = &v.IntValue
	case *deep_common.AnyValue_DoubleValue:
		p.ValueDouble = &v.DoubleValue
	case *deep_common.AnyValue_BoolValue:
		p.ValueBool = &v.BoolValue
	case *deep_common.AnyValue_ArrayValue:
		jsonBytes := &bytes.Buffer{}
		_ = jsonMarshaler.Marshal(jsonBytes, a.Value) // deliberately marshalling a.Value because of AnyValue logic
		p.ValueArray = jsonBytes.String()
	case *deep_common.AnyValue_KvlistValue:
		jsonBytes := &bytes.Buffer{}
		_ = jsonMarshaler.Marshal(jsonBytes, a.Value) // deliberately marshalling a.Value because of AnyValue logic
		p.ValueKVList = jsonBytes.String()
	}
}

func snapshotToParquet(id common.ID, snapshot *deep_tp.Snapshot, sp *Snapshot) *Snapshot {
	if sp == nil {
		sp = &Snapshot{}
	}

	sp.ID = util.PadTraceIDTo16Bytes(id)
	sp.IDText = util.TraceIDToHexString(id)

	sp.Tracepoint = convertTracepoint(snapshot.Tracepoint)
	sp.VarLookup = convertLookup(snapshot.VarLookup)
	sp.Ts = snapshot.Ts

	sp.Frames = convertFrames(snapshot.Frames)
	sp.Watches = convertWatches(snapshot.Watches)

	sp.Attributes = convertAttributes(snapshot.Attributes)

	sp.Resource.ServiceName = ""
	sp.Resource.Cluster = nil
	sp.Resource.Namespace = nil
	sp.Resource.Pod = nil
	sp.Resource.Container = nil
	sp.Resource.K8sClusterName = nil
	sp.Resource.K8sNamespaceName = nil
	sp.Resource.K8sPodName = nil
	sp.Resource.K8sContainerName = nil

	if snapshot.Resource != nil {
		sp.Resource.Attrs = extendReuseSlice(len(snapshot.Resource), sp.Resource.Attrs)
		attrCount := 0
		for _, a := range snapshot.Resource {
			strVal, ok := a.Value.Value.(*deep_common.AnyValue_StringValue)
			special := ok
			if ok {
				switch a.Key {
				case LabelServiceName:
					sp.Resource.ServiceName = strVal.StringValue
				case LabelCluster:
					sp.Resource.Cluster = &strVal.StringValue
				case LabelNamespace:
					sp.Resource.Namespace = &strVal.StringValue
				case LabelPod:
					sp.Resource.Pod = &strVal.StringValue
				case LabelContainer:
					sp.Resource.Container = &strVal.StringValue

				case LabelK8sClusterName:
					sp.Resource.K8sClusterName = &strVal.StringValue
				case LabelK8sNamespaceName:
					sp.Resource.K8sNamespaceName = &strVal.StringValue
				case LabelK8sPodName:
					sp.Resource.K8sPodName = &strVal.StringValue
				case LabelK8sContainerName:
					sp.Resource.K8sContainerName = &strVal.StringValue
				default:
					special = false
				}
			}

			if !special {
				// Other attributes put in generic columns
				attrToParquet(a, &sp.Resource.Attrs[attrCount])
				attrCount++
			}
		}
		sp.Resource.Attrs = sp.Resource.Attrs[:attrCount]
	}

	return sp
}

func convertAttributes(attributes []*deep_common.KeyValue) []Attribute {
	var parAttributes = make([]Attribute, len(attributes))
	for i, attribute := range attributes {
		attrToParquet(attribute, &parAttributes[i])
	}
	return parAttributes
}

func convertWatches(watches []*deep_tp.WatchResult) []WatchResult {
	var parWatches = make([]WatchResult, len(watches))
	for i, watch := range watches {
		parWatches[i] = convertWatch(watch)
	}
	return parWatches
}

func convertWatch(watch *deep_tp.WatchResult) WatchResult {
	variableId := convertVariableId(watch.GetGoodResult())
	result := watch.GetErrorResult()
	return WatchResult{
		Expression:  watch.Expression,
		GoodResult:  &variableId,
		ErrorResult: &result,
	}
}

func convertFrames(frames []*deep_tp.StackFrame) []StackFrame {
	var parFrames = make([]StackFrame, len(frames))
	for i, frame := range frames {
		parFrames[i] = convertFrame(frame)
	}
	return parFrames
}

func convertFrame(frame *deep_tp.StackFrame) StackFrame {
	return StackFrame{
		FileName:               frame.FileName,
		MethodName:             frame.MethodName,
		LineNumber:             frame.LineNumber,
		ClassName:              frame.ClassName,
		IsAsync:                *frame.IsAsync,
		ColumnNumber:           frame.ColumnNumber,
		TranspiledFileName:     frame.TranspiledFileName,
		TranspiledLineNumber:   frame.TranspiledLineNumber,
		TranspiledColumnNumber: frame.TranspiledColumnNumber,
		Variables:              convertChildren(frame.Variables),
		AppFrame:               *frame.AppFrame,
	}
}

func convertLookup(lookup map[string]*deep_tp.Variable) map[string]Variable {
	var parLookup = make(map[string]Variable, len(lookup))
	for varId, variable := range lookup {
		parLookup[varId] = convertVariable(variable)
	}
	return parLookup
}

func convertVariable(variable *deep_tp.Variable) Variable {
	return Variable{
		Type:      variable.Type,
		Value:     variable.Value,
		Hash:      variable.Hash,
		Children:  convertChildren(variable.Children),
		Truncated: *variable.Truncated,
	}
}

func convertChildren(children []*deep_tp.VariableID) []VariableID {
	var parChildren = make([]VariableID, len(children))
	for i, child := range children {
		parChildren[i] = convertVariableId(child)
	}
	return parChildren
}

func convertVariableId(child *deep_tp.VariableID) VariableID {
	return VariableID{
		ID:        child.ID,
		Name:      child.Name,
		Modifiers: child.Modifiers,
	}
}

func convertTracepoint(tracepoint *deep_tp.TracePointConfig) TracePointConfig {
	return TracePointConfig{
		ID:      tracepoint.ID,
		Path:    tracepoint.Path,
		LineNo:  tracepoint.LineNo,
		Args:    tracepoint.Args,
		Watches: tracepoint.Watches,
	}
}

func extendReuseSlice[T any](sz int, in []T) []T {
	if cap(in) >= sz {
		// slice is large enough
		return in[:sz]
	}

	// append until we're large enough
	in = in[:cap(in)]
	return append(in, make([]T, sz-len(in))...)
}

func parquetToDeepSnapshot(snap *Snapshot) *deep_tp.Snapshot {
	return &deep_tp.Snapshot{
		ID: snap.ID,
		Tracepoint: &deep_tp.TracePointConfig{
			ID:      snap.Tracepoint.ID,
			Path:    snap.Tracepoint.Path,
			LineNo:  snap.Tracepoint.LineNo,
			Args:    snap.Tracepoint.Args,
			Watches: snap.Tracepoint.Watches,
		},
		VarLookup:     parquetConvertVariables(snap.VarLookup),
		Ts:            snap.Ts,
		Frames:        parquetConvertFrames(snap.Frames),
		Watches:       parquetConvertWatches(snap.Watches),
		Attributes:    parquetConvertAttributes(snap.Attributes),
		NanosDuration: snap.NanosDuration,
		Resource:      parquetConvertResource(snap.Resource),
	}
}

func parquetConvertResource(resource Resource) []*deep_common.KeyValue {
	protoAttrs := parquetConvertAttributes(resource.Attrs)

	for _, attr := range []struct {
		Key   string
		Value *string
	}{
		{Key: LabelServiceName, Value: &resource.ServiceName},
		{Key: LabelCluster, Value: resource.Cluster},
		{Key: LabelNamespace, Value: resource.Namespace},
		{Key: LabelPod, Value: resource.Pod},
		{Key: LabelContainer, Value: resource.Container},
		{Key: LabelK8sClusterName, Value: resource.K8sClusterName},
		{Key: LabelK8sNamespaceName, Value: resource.K8sNamespaceName},
		{Key: LabelK8sPodName, Value: resource.K8sPodName},
		{Key: LabelK8sContainerName, Value: resource.K8sContainerName},
	} {
		if attr.Value != nil {
			protoAttrs = append(protoAttrs, &deep_common.KeyValue{
				Key: attr.Key,
				Value: &deep_common.AnyValue{
					Value: &deep_common.AnyValue_StringValue{
						StringValue: *attr.Value,
					},
				},
			})
		}
	}

	return protoAttrs
}

func parquetConvertAttributes(parquetAttrs []Attribute) []*deep_common.KeyValue {
	var protoAttrs []*deep_common.KeyValue

	for _, attr := range parquetAttrs {
		protoVal := &deep_common.AnyValue{}

		if attr.Value != nil {
			protoVal.Value = &deep_common.AnyValue_StringValue{
				StringValue: *attr.Value,
			}
		} else if attr.ValueInt != nil {
			protoVal.Value = &deep_common.AnyValue_IntValue{
				IntValue: *attr.ValueInt,
			}
		} else if attr.ValueDouble != nil {
			protoVal.Value = &deep_common.AnyValue_DoubleValue{
				DoubleValue: *attr.ValueDouble,
			}
		} else if attr.ValueBool != nil {
			protoVal.Value = &deep_common.AnyValue_BoolValue{
				BoolValue: *attr.ValueBool,
			}
		} else if attr.ValueArray != "" {
			_ = jsonpb.Unmarshal(bytes.NewBufferString(attr.ValueArray), protoVal)
		} else if attr.ValueKVList != "" {
			_ = jsonpb.Unmarshal(bytes.NewBufferString(attr.ValueKVList), protoVal)
		}

		protoAttrs = append(protoAttrs, &deep_common.KeyValue{
			Key:   attr.Key,
			Value: protoVal,
		})
	}

	return protoAttrs
}

func parquetConvertWatches(watches []WatchResult) []*deep_tp.WatchResult {
	var varWatches = make([]*deep_tp.WatchResult, len(watches))
	for i, watch := range watches {
		varWatches[i] = parquetConvertWatchResult(watch)
	}
	return varWatches
}

func parquetConvertWatchResult(watch WatchResult) *deep_tp.WatchResult {
	if watch.GoodResult != nil {
		return &deep_tp.WatchResult{
			Expression: watch.Expression,
			Result:     &deep_tp.WatchResult_GoodResult{GoodResult: parquetConvertVariableID(*watch.GoodResult)},
		}
	} else {
		return &deep_tp.WatchResult{
			Expression: watch.Expression,
			Result:     &deep_tp.WatchResult_ErrorResult{ErrorResult: *watch.ErrorResult},
		}
	}
}

func parquetConvertFrames(frames []StackFrame) []*deep_tp.StackFrame {
	var varFrames = make([]*deep_tp.StackFrame, len(frames))
	for i, frame := range frames {
		varFrames[i] = parquetConvertFrame(frame)
	}
	return varFrames
}

func parquetConvertFrame(frame StackFrame) *deep_tp.StackFrame {
	return &deep_tp.StackFrame{
		FileName:               frame.FileName,
		MethodName:             frame.MethodName,
		LineNumber:             frame.LineNumber,
		ClassName:              frame.ClassName,
		IsAsync:                &frame.IsAsync,
		ColumnNumber:           frame.ColumnNumber,
		TranspiledFileName:     frame.TranspiledFileName,
		TranspiledLineNumber:   frame.TranspiledLineNumber,
		TranspiledColumnNumber: frame.TranspiledColumnNumber,
		Variables:              parquetConvertChildren(frame.Variables),
		AppFrame:               &frame.AppFrame,
	}
}

func parquetConvertVariables(lookup map[string]Variable) map[string]*deep_tp.Variable {
	var varLookup = make(map[string]*deep_tp.Variable, len(lookup))
	for varId, variable := range lookup {
		varLookup[varId] = parquetConvertVariable(variable)
	}
	return varLookup
}

func parquetConvertVariable(variable Variable) *deep_tp.Variable {
	return &deep_tp.Variable{
		Type:      variable.Type,
		Value:     variable.Value,
		Hash:      variable.Hash,
		Children:  parquetConvertChildren(variable.Children),
		Truncated: &variable.Truncated,
	}
}

func parquetConvertChildren(children []VariableID) []*deep_tp.VariableID {
	var varChildren = make([]*deep_tp.VariableID, len(children))
	for i, child := range children {
		varChildren[i] = parquetConvertVariableID(child)
	}
	return varChildren
}

func parquetConvertVariableID(child VariableID) *deep_tp.VariableID {
	return &deep_tp.VariableID{
		ID:        child.ID,
		Name:      child.Name,
		Modifiers: child.Modifiers,
	}
}
