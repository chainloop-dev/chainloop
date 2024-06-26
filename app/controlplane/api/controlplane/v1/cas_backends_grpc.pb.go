//
// Copyright 2024 The Chainloop Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: controlplane/v1/cas_backends.proto

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

const (
	CASBackendService_List_FullMethodName   = "/controlplane.v1.CASBackendService/List"
	CASBackendService_Create_FullMethodName = "/controlplane.v1.CASBackendService/Create"
	CASBackendService_Update_FullMethodName = "/controlplane.v1.CASBackendService/Update"
	CASBackendService_Delete_FullMethodName = "/controlplane.v1.CASBackendService/Delete"
)

// CASBackendServiceClient is the client API for CASBackendService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type CASBackendServiceClient interface {
	List(ctx context.Context, in *CASBackendServiceListRequest, opts ...grpc.CallOption) (*CASBackendServiceListResponse, error)
	Create(ctx context.Context, in *CASBackendServiceCreateRequest, opts ...grpc.CallOption) (*CASBackendServiceCreateResponse, error)
	Update(ctx context.Context, in *CASBackendServiceUpdateRequest, opts ...grpc.CallOption) (*CASBackendServiceUpdateResponse, error)
	Delete(ctx context.Context, in *CASBackendServiceDeleteRequest, opts ...grpc.CallOption) (*CASBackendServiceDeleteResponse, error)
}

type cASBackendServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewCASBackendServiceClient(cc grpc.ClientConnInterface) CASBackendServiceClient {
	return &cASBackendServiceClient{cc}
}

func (c *cASBackendServiceClient) List(ctx context.Context, in *CASBackendServiceListRequest, opts ...grpc.CallOption) (*CASBackendServiceListResponse, error) {
	out := new(CASBackendServiceListResponse)
	err := c.cc.Invoke(ctx, CASBackendService_List_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cASBackendServiceClient) Create(ctx context.Context, in *CASBackendServiceCreateRequest, opts ...grpc.CallOption) (*CASBackendServiceCreateResponse, error) {
	out := new(CASBackendServiceCreateResponse)
	err := c.cc.Invoke(ctx, CASBackendService_Create_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cASBackendServiceClient) Update(ctx context.Context, in *CASBackendServiceUpdateRequest, opts ...grpc.CallOption) (*CASBackendServiceUpdateResponse, error) {
	out := new(CASBackendServiceUpdateResponse)
	err := c.cc.Invoke(ctx, CASBackendService_Update_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cASBackendServiceClient) Delete(ctx context.Context, in *CASBackendServiceDeleteRequest, opts ...grpc.CallOption) (*CASBackendServiceDeleteResponse, error) {
	out := new(CASBackendServiceDeleteResponse)
	err := c.cc.Invoke(ctx, CASBackendService_Delete_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CASBackendServiceServer is the server API for CASBackendService service.
// All implementations must embed UnimplementedCASBackendServiceServer
// for forward compatibility
type CASBackendServiceServer interface {
	List(context.Context, *CASBackendServiceListRequest) (*CASBackendServiceListResponse, error)
	Create(context.Context, *CASBackendServiceCreateRequest) (*CASBackendServiceCreateResponse, error)
	Update(context.Context, *CASBackendServiceUpdateRequest) (*CASBackendServiceUpdateResponse, error)
	Delete(context.Context, *CASBackendServiceDeleteRequest) (*CASBackendServiceDeleteResponse, error)
	mustEmbedUnimplementedCASBackendServiceServer()
}

// UnimplementedCASBackendServiceServer must be embedded to have forward compatible implementations.
type UnimplementedCASBackendServiceServer struct {
}

func (UnimplementedCASBackendServiceServer) List(context.Context, *CASBackendServiceListRequest) (*CASBackendServiceListResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedCASBackendServiceServer) Create(context.Context, *CASBackendServiceCreateRequest) (*CASBackendServiceCreateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedCASBackendServiceServer) Update(context.Context, *CASBackendServiceUpdateRequest) (*CASBackendServiceUpdateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedCASBackendServiceServer) Delete(context.Context, *CASBackendServiceDeleteRequest) (*CASBackendServiceDeleteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedCASBackendServiceServer) mustEmbedUnimplementedCASBackendServiceServer() {}

// UnsafeCASBackendServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to CASBackendServiceServer will
// result in compilation errors.
type UnsafeCASBackendServiceServer interface {
	mustEmbedUnimplementedCASBackendServiceServer()
}

func RegisterCASBackendServiceServer(s grpc.ServiceRegistrar, srv CASBackendServiceServer) {
	s.RegisterService(&CASBackendService_ServiceDesc, srv)
}

func _CASBackendService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CASBackendServiceListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CASBackendServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CASBackendService_List_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CASBackendServiceServer).List(ctx, req.(*CASBackendServiceListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CASBackendService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CASBackendServiceCreateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CASBackendServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CASBackendService_Create_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CASBackendServiceServer).Create(ctx, req.(*CASBackendServiceCreateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CASBackendService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CASBackendServiceUpdateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CASBackendServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CASBackendService_Update_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CASBackendServiceServer).Update(ctx, req.(*CASBackendServiceUpdateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CASBackendService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CASBackendServiceDeleteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CASBackendServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CASBackendService_Delete_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CASBackendServiceServer).Delete(ctx, req.(*CASBackendServiceDeleteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// CASBackendService_ServiceDesc is the grpc.ServiceDesc for CASBackendService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var CASBackendService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "controlplane.v1.CASBackendService",
	HandlerType: (*CASBackendServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "List",
			Handler:    _CASBackendService_List_Handler,
		},
		{
			MethodName: "Create",
			Handler:    _CASBackendService_Create_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _CASBackendService_Update_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _CASBackendService_Delete_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "controlplane/v1/cas_backends.proto",
}
