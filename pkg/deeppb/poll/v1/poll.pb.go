//   Copyright (C) 2023  Intergral GmbH
//
//   This program is free software: you can redistribute it and/or modify
//   it under the terms of the GNU Affero General Public License as published by
//   the Free Software Foundation, either version 3 of the License, or
//   (at your option) any later version.
//
//   This program is distributed in the hope that it will be useful,
//   but WITHOUT ANY WARRANTY; without even the implied warranty of
//   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//   GNU Affero General Public License for more details.
//
//   You should have received a copy of the GNU Affero General Public License
//   along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.17.3
// source: poll/v1/poll.proto

package v1

import (
	v1 "github.com/intergral/deep/pkg/deeppb/resource/v1"
	v11 "github.com/intergral/deep/pkg/deeppb/tracepoint/v1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// ResponseType is used to indicate if the response from the server should trigger an update of the client config.
// If the client sends the same 'current_hash' as that which the server has for the client, then the server will response with 'NO_CHANGE'. This allows the client to not trigger any changes to the current tracepoint configs.
type ResponseType int32

const (
	ResponseType_NO_CHANGE ResponseType = 0 // This is sent when the 'current_hash' from the request is the same as the response. So the client should do nothing.
	ResponseType_UPDATE    ResponseType = 1 // This is sent when the client should process the response to update the config.
)

// Enum value maps for ResponseType.
var (
	ResponseType_name = map[int32]string{
		0: "NO_CHANGE",
		1: "UPDATE",
	}
	ResponseType_value = map[string]int32{
		"NO_CHANGE": 0,
		"UPDATE":    1,
	}
)

func (x ResponseType) Enum() *ResponseType {
	p := new(ResponseType)
	*p = x
	return p
}

func (x ResponseType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ResponseType) Descriptor() protoreflect.EnumDescriptor {
	return file_poll_v1_poll_proto_enumTypes[0].Descriptor()
}

func (ResponseType) Type() protoreflect.EnumType {
	return &file_poll_v1_poll_proto_enumTypes[0]
}

func (x ResponseType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ResponseType.Descriptor instead.
func (ResponseType) EnumDescriptor() ([]byte, []int) {
	return file_poll_v1_poll_proto_rawDescGZIP(), []int{0}
}

// PollRequest is sent by the client to check its current config with the server.
type PollRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TsNanos     uint64       `protobuf:"fixed64,1,opt,name=ts_nanos,json=tsNanos,proto3" json:"ts_nanos,omitempty"`           //The time (as epoch nanos) when the message was sent, acts as message ID.
	CurrentHash string       `protobuf:"bytes,2,opt,name=current_hash,json=currentHash,proto3" json:"current_hash,omitempty"` //This should the the hash that was last sent by the server, this is used to determine if the config has changed since the last poll. If no hash is available (e.g. first poll) then do not set this value.
	Resource    *v1.Resource `protobuf:"bytes,3,opt,name=resource,proto3" json:"resource,omitempty"`                          //The attributes that represent the client making the request. These attributes are used to filter the tracepoint response to just the tracepoints that are for the requesting client.
}

func (x *PollRequest) Reset() {
	*x = PollRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_poll_v1_poll_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PollRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PollRequest) ProtoMessage() {}

func (x *PollRequest) ProtoReflect() protoreflect.Message {
	mi := &file_poll_v1_poll_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PollRequest.ProtoReflect.Descriptor instead.
func (*PollRequest) Descriptor() ([]byte, []int) {
	return file_poll_v1_poll_proto_rawDescGZIP(), []int{0}
}

func (x *PollRequest) GetTsNanos() uint64 {
	if x != nil {
		return x.TsNanos
	}
	return 0
}

func (x *PollRequest) GetCurrentHash() string {
	if x != nil {
		return x.CurrentHash
	}
	return ""
}

func (x *PollRequest) GetResource() *v1.Resource {
	if x != nil {
		return x.Resource
	}
	return nil
}

// PollResponse is the response the server will send in response to a PollRequest.
type PollResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TsNanos      uint64                  `protobuf:"fixed64,1,opt,name=ts_nanos,json=tsNanos,proto3" json:"ts_nanos,omitempty"`                                                //The time (as epoch nanos) then he message was sent. This should match the 'ts_nanos' from the triggering PollRequest.
	CurrentHash  string                  `protobuf:"bytes,2,opt,name=current_hash,json=currentHash,proto3" json:"current_hash,omitempty"`                                      //This is the hash that the server has assigned this current config for this client. This should be sent with the next PollRequest.
	Response     []*v11.TracePointConfig `protobuf:"bytes,3,rep,name=response,proto3" json:"response,omitempty"`                                                               //This is the list of tracepoints that are currently configured for this client.
	ResponseType ResponseType            `protobuf:"varint,4,opt,name=response_type,json=responseType,proto3,enum=deeppb.poll.v1.ResponseType" json:"response_type,omitempty"` // This indicates if the config has changed or not. If 'NO_CHANGE' then 'response' should be null or empty.
}

func (x *PollResponse) Reset() {
	*x = PollResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_poll_v1_poll_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PollResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PollResponse) ProtoMessage() {}

func (x *PollResponse) ProtoReflect() protoreflect.Message {
	mi := &file_poll_v1_poll_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PollResponse.ProtoReflect.Descriptor instead.
func (*PollResponse) Descriptor() ([]byte, []int) {
	return file_poll_v1_poll_proto_rawDescGZIP(), []int{1}
}

func (x *PollResponse) GetTsNanos() uint64 {
	if x != nil {
		return x.TsNanos
	}
	return 0
}

func (x *PollResponse) GetCurrentHash() string {
	if x != nil {
		return x.CurrentHash
	}
	return ""
}

func (x *PollResponse) GetResponse() []*v11.TracePointConfig {
	if x != nil {
		return x.Response
	}
	return nil
}

func (x *PollResponse) GetResponseType() ResponseType {
	if x != nil {
		return x.ResponseType
	}
	return ResponseType_NO_CHANGE
}

var File_poll_v1_poll_proto protoreflect.FileDescriptor

var file_poll_v1_poll_proto_rawDesc = []byte{
	0x0a, 0x12, 0x70, 0x6f, 0x6c, 0x6c, 0x2f, 0x76, 0x31, 0x2f, 0x70, 0x6f, 0x6c, 0x6c, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0e, 0x64, 0x65, 0x65, 0x70, 0x70, 0x62, 0x2e, 0x70, 0x6f, 0x6c,
	0x6c, 0x2e, 0x76, 0x31, 0x1a, 0x1a, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2f, 0x76,
	0x31, 0x2f, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x1a, 0x1e, 0x74, 0x72, 0x61, 0x63, 0x65, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x2f, 0x76, 0x31, 0x2f,
	0x74, 0x72, 0x61, 0x63, 0x65, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x85, 0x01, 0x0a, 0x0b, 0x50, 0x6f, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x19, 0x0a, 0x08, 0x74, 0x73, 0x5f, 0x6e, 0x61, 0x6e, 0x6f, 0x73, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x06, 0x52, 0x07, 0x74, 0x73, 0x4e, 0x61, 0x6e, 0x6f, 0x73, 0x12, 0x21, 0x0a, 0x0c, 0x63,
	0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x68, 0x61, 0x73, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0b, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x48, 0x61, 0x73, 0x68, 0x12, 0x38,
	0x0a, 0x08, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1c, 0x2e, 0x64, 0x65, 0x65, 0x70, 0x70, 0x62, 0x2e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72,
	0x63, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x52, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x52, 0x08,
	0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x22, 0xd3, 0x01, 0x0a, 0x0c, 0x50, 0x6f, 0x6c,
	0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x74, 0x73, 0x5f,
	0x6e, 0x61, 0x6e, 0x6f, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x06, 0x52, 0x07, 0x74, 0x73, 0x4e,
	0x61, 0x6e, 0x6f, 0x73, 0x12, 0x21, 0x0a, 0x0c, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x5f,
	0x68, 0x61, 0x73, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x63, 0x75, 0x72, 0x72,
	0x65, 0x6e, 0x74, 0x48, 0x61, 0x73, 0x68, 0x12, 0x42, 0x0a, 0x08, 0x72, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x26, 0x2e, 0x64, 0x65, 0x65, 0x70,
	0x70, 0x62, 0x2e, 0x74, 0x72, 0x61, 0x63, 0x65, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x2e, 0x76, 0x31,
	0x2e, 0x54, 0x72, 0x61, 0x63, 0x65, 0x50, 0x6f, 0x69, 0x6e, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x52, 0x08, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x41, 0x0a, 0x0d, 0x72,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x0e, 0x32, 0x1c, 0x2e, 0x64, 0x65, 0x65, 0x70, 0x70, 0x62, 0x2e, 0x70, 0x6f, 0x6c, 0x6c,
	0x2e, 0x76, 0x31, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x54, 0x79, 0x70, 0x65,
	0x52, 0x0c, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x54, 0x79, 0x70, 0x65, 0x2a, 0x29,
	0x0a, 0x0c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0d,
	0x0a, 0x09, 0x4e, 0x4f, 0x5f, 0x43, 0x48, 0x41, 0x4e, 0x47, 0x45, 0x10, 0x00, 0x12, 0x0a, 0x0a,
	0x06, 0x55, 0x50, 0x44, 0x41, 0x54, 0x45, 0x10, 0x01, 0x32, 0x51, 0x0a, 0x0a, 0x50, 0x6f, 0x6c,
	0x6c, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x43, 0x0a, 0x04, 0x70, 0x6f, 0x6c, 0x6c, 0x12,
	0x1b, 0x2e, 0x64, 0x65, 0x65, 0x70, 0x70, 0x62, 0x2e, 0x70, 0x6f, 0x6c, 0x6c, 0x2e, 0x76, 0x31,
	0x2e, 0x50, 0x6f, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1c, 0x2e, 0x64,
	0x65, 0x65, 0x70, 0x70, 0x62, 0x2e, 0x70, 0x6f, 0x6c, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x50, 0x6f,
	0x6c, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x52, 0x0a, 0x20,
	0x63, 0x6f, 0x6d, 0x2e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x67, 0x72, 0x61, 0x6c, 0x2e, 0x64, 0x65,
	0x65, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x70, 0x6f, 0x6c, 0x6c, 0x2e, 0x76, 0x31,
	0x50, 0x01, 0x5a, 0x2c, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x67, 0x72, 0x61, 0x6c, 0x2f, 0x64, 0x65, 0x65, 0x70, 0x2f, 0x70, 0x6b,
	0x67, 0x2f, 0x64, 0x65, 0x65, 0x70, 0x70, 0x62, 0x2f, 0x70, 0x6f, 0x6c, 0x6c, 0x2f, 0x76, 0x31,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_poll_v1_poll_proto_rawDescOnce sync.Once
	file_poll_v1_poll_proto_rawDescData = file_poll_v1_poll_proto_rawDesc
)

func file_poll_v1_poll_proto_rawDescGZIP() []byte {
	file_poll_v1_poll_proto_rawDescOnce.Do(func() {
		file_poll_v1_poll_proto_rawDescData = protoimpl.X.CompressGZIP(file_poll_v1_poll_proto_rawDescData)
	})
	return file_poll_v1_poll_proto_rawDescData
}

var file_poll_v1_poll_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_poll_v1_poll_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_poll_v1_poll_proto_goTypes = []interface{}{
	(ResponseType)(0),            // 0: deeppb.poll.v1.ResponseType
	(*PollRequest)(nil),          // 1: deeppb.poll.v1.PollRequest
	(*PollResponse)(nil),         // 2: deeppb.poll.v1.PollResponse
	(*v1.Resource)(nil),          // 3: deeppb.resource.v1.Resource
	(*v11.TracePointConfig)(nil), // 4: deeppb.tracepoint.v1.TracePointConfig
}
var file_poll_v1_poll_proto_depIdxs = []int32{
	3, // 0: deeppb.poll.v1.PollRequest.resource:type_name -> deeppb.resource.v1.Resource
	4, // 1: deeppb.poll.v1.PollResponse.response:type_name -> deeppb.tracepoint.v1.TracePointConfig
	0, // 2: deeppb.poll.v1.PollResponse.response_type:type_name -> deeppb.poll.v1.ResponseType
	1, // 3: deeppb.poll.v1.PollConfig.poll:input_type -> deeppb.poll.v1.PollRequest
	2, // 4: deeppb.poll.v1.PollConfig.poll:output_type -> deeppb.poll.v1.PollResponse
	4, // [4:5] is the sub-list for method output_type
	3, // [3:4] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_poll_v1_poll_proto_init() }
func file_poll_v1_poll_proto_init() {
	if File_poll_v1_poll_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_poll_v1_poll_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PollRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_poll_v1_poll_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PollResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_poll_v1_poll_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_poll_v1_poll_proto_goTypes,
		DependencyIndexes: file_poll_v1_poll_proto_depIdxs,
		EnumInfos:         file_poll_v1_poll_proto_enumTypes,
		MessageInfos:      file_poll_v1_poll_proto_msgTypes,
	}.Build()
	File_poll_v1_poll_proto = out.File
	file_poll_v1_poll_proto_rawDesc = nil
	file_poll_v1_poll_proto_goTypes = nil
	file_poll_v1_poll_proto_depIdxs = nil
}
