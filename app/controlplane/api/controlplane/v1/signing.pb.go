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

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        (unknown)
// source: controlplane/v1/signing.proto

package v1

import (
	_ "buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
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

type GenerateSigningCertRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CertificateSigningRequest []byte `protobuf:"bytes,1,opt,name=certificate_signing_request,json=certificateSigningRequest,proto3" json:"certificate_signing_request,omitempty"`
}

func (x *GenerateSigningCertRequest) Reset() {
	*x = GenerateSigningCertRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_controlplane_v1_signing_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GenerateSigningCertRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenerateSigningCertRequest) ProtoMessage() {}

func (x *GenerateSigningCertRequest) ProtoReflect() protoreflect.Message {
	mi := &file_controlplane_v1_signing_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenerateSigningCertRequest.ProtoReflect.Descriptor instead.
func (*GenerateSigningCertRequest) Descriptor() ([]byte, []int) {
	return file_controlplane_v1_signing_proto_rawDescGZIP(), []int{0}
}

func (x *GenerateSigningCertRequest) GetCertificateSigningRequest() []byte {
	if x != nil {
		return x.CertificateSigningRequest
	}
	return nil
}

type GenerateSigningCertResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Chain *CertificateChain `protobuf:"bytes,1,opt,name=chain,proto3" json:"chain,omitempty"`
}

func (x *GenerateSigningCertResponse) Reset() {
	*x = GenerateSigningCertResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_controlplane_v1_signing_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GenerateSigningCertResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenerateSigningCertResponse) ProtoMessage() {}

func (x *GenerateSigningCertResponse) ProtoReflect() protoreflect.Message {
	mi := &file_controlplane_v1_signing_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenerateSigningCertResponse.ProtoReflect.Descriptor instead.
func (*GenerateSigningCertResponse) Descriptor() ([]byte, []int) {
	return file_controlplane_v1_signing_proto_rawDescGZIP(), []int{1}
}

func (x *GenerateSigningCertResponse) GetChain() *CertificateChain {
	if x != nil {
		return x.Chain
	}
	return nil
}

type CertificateChain struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The PEM-encoded certificate chain, ordered from leaf to intermediate to root as applicable.
	Certificates []string `protobuf:"bytes,1,rep,name=certificates,proto3" json:"certificates,omitempty"`
}

func (x *CertificateChain) Reset() {
	*x = CertificateChain{}
	if protoimpl.UnsafeEnabled {
		mi := &file_controlplane_v1_signing_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CertificateChain) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CertificateChain) ProtoMessage() {}

func (x *CertificateChain) ProtoReflect() protoreflect.Message {
	mi := &file_controlplane_v1_signing_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CertificateChain.ProtoReflect.Descriptor instead.
func (*CertificateChain) Descriptor() ([]byte, []int) {
	return file_controlplane_v1_signing_proto_rawDescGZIP(), []int{2}
}

func (x *CertificateChain) GetCertificates() []string {
	if x != nil {
		return x.Certificates
	}
	return nil
}

type GetTrustedRootRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *GetTrustedRootRequest) Reset() {
	*x = GetTrustedRootRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_controlplane_v1_signing_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTrustedRootRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTrustedRootRequest) ProtoMessage() {}

func (x *GetTrustedRootRequest) ProtoReflect() protoreflect.Message {
	mi := &file_controlplane_v1_signing_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTrustedRootRequest.ProtoReflect.Descriptor instead.
func (*GetTrustedRootRequest) Descriptor() ([]byte, []int) {
	return file_controlplane_v1_signing_proto_rawDescGZIP(), []int{3}
}

type GetTrustedRootResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// map keyID (cert SubjectKeyIdentifier) to PEM encoded chains
	Keys map[string]*CertificateChain `protobuf:"bytes,1,rep,name=keys,proto3" json:"keys,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// timestamp authorities
	TimestampAuthorities map[string]*CertificateChain `protobuf:"bytes,2,rep,name=timestamp_authorities,json=timestampAuthorities,proto3" json:"timestamp_authorities,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *GetTrustedRootResponse) Reset() {
	*x = GetTrustedRootResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_controlplane_v1_signing_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTrustedRootResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTrustedRootResponse) ProtoMessage() {}

