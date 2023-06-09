// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.1.0
// - protoc             v3.17.3
// source: tracepoint/v1/tracepoint.proto

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// SnapshotServiceClient is the client API for SnapshotService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SnapshotServiceClient interface {
	Send(ctx context.Context, in *Snapshot, opts ...grpc.CallOption) (*SnapshotResponse, error)
}

type snapshotServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSnapshotServiceClient(cc grpc.ClientConnInterface) SnapshotServiceClient {
	return &snapshotServiceClient{cc}
}

func (c *snapshotServiceClient) Send(ctx context.Context, in *Snapshot, opts ...grpc.CallOption) (*SnapshotResponse, error) {
	out := new(SnapshotResponse)
	err := c.cc.Invoke(ctx, "/deeppb.tracepoint.v1.SnapshotService/send", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SnapshotServiceServer is the server API for SnapshotService service.
// All implementations must embed UnimplementedSnapshotServiceServer
// for forward compatibility
type SnapshotServiceServer interface {
	Send(context.Context, *Snapshot) (*SnapshotResponse, error)
	mustEmbedUnimplementedSnapshotServiceServer()
}

// UnimplementedSnapshotServiceServer must be embedded to have forward compatible implementations.
type UnimplementedSnapshotServiceServer struct {
}

func (UnimplementedSnapshotServiceServer) Send(context.Context, *Snapshot) (*SnapshotResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Send not implemented")
}
func (UnimplementedSnapshotServiceServer) mustEmbedUnimplementedSnapshotServiceServer() {}

// UnsafeSnapshotServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SnapshotServiceServer will
// result in compilation errors.
type UnsafeSnapshotServiceServer interface {
	mustEmbedUnimplementedSnapshotServiceServer()
}

func RegisterSnapshotServiceServer(s grpc.ServiceRegistrar, srv SnapshotServiceServer) {
	s.RegisterService(&SnapshotService_ServiceDesc, srv)
}

func _SnapshotService_Send_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Snapshot)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SnapshotServiceServer).Send(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/deeppb.tracepoint.v1.SnapshotService/send",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SnapshotServiceServer).Send(ctx, req.(*Snapshot))
	}
	return interceptor(ctx, in, info, handler)
}

// SnapshotService_ServiceDesc is the grpc.ServiceDesc for SnapshotService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SnapshotService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "deeppb.tracepoint.v1.SnapshotService",
	HandlerType: (*SnapshotServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "send",
			Handler:    _SnapshotService_Send_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "tracepoint/v1/tracepoint.proto",
}
