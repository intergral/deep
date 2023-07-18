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

package vparquet

import (
	"bytes"
	deepCommon "github.com/intergral/deep/pkg/deeppb/common/v1"
	deepTP "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"

	"github.com/golang/protobuf/jsonpb" //nolint:all //deprecated
	"github.com/intergral/deep/pkg/deepdb/encoding/common"
	"github.com/intergral/deep/pkg/util"
)

// Label names for conversion b/n Proto <> Parquet
const (
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
	DefinitionLevelSnapshot      = 0
	DefinitionLevelResourceAttrs = 1
	DefinitionLevelSnapshotAttrs = 1

	FieldResourceAttrKey       = "rs.Attrs.Key"
	FieldResourceAttrVal       = "rs.Attrs.Value"
	FieldResourceAttrValInt    = "rs.Attrs.ValueInt"
	FieldResourceAttrValDouble = "rs.Attrs.ValueDouble"
	FieldResourceAttrValBool   = "rs.Attrs.ValueBool"

	FieldAttrKey       = "attr.Key"
	FieldAttrVal       = "attr.Value"
	FieldAttrValInt    = "attr.ValueInt"
	FieldAttrValDouble = "attr.ValueDouble"
	FieldAttrValBool   = "attr.ValueBool"
)

var (
	jsonMarshaller = new(jsonpb.Marshaler)

	labelMappings = map[string]string{
		LabelServiceName:      "rs.ServiceName",
		LabelCluster:          "rs.Cluster",
		LabelNamespace:        "rs.Namespace",
		LabelPod:              "rs.Pod",
		LabelContainer:        "rs.Container",
		LabelK8sClusterName:   "rs.K8sClusterName",
		LabelK8sNamespaceName: "rs.K8sNamespaceName",
		LabelK8sPodName:       "rs.K8sPodName",
		LabelK8sContainerName: "rs.K8sContainerName",
	}
)

type Attribute struct {
	Key string `parquet:",snappy,dict"`

	// This is a bad design that leads to millions of null values. How can we fix this?
	Value       *string  `parquet:",dict,snappy,optional"  json:",omitempty"`
	ValueInt    *int64   `parquet:",snappy,optional"  json:",omitempty"`
	ValueDouble *float64 `parquet:",snappy,optional"  json:",omitempty"`
	ValueBool   *bool    `parquet:",snappy,optional"  json:",omitempty"`
	ValueKVList string   `parquet:",snappy,optional"  json:",omitempty"`
	ValueArray  string   `parquet:",snappy,optional"  json:",omitempty"`
}

type Resource struct {
	Attrs []Attribute

	// Known attributes
	ServiceName      string  `parquet:",snappy,dict" json:",omitempty"`
	Cluster          *string `parquet:",snappy,optional,dict" json:",omitempty"`
	Namespace        *string `parquet:",snappy,optional,dict" json:",omitempty"`
	Pod              *string `parquet:",snappy,optional,dict" json:",omitempty"`
	Container        *string `parquet:",snappy,optional,dict" json:",omitempty"`
	K8sClusterName   *string `parquet:",snappy,optional,dict" json:",omitempty"`
	K8sNamespaceName *string `parquet:",snappy,optional,dict" json:",omitempty"`
	K8sPodName       *string `parquet:",snappy,optional,dict" json:",omitempty"`
	K8sContainerName *string `parquet:",snappy,optional,dict" json:",omitempty"`

	Test string `parquet:",snappy,dict,optional" json:",omitempty"` // Always empty for testing
}

type TracePointConfig struct {
	ID         string            `parquet:",snappy,dict"`
	Path       string            `parquet:",snappy,dict"`
	LineNumber uint32            `parquet:",delta"`
	Args       map[string]string `parquet:""`
	Watches    []string          `parquet:""`
}