func (x *GetTrustedRootResponse) ProtoReflect() protoreflect.Message {
	mi := &file_controlplane_v1_signing_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTrustedRootResponse.ProtoReflect.Descriptor instead.
func (*GetTrustedRootResponse) Descriptor() ([]byte, []int) {
	return file_controlplane_v1_signing_proto_rawDescGZIP(), []int{4}
}

func (x *GetTrustedRootResponse) GetKeys() map[string]*CertificateChain {
	if x != nil {
		return x.Keys
	}
	return nil
}

func (x *GetTrustedRootResponse) GetTimestampAuthorities() map[string]*CertificateChain {
	if x != nil {
		return x.TimestampAuthorities
	}
	return nil
}

var File_controlplane_v1_signing_proto protoreflect.FileDescriptor

var file_controlplane_v1_signing_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2f, 0x76,
	0x31, 0x2f, 0x73, 0x69, 0x67, 0x6e, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x0f, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x76, 0x31,
	0x1a, 0x1b, 0x62, 0x75, 0x66, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2f, 0x76,
	0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x65, 0x0a,
	0x1a, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x53, 0x69, 0x67, 0x6e, 0x69, 0x6e, 0x67,
	0x43, 0x65, 0x72, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x47, 0x0a, 0x1b, 0x63,
	0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x5f, 0x73, 0x69, 0x67, 0x6e, 0x69,
	0x6e, 0x67, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c,
	0x42, 0x07, 0xba, 0x48, 0x04, 0x7a, 0x02, 0x10, 0x01, 0x52, 0x19, 0x63, 0x65, 0x72, 0x74, 0x69,
	0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x53, 0x69, 0x67, 0x6e, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x22, 0x56, 0x0a, 0x1b, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65,
	0x53, 0x69, 0x67, 0x6e, 0x69, 0x6e, 0x67, 0x43, 0x65, 0x72, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x12, 0x37, 0x0a, 0x05, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x21, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x70, 0x6c, 0x61, 0x6e,
	0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65,
	0x43, 0x68, 0x61, 0x69, 0x6e, 0x52, 0x05, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x22, 0x36, 0x0a, 0x10,
	0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x43, 0x68, 0x61, 0x69, 0x6e,
	0x12, 0x22, 0x0a, 0x0c, 0x63, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x73,
	0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0c, 0x63, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63,
	0x61, 0x74, 0x65, 0x73, 0x22, 0x17, 0x0a, 0x15, 0x47, 0x65, 0x74, 0x54, 0x72, 0x75, 0x73, 0x74,
	0x65, 0x64, 0x52, 0x6f, 0x6f, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x9f, 0x03,
	0x0a, 0x16, 0x47, 0x65, 0x74, 0x54, 0x72, 0x75, 0x73, 0x74, 0x65, 0x64, 0x52, 0x6f, 0x6f, 0x74,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x45, 0x0a, 0x04, 0x6b, 0x65, 0x79, 0x73,
	0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x31, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c,
	0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x75, 0x73,
	0x74, 0x65, 0x64, 0x52, 0x6f, 0x6f, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2e,
	0x4b, 0x65, 0x79, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x04, 0x6b, 0x65, 0x79, 0x73, 0x12,
	0x76, 0x0a, 0x15, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x5f, 0x61, 0x75, 0x74,
	0x68, 0x6f, 0x72, 0x69, 0x74, 0x69, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x41,
	0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x76, 0x31,
	0x2e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x75, 0x73, 0x74, 0x65, 0x64, 0x52, 0x6f, 0x6f, 0x74, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2e, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d,
	0x70, 0x41, 0x75, 0x74, 0x68, 0x6f, 0x72, 0x69, 0x74, 0x69, 0x65, 0x73, 0x45, 0x6e, 0x74, 0x72,
	0x79, 0x52, 0x14, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x41, 0x75, 0x74, 0x68,
	0x6f, 0x72, 0x69, 0x74, 0x69, 0x65, 0x73, 0x1a, 0x5a, 0x0a, 0x09, 0x4b, 0x65, 0x79, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x37, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x21, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x70,
	0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63,
	0x61, 0x74, 0x65, 0x43, 0x68, 0x61, 0x69, 0x6e, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a,
	0x02, 0x38, 0x01, 0x1a, 0x6a, 0x0a, 0x19, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70,
	0x41, 0x75, 0x74, 0x68, 0x6f, 0x72, 0x69, 0x74, 0x69, 0x65, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79,
	0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b,
	0x65, 0x79, 0x12, 0x37, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x21, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x70, 0x6c, 0x61, 0x6e, 0x65,
	0x2e, 0x76, 0x31, 0x2e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x43,
	0x68, 0x61, 0x69, 0x6e, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x32,
	0xe5, 0x01, 0x0a, 0x0e, 0x53, 0x69, 0x67, 0x6e, 0x69, 0x6e, 0x67, 0x53, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0x70, 0x0a, 0x13, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x53, 0x69,
	0x67, 0x6e, 0x69, 0x6e, 0x67, 0x43, 0x65, 0x72, 0x74, 0x12, 0x2b, 0x2e, 0x63, 0x6f, 0x6e, 0x74,
	0x72, 0x6f, 0x6c, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x6e, 0x65,
	0x72, 0x61, 0x74, 0x65, 0x53, 0x69, 0x67, 0x6e, 0x69, 0x6e, 0x67, 0x43, 0x65, 0x72, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2c, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c,
	0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74,
	0x65, 0x53, 0x69, 0x67, 0x6e, 0x69, 0x6e, 0x67, 0x43, 0x65, 0x72, 0x74, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x61, 0x0a, 0x0e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x75, 0x73, 0x74,
	0x65, 0x64, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x26, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c,
	0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x75, 0x73,
	0x74, 0x65, 0x64, 0x52, 0x6f, 0x6f, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x27,
	0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2e, 0x76, 0x31,
	0x2e, 0x47, 0x65, 0x74, 0x54, 0x72, 0x75, 0x73, 0x74, 0x65, 0x64, 0x52, 0x6f, 0x6f, 0x74, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x4c, 0x5a, 0x4a, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x6c, 0x6f, 0x6f, 0x70, 0x2d,
	0x64, 0x65, 0x76, 0x2f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x6c, 0x6f, 0x6f, 0x70, 0x2f, 0x61, 0x70,
	0x70, 0x2f, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2f, 0x61,
	0x70, 0x69, 0x2f, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x70, 0x6c, 0x61, 0x6e, 0x65, 0x2f,
	0x76, 0x31, 0x3b, 0x76, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_controlplane_v1_signing_proto_rawDescOnce sync.Once
	file_controlplane_v1_signing_proto_rawDescData = file_controlplane_v1_signing_proto_rawDesc
)

func file_controlplane_v1_signing_proto_rawDescGZIP() []byte {
	file_controlplane_v1_signing_proto_rawDescOnce.Do(func() {
		file_controlplane_v1_signing_proto_rawDescData = protoimpl.X.CompressGZIP(file_controlplane_v1_signing_proto_rawDescData)
	})
	return file_controlplane_v1_signing_proto_rawDescData
}

var file_controlplane_v1_signing_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_controlplane_v1_signing_proto_goTypes = []interface{}{
	(*GenerateSigningCertRequest)(nil),  // 0: controlplane.v1.GenerateSigningCertRequest
	(*GenerateSigningCertResponse)(nil), // 1: controlplane.v1.GenerateSigningCertResponse
	(*CertificateChain)(nil),            // 2: controlplane.v1.CertificateChain
	(*GetTrustedRootRequest)(nil),       // 3: controlplane.v1.GetTrustedRootRequest
	(*GetTrustedRootResponse)(nil),      // 4: controlplane.v1.GetTrustedRootResponse
	nil,                                 // 5: controlplane.v1.GetTrustedRootResponse.KeysEntry
	nil,                                 // 6: controlplane.v1.GetTrustedRootResponse.TimestampAuthoritiesEntry
}
var file_controlplane_v1_signing_proto_depIdxs = []int32{
	2, // 0: controlplane.v1.GenerateSigningCertResponse.chain:type_name -> controlplane.v1.CertificateChain
	5, // 1: controlplane.v1.GetTrustedRootResponse.keys:type_name -> controlplane.v1.GetTrustedRootResponse.KeysEntry
	6, // 2: controlplane.v1.GetTrustedRootResponse.timestamp_authorities:type_name -> controlplane.v1.GetTrustedRootResponse.TimestampAuthoritiesEntry
	2, // 3: controlplane.v1.GetTrustedRootResponse.KeysEntry.value:type_name -> controlplane.v1.CertificateChain
	2, // 4: controlplane.v1.GetTrustedRootResponse.TimestampAuthoritiesEntry.value:type_name -> controlplane.v1.CertificateChain
	0, // 5: controlplane.v1.SigningService.GenerateSigningCert:input_type -> controlplane.v1.GenerateSigningCertRequest
	3, // 6: controlplane.v1.SigningService.GetTrustedRoot:input_type -> controlplane.v1.GetTrustedRootRequest
	1, // 7: controlplane.v1.SigningService.GenerateSigningCert:output_type -> controlplane.v1.GenerateSigningCertResponse
	4, // 8: controlplane.v1.SigningService.GetTrustedRoot:output_type -> controlplane.v1.GetTrustedRootResponse
	7, // [7:9] is the sub-list for method output_type
	5, // [5:7] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_controlplane_v1_signing_proto_init() }
func file_controlplane_v1_signing_proto_init() {
	if File_controlplane_v1_signing_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_controlplane_v1_signing_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GenerateSigningCertRequest); i {
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
		file_controlplane_v1_signing_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GenerateSigningCertResponse); i {
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
		file_controlplane_v1_signing_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CertificateChain); i {
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
		file_controlplane_v1_signing_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetTrustedRootRequest); i {
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
		file_controlplane_v1_signing_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetTrustedRootResponse); i {
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
			RawDescriptor: file_controlplane_v1_signing_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_controlplane_v1_signing_proto_goTypes,
		DependencyIndexes: file_controlplane_v1_signing_proto_depIdxs,
		MessageInfos:      file_controlplane_v1_signing_proto_msgTypes,
	}.Build()
	File_controlplane_v1_signing_proto = out.File
	file_controlplane_v1_signing_proto_rawDesc = nil
	file_controlplane_v1_signing_proto_goTypes = nil
	file_controlplane_v1_signing_proto_depIdxs = nil
}