type VariableID struct {
	ID           string   `parquet:",snappy,dict"`
	Name         string   `parquet:",snappy,dict"`
	OriginalName *string  `parquet:",snappy,optional"`
	Modifiers    []string `parquet:""`
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
	LineNumber             uint32       `parquet:",delta"`
	ClassName              *string      `parquet:",snappy,optional,dict"`
	IsAsync                bool         `parquet:""`
	ColumnNumber           *uint32      `parquet:",snappy,optional"`
	TranspiledFileName     *string      `parquet:",snappy,optional,dict"`
	TranspiledLineNumber   *uint32      `parquet:",snappy,optional"`
	TranspiledColumnNumber *uint32      `parquet:",snappy,optional"`
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
	TsNanos       uint64              `parquet:",delta"`
	Frames        []StackFrame        `parquet:""`
	Watches       []WatchResult       `parquet:""`
	Attributes    []Attribute         `parquet:"attr"`
	DurationNanos uint64              `parquet:",delta"`
	Resource      Resource            `parquet:"rs"`
}

func attrToParquet(a *deepCommon.KeyValue, p *Attribute) {
	p.Key = a.Key
	p.Value = nil
	p.ValueArray = ""
	p.ValueBool = nil
	p.ValueDouble = nil
	p.ValueInt = nil
	p.ValueKVList = ""

	switch v := a.GetValue().Value.(type) {
	case *deepCommon.AnyValue_StringValue:
		p.Value = &v.StringValue
	case *deepCommon.AnyValue_IntValue:
		p.ValueInt = &v.IntValue
	case *deepCommon.AnyValue_DoubleValue:
		p.ValueDouble = &v.DoubleValue
	case *deepCommon.AnyValue_BoolValue:
		p.ValueBool = &v.BoolValue
	case *deepCommon.AnyValue_ArrayValue:
		jsonBytes := &bytes.Buffer{}
		_ = jsonMarshaller.Marshal(jsonBytes, a.Value) // deliberately marshalling a.Value because of AnyValue logic
		p.ValueArray = jsonBytes.String()
	case *deepCommon.AnyValue_KvlistValue:
		jsonBytes := &bytes.Buffer{}
		_ = jsonMarshaller.Marshal(jsonBytes, a.Value) // deliberately marshalling a.Value because of AnyValue logic
		p.ValueKVList = jsonBytes.String()
	}
}

func snapshotToParquet(id common.ID, snapshot *deepTP.Snapshot, sp *Snapshot) *Snapshot {
	if sp == nil {
		sp = &Snapshot{}
	}

	sp.ID = util.PadSnapshotIDTo16Bytes(id)
	sp.IDText = util.SnapshotIDToHexString(id)

	sp.Tracepoint = convertTracepoint(snapshot.Tracepoint)
	sp.VarLookup = convertLookup(snapshot.VarLookup)
	sp.TsNanos = snapshot.TsNanos
	sp.DurationNanos = snapshot.DurationNanos

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
			strVal, ok := a.Value.Value.(*deepCommon.AnyValue_StringValue)
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

func convertAttributes(attributes []*deepCommon.KeyValue) []Attribute {
	var parAttributes = make([]Attribute, len(attributes))
	for i, attribute := range attributes {
		attrToParquet(attribute, &parAttributes[i])
	}
	return parAttributes
}

func convertWatches(watches []*deepTP.WatchResult) []WatchResult {
	var parWatches = make([]WatchResult, len(watches))
	for i, watch := range watches {
		parWatches[i] = convertWatch(watch)
	}
	return parWatches
}

func convertWatch(watch *deepTP.WatchResult) WatchResult {
	goodResult := watch.GetGoodResult()
	if goodResult != nil {
		variableId := convertVariableId(goodResult)
		return WatchResult{
			Expression: watch.Expression,
			GoodResult: &variableId,
		}
	}

	result := watch.GetErrorResult()
	return WatchResult{
		Expression:  watch.Expression,
		ErrorResult: &result,
	}
}

func convertFrames(frames []*deepTP.StackFrame) []StackFrame {
	var parFrames = make([]StackFrame, len(frames))
	for i, frame := range frames {
		parFrames[i] = convertFrame(frame)
	}
	return parFrames
}

func convertFrame(frame *deepTP.StackFrame) StackFrame {
	var isAsync = false
	if frame.IsAsync != nil {
		isAsync = *frame.IsAsync
	}
	var appFrame = false
	if frame.AppFrame != nil {
		appFrame = *frame.AppFrame
	}
	return StackFrame{
		FileName:               frame.FileName,
		MethodName:             frame.MethodName,
		LineNumber:             frame.LineNumber,
		ClassName:              frame.ClassName,
		IsAsync:                isAsync,
		ColumnNumber:           frame.ColumnNumber,
		TranspiledFileName:     frame.TranspiledFileName,
		TranspiledLineNumber:   frame.TranspiledLineNumber,
		TranspiledColumnNumber: frame.TranspiledColumnNumber,
		Variables:              convertChildren(frame.Variables),
		AppFrame:               appFrame,
	}
}

func convertLookup(lookup map[string]*deepTP.Variable) map[string]Variable {
	var parLookup = make(map[string]Variable, len(lookup))
	for varId, variable := range lookup {
		parLookup[varId] = convertVariable(variable)
	}
	return parLookup
}

func convertVariable(variable *deepTP.Variable) Variable {
	var truncated = false
	if variable.Truncated != nil {
		truncated = *variable.Truncated
	}
	return Variable{
		Type:      variable.Type,
		Value:     variable.Value,
		Hash:      variable.Hash,
		Children:  convertChildren(variable.Children),
		Truncated: truncated,
	}
}

func convertChildren(children []*deepTP.VariableID) []VariableID {
	var parChildren = make([]VariableID, len(children))
	for i, child := range children {
		parChildren[i] = convertVariableId(child)
	}
	return parChildren
}

func convertVariableId(child *deepTP.VariableID) VariableID {
	return VariableID{
		ID:           child.ID,
		Name:         child.Name,
		OriginalName: child.OriginalName,
		Modifiers:    child.Modifiers,
	}
}

func convertTracepoint(tracepoint *deepTP.TracePointConfig) TracePointConfig {
	return TracePointConfig{
		ID:         tracepoint.ID,
		Path:       tracepoint.Path,
		LineNumber: tracepoint.LineNumber,
		Args:       tracepoint.Args,
		Watches:    tracepoint.Watches,
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

func parquetToDeepSnapshot(snap *Snapshot) *deepTP.Snapshot {
	return &deepTP.Snapshot{
		ID: snap.ID,
		Tracepoint: &deepTP.TracePointConfig{
			ID:         snap.Tracepoint.ID,
			Path:       snap.Tracepoint.Path,
			LineNumber: snap.Tracepoint.LineNumber,
			Args:       snap.Tracepoint.Args,
			Watches:    snap.Tracepoint.Watches,
		},
		VarLookup:     parquetConvertVariables(snap.VarLookup),
		TsNanos:       snap.TsNanos,
		Frames:        parquetConvertFrames(snap.Frames),
		Watches:       parquetConvertWatches(snap.Watches),
		Attributes:    parquetConvertAttributes(snap.Attributes),
		DurationNanos: snap.DurationNanos,
		Resource:      parquetConvertResource(snap.Resource),
	}
}

func parquetConvertResource(resource Resource) []*deepCommon.KeyValue {
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
			protoAttrs = append(protoAttrs, &deepCommon.KeyValue{
				Key: attr.Key,
				Value: &deepCommon.AnyValue{
					Value: &deepCommon.AnyValue_StringValue{
						StringValue: *attr.Value,
					},
				},
			})
		}
	}

	return protoAttrs
}

func parquetConvertAttributes(parquetAttrs []Attribute) []*deepCommon.KeyValue {
	var protoAttrs []*deepCommon.KeyValue

	for _, attr := range parquetAttrs {
		protoVal := &deepCommon.AnyValue{}

		if attr.Value != nil {
			protoVal.Value = &deepCommon.AnyValue_StringValue{
				StringValue: *attr.Value,
			}
		} else if attr.ValueInt != nil {
			protoVal.Value = &deepCommon.AnyValue_IntValue{
				IntValue: *attr.ValueInt,
			}
		} else if attr.ValueDouble != nil {
			protoVal.Value = &deepCommon.AnyValue_DoubleValue{
				DoubleValue: *attr.ValueDouble,
			}
		} else if attr.ValueBool != nil {
			protoVal.Value = &deepCommon.AnyValue_BoolValue{
				BoolValue: *attr.ValueBool,
			}
		} else if attr.ValueArray != "" {
			_ = jsonpb.Unmarshal(bytes.NewBufferString(attr.ValueArray), protoVal)
		} else if attr.ValueKVList != "" {
			_ = jsonpb.Unmarshal(bytes.NewBufferString(attr.ValueKVList), protoVal)
		}

		protoAttrs = append(protoAttrs, &deepCommon.KeyValue{
			Key:   attr.Key,
			Value: protoVal,
		})
	}

	return protoAttrs
}

func parquetConvertWatches(watches []WatchResult) []*deepTP.WatchResult {
	var varWatches = make([]*deepTP.WatchResult, len(watches))
	for i, watch := range watches {
		varWatches[i] = parquetConvertWatchResult(watch)
	}
	return varWatches
}

func parquetConvertWatchResult(watch WatchResult) *deepTP.WatchResult {
	if watch.GoodResult != nil {
		return &deepTP.WatchResult{
			Expression: watch.Expression,
			Result:     &deepTP.WatchResult_GoodResult{GoodResult: parquetConvertVariableID(*watch.GoodResult)},
		}
	} else {
		return &deepTP.WatchResult{
			Expression: watch.Expression,
			Result:     &deepTP.WatchResult_ErrorResult{ErrorResult: *watch.ErrorResult},
		}
	}
}

func parquetConvertFrames(frames []StackFrame) []*deepTP.StackFrame {
	var varFrames = make([]*deepTP.StackFrame, len(frames))
	for i, frame := range frames {
		varFrames[i] = parquetConvertFrame(frame)
	}
	return varFrames
}

func parquetConvertFrame(frame StackFrame) *deepTP.StackFrame {
	trueBool := true
	var isAsync *bool = nil
	if frame.IsAsync {
		isAsync = &trueBool
	}
	var appFrame *bool = nil
	if frame.AppFrame {
		appFrame = &trueBool
	}
	return &deepTP.StackFrame{
		FileName:               frame.FileName,
		MethodName:             frame.MethodName,
		LineNumber:             frame.LineNumber,
		ClassName:              frame.ClassName,
		IsAsync:                isAsync,
		ColumnNumber:           frame.ColumnNumber,
		TranspiledFileName:     frame.TranspiledFileName,
		TranspiledLineNumber:   frame.TranspiledLineNumber,
		TranspiledColumnNumber: frame.TranspiledColumnNumber,
		Variables:              parquetConvertChildren(frame.Variables),
		AppFrame:               appFrame,
	}
}

func parquetConvertVariables(lookup map[string]Variable) map[string]*deepTP.Variable {
	var varLookup = make(map[string]*deepTP.Variable, len(lookup))
	for varId, variable := range lookup {
		varLookup[varId] = parquetConvertVariable(variable)
	}
	return varLookup
}

func parquetConvertVariable(variable Variable) *deepTP.Variable {
	var truncated *bool = nil
	if variable.Truncated {
		trueBool := true
		truncated = &trueBool
	}
	return &deepTP.Variable{
		Type:      variable.Type,
		Value:     variable.Value,
		Hash:      variable.Hash,
		Children:  parquetConvertChildren(variable.Children),
		Truncated: truncated,
	}
}

func parquetConvertChildren(children []VariableID) []*deepTP.VariableID {
	var varChildren = make([]*deepTP.VariableID, len(children))
	for i, child := range children {
		varChildren[i] = parquetConvertVariableID(child)
	}
	return varChildren
}

func parquetConvertVariableID(child VariableID) *deepTP.VariableID {
	return &deepTP.VariableID{
		ID:           child.ID,
		Name:         child.Name,
		OriginalName: child.OriginalName,
		Modifiers:    child.Modifiers,
	}
}
