//
// Copyright 2023 The Chainloop Authors.
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
// 	protoc-gen-go v1.28.1
// 	protoc        (unknown)
// source: conf.proto

package conf

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Bootstrap struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Server             *Server                  `protobuf:"bytes,1,opt,name=server,proto3" json:"server,omitempty"`
	Auth               *Auth                    `protobuf:"bytes,2,opt,name=auth,proto3" json:"auth,omitempty"`
	Observability      *Bootstrap_Observability `protobuf:"bytes,3,opt,name=observability,proto3" json:"observability,omitempty"`
	CredentialsService *Credentials             `protobuf:"bytes,4,opt,name=credentials_service,json=credentialsService,proto3" json:"credentials_service,omitempty"`
}

func (x *Bootstrap) Reset() {
	*x = Bootstrap{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Bootstrap) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Bootstrap) ProtoMessage() {}

func (x *Bootstrap) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Bootstrap.ProtoReflect.Descriptor instead.
func (*Bootstrap) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{0}
}

func (x *Bootstrap) GetServer() *Server {
	if x != nil {
		return x.Server
	}
	return nil
}

func (x *Bootstrap) GetAuth() *Auth {
	if x != nil {
		return x.Auth
	}
	return nil
}

func (x *Bootstrap) GetObservability() *Bootstrap_Observability {
	if x != nil {
		return x.Observability
	}
	return nil
}

func (x *Bootstrap) GetCredentialsService() *Credentials {
	if x != nil {
		return x.CredentialsService
	}
	return nil
}

type Server struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Regular HTTP endpoint
	Http *Server_HTTP `protobuf:"bytes,1,opt,name=http,proto3" json:"http,omitempty"`
	// GRPC API endpoint
	Grpc *Server_GRPC `protobuf:"bytes,2,opt,name=grpc,proto3" json:"grpc,omitempty"`
	// hHTTP server where the prometheus metrics will get exposed
	HttpMetrics *Server_HTTP `protobuf:"bytes,3,opt,name=http_metrics,json=httpMetrics,proto3" json:"http_metrics,omitempty"`
}

func (x *Server) Reset() {
	*x = Server{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Server) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Server) ProtoMessage() {}

func (x *Server) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Server.ProtoReflect.Descriptor instead.
func (*Server) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{1}
}

func (x *Server) GetHttp() *Server_HTTP {
	if x != nil {
		return x.Http
	}
	return nil
}

func (x *Server) GetGrpc() *Server_GRPC {
	if x != nil {
		return x.Grpc
	}
	return nil
}

func (x *Server) GetHttpMetrics() *Server_HTTP {
	if x != nil {
		return x.HttpMetrics
	}
	return nil
}

type Auth struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Public key used to verify the received JWT token
	// This token in the context of chainloop has been crafted by the controlplane
	RobotAccountPublicKeyPath string `protobuf:"bytes,1,opt,name=robot_account_public_key_path,json=robotAccountPublicKeyPath,proto3" json:"robot_account_public_key_path,omitempty"`
}

func (x *Auth) Reset() {
	*x = Auth{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Auth) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Auth) ProtoMessage() {}

func (x *Auth) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Auth.ProtoReflect.Descriptor instead.
func (*Auth) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{2}
}

func (x *Auth) GetRobotAccountPublicKeyPath() string {
	if x != nil {
		return x.RobotAccountPublicKeyPath
	}
	return ""
}

// Where the credentials to access the backends are stored
type Credentials struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Backend:
	//
	//	*Credentials_AwsSecretManager
	//	*Credentials_Vault_
	Backend isCredentials_Backend `protobuf_oneof:"backend"`
}

func (x *Credentials) Reset() {
	*x = Credentials{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Credentials) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Credentials) ProtoMessage() {}

func (x *Credentials) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Credentials.ProtoReflect.Descriptor instead.
func (*Credentials) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{3}
}

func (m *Credentials) GetBackend() isCredentials_Backend {
	if m != nil {
		return m.Backend
	}
	return nil
}

func (x *Credentials) GetAwsSecretManager() *Credentials_AWSSecretManager {
	if x, ok := x.GetBackend().(*Credentials_AwsSecretManager); ok {
		return x.AwsSecretManager
	}
	return nil
}

func (x *Credentials) GetVault() *Credentials_Vault {
	if x, ok := x.GetBackend().(*Credentials_Vault_); ok {
		return x.Vault
	}
	return nil
}

type isCredentials_Backend interface {
	isCredentials_Backend()
}

type Credentials_AwsSecretManager struct {
	AwsSecretManager *Credentials_AWSSecretManager `protobuf:"bytes,1,opt,name=aws_secret_manager,json=awsSecretManager,proto3,oneof"`
}

type Credentials_Vault_ struct {
	Vault *Credentials_Vault `protobuf:"bytes,2,opt,name=vault,proto3,oneof"`
}

func (*Credentials_AwsSecretManager) isCredentials_Backend() {}

func (*Credentials_Vault_) isCredentials_Backend() {}

type Bootstrap_Observability struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Sentry *Bootstrap_Observability_Sentry `protobuf:"bytes,1,opt,name=sentry,proto3" json:"sentry,omitempty"`
}

func (x *Bootstrap_Observability) Reset() {
	*x = Bootstrap_Observability{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Bootstrap_Observability) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Bootstrap_Observability) ProtoMessage() {}

func (x *Bootstrap_Observability) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Bootstrap_Observability.ProtoReflect.Descriptor instead.
func (*Bootstrap_Observability) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Bootstrap_Observability) GetSentry() *Bootstrap_Observability_Sentry {
	if x != nil {
		return x.Sentry
	}
	return nil
}

// Sentry notification support
type Bootstrap_Observability_Sentry struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Dsn         string `protobuf:"bytes,1,opt,name=dsn,proto3" json:"dsn,omitempty"`
	Environment string `protobuf:"bytes,2,opt,name=environment,proto3" json:"environment,omitempty"`
}

func (x *Bootstrap_Observability_Sentry) Reset() {
	*x = Bootstrap_Observability_Sentry{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Bootstrap_Observability_Sentry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Bootstrap_Observability_Sentry) ProtoMessage() {}

func (x *Bootstrap_Observability_Sentry) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Bootstrap_Observability_Sentry.ProtoReflect.Descriptor instead.
func (*Bootstrap_Observability_Sentry) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{0, 0, 0}
}

func (x *Bootstrap_Observability_Sentry) GetDsn() string {
	if x != nil {
		return x.Dsn
	}
	return ""
}

func (x *Bootstrap_Observability_Sentry) GetEnvironment() string {
	if x != nil {
		return x.Environment
	}
	return ""
}

type Server_HTTP struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Network string               `protobuf:"bytes,1,opt,name=network,proto3" json:"network,omitempty"`
	Addr    string               `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	Timeout *durationpb.Duration `protobuf:"bytes,3,opt,name=timeout,proto3" json:"timeout,omitempty"`
}

func (x *Server_HTTP) Reset() {
	*x = Server_HTTP{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Server_HTTP) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Server_HTTP) ProtoMessage() {}

func (x *Server_HTTP) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Server_HTTP.ProtoReflect.Descriptor instead.
func (*Server_HTTP) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{1, 0}
}

func (x *Server_HTTP) GetNetwork() string {
	if x != nil {
		return x.Network
	}
	return ""
}

func (x *Server_HTTP) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *Server_HTTP) GetTimeout() *durationpb.Duration {
	if x != nil {
		return x.Timeout
	}
	return nil
}

type Server_GRPC struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Network string               `protobuf:"bytes,1,opt,name=network,proto3" json:"network,omitempty"`
	Addr    string               `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`
	Timeout *durationpb.Duration `protobuf:"bytes,3,opt,name=timeout,proto3" json:"timeout,omitempty"`
}

func (x *Server_GRPC) Reset() {
	*x = Server_GRPC{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Server_GRPC) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Server_GRPC) ProtoMessage() {}

func (x *Server_GRPC) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Server_GRPC.ProtoReflect.Descriptor instead.
func (*Server_GRPC) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{1, 1}
}

func (x *Server_GRPC) GetNetwork() string {
	if x != nil {
		return x.Network
	}
	return ""
}

func (x *Server_GRPC) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *Server_GRPC) GetTimeout() *durationpb.Duration {
	if x != nil {
		return x.Timeout
	}
	return nil
}

// Top level is deprecated now
type Credentials_AWSSecretManager struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Creds  *Credentials_AWSSecretManager_Creds `protobuf:"bytes,1,opt,name=creds,proto3" json:"creds,omitempty"`
	Region string                              `protobuf:"bytes,2,opt,name=region,proto3" json:"region,omitempty"`
}

func (x *Credentials_AWSSecretManager) Reset() {
	*x = Credentials_AWSSecretManager{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Credentials_AWSSecretManager) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Credentials_AWSSecretManager) ProtoMessage() {}

func (x *Credentials_AWSSecretManager) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Credentials_AWSSecretManager.ProtoReflect.Descriptor instead.
func (*Credentials_AWSSecretManager) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{3, 0}
}

func (x *Credentials_AWSSecretManager) GetCreds() *Credentials_AWSSecretManager_Creds {
	if x != nil {
		return x.Creds
	}
	return nil
}

func (x *Credentials_AWSSecretManager) GetRegion() string {
	if x != nil {
		return x.Region
	}
	return ""
}

type Credentials_Vault struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// TODO: Use application role auth instead
	Token string `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
	// Instance address, including port
	// i.e "http://127.0.0.1:8200"
	Address string `protobuf:"bytes,2,opt,name=address,proto3" json:"address,omitempty"`
	// mount path of the kv engine, default /secret
	MountPath string `protobuf:"bytes,3,opt,name=mount_path,json=mountPath,proto3" json:"mount_path,omitempty"`
}

func (x *Credentials_Vault) Reset() {
	*x = Credentials_Vault{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Credentials_Vault) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Credentials_Vault) ProtoMessage() {}

func (x *Credentials_Vault) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Credentials_Vault.ProtoReflect.Descriptor instead.
func (*Credentials_Vault) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{3, 1}
}

func (x *Credentials_Vault) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

func (x *Credentials_Vault) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *Credentials_Vault) GetMountPath() string {
	if x != nil {
		return x.MountPath
	}
	return ""
}

type Credentials_AWSSecretManager_Creds struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	AccessKey string `protobuf:"bytes,1,opt,name=access_key,json=accessKey,proto3" json:"access_key,omitempty"`
	SecretKey string `protobuf:"bytes,2,opt,name=secret_key,json=secretKey,proto3" json:"secret_key,omitempty"`
}

func (x *Credentials_AWSSecretManager_Creds) Reset() {
	*x = Credentials_AWSSecretManager_Creds{}
	if protoimpl.UnsafeEnabled {
		mi := &file_conf_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Credentials_AWSSecretManager_Creds) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Credentials_AWSSecretManager_Creds) ProtoMessage() {}

func (x *Credentials_AWSSecretManager_Creds) ProtoReflect() protoreflect.Message {
	mi := &file_conf_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Credentials_AWSSecretManager_Creds.ProtoReflect.Descriptor instead.
func (*Credentials_AWSSecretManager_Creds) Descriptor() ([]byte, []int) {
	return file_conf_proto_rawDescGZIP(), []int{3, 0, 0}
}

func (x *Credentials_AWSSecretManager_Creds) GetAccessKey() string {
	if x != nil {
		return x.AccessKey
	}
	return ""
}

func (x *Credentials_AWSSecretManager_Creds) GetSecretKey() string {
	if x != nil {
		return x.SecretKey
	}
	return ""
}

var File_conf_proto protoreflect.FileDescriptor

var file_conf_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x63, 0x6f, 0x6e, 0x66, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x64, 0x75,
	0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xcf, 0x02, 0x0a,
	0x09, 0x42, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x12, 0x1f, 0x0a, 0x06, 0x73, 0x65,
	0x72, 0x76, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x07, 0x2e, 0x53, 0x65, 0x72,
	0x76, 0x65, 0x72, 0x52, 0x06, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x12, 0x19, 0x0a, 0x04, 0x61,
	0x75, 0x74, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x05, 0x2e, 0x41, 0x75, 0x74, 0x68,
	0x52, 0x04, 0x61, 0x75, 0x74, 0x68, 0x12, 0x3e, 0x0a, 0x0d, 0x6f, 0x62, 0x73, 0x65, 0x72, 0x76,
	0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18, 0x2e,
	0x42, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x2e, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76,
	0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x52, 0x0d, 0x6f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61,
	0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x12, 0x3d, 0x0a, 0x13, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e,
	0x74, 0x69, 0x61, 0x6c, 0x73, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c,
	0x73, 0x52, 0x12, 0x63, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x53, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x1a, 0x86, 0x01, 0x0a, 0x0d, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76,
	0x61, 0x62, 0x69, 0x6c, 0x69, 0x74, 0x79, 0x12, 0x37, 0x0a, 0x06, 0x73, 0x65, 0x6e, 0x74, 0x72,
	0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x42, 0x6f, 0x6f, 0x74, 0x73, 0x74,
	0x72, 0x61, 0x70, 0x2e, 0x4f, 0x62, 0x73, 0x65, 0x72, 0x76, 0x61, 0x62, 0x69, 0x6c, 0x69, 0x74,
	0x79, 0x2e, 0x53, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x06, 0x73, 0x65, 0x6e, 0x74, 0x72, 0x79,
	0x1a, 0x3c, 0x0a, 0x06, 0x53, 0x65, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x64, 0x73,
	0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x64, 0x73, 0x6e, 0x12, 0x20, 0x0a, 0x0b,
	0x65, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0b, 0x65, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x22, 0xd3,
	0x02, 0x0a, 0x06, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x12, 0x20, 0x0a, 0x04, 0x68, 0x74, 0x74,
	0x70, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72,
	0x2e, 0x48, 0x54, 0x54, 0x50, 0x52, 0x04, 0x68, 0x74, 0x74, 0x70, 0x12, 0x20, 0x0a, 0x04, 0x67,
	0x72, 0x70, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x53, 0x65, 0x72, 0x76,
	0x65, 0x72, 0x2e, 0x47, 0x52, 0x50, 0x43, 0x52, 0x04, 0x67, 0x72, 0x70, 0x63, 0x12, 0x2f, 0x0a,
	0x0c, 0x68, 0x74, 0x74, 0x70, 0x5f, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x0c, 0x2e, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2e, 0x48, 0x54, 0x54,
	0x50, 0x52, 0x0b, 0x68, 0x74, 0x74, 0x70, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x1a, 0x69,
	0x0a, 0x04, 0x48, 0x54, 0x54, 0x50, 0x12, 0x18, 0x0a, 0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72,
	0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b,
	0x12, 0x12, 0x0a, 0x04, 0x61, 0x64, 0x64, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x61, 0x64, 0x64, 0x72, 0x12, 0x33, 0x0a, 0x07, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x52, 0x07, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x1a, 0x69, 0x0a, 0x04, 0x47, 0x52, 0x50,
	0x43, 0x12, 0x18, 0x0a, 0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x6e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x12, 0x12, 0x0a, 0x04, 0x61,
	0x64, 0x64, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x61, 0x64, 0x64, 0x72, 0x12,
	0x33, 0x0a, 0x07, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x07, 0x74, 0x69, 0x6d,
	0x65, 0x6f, 0x75, 0x74, 0x22, 0x48, 0x0a, 0x04, 0x41, 0x75, 0x74, 0x68, 0x12, 0x40, 0x0a, 0x1d,
	0x72, 0x6f, 0x62, 0x6f, 0x74, 0x5f, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x5f, 0x70, 0x75,
	0x62, 0x6c, 0x69, 0x63, 0x5f, 0x6b, 0x65, 0x79, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x19, 0x72, 0x6f, 0x62, 0x6f, 0x74, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e,
	0x74, 0x50, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x4b, 0x65, 0x79, 0x50, 0x61, 0x74, 0x68, 0x22, 0x9a,
	0x03, 0x0a, 0x0b, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x12, 0x4d,
	0x0a, 0x12, 0x61, 0x77, 0x73, 0x5f, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x5f, 0x6d, 0x61, 0x6e,
	0x61, 0x67, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x43, 0x72, 0x65,
	0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x2e, 0x41, 0x57, 0x53, 0x53, 0x65, 0x63, 0x72,
	0x65, 0x74, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x48, 0x00, 0x52, 0x10, 0x61, 0x77, 0x73,
	0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x12, 0x2a, 0x0a,
	0x05, 0x76, 0x61, 0x75, 0x6c, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x43,
	0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x2e, 0x56, 0x61, 0x75, 0x6c, 0x74,
	0x48, 0x00, 0x52, 0x05, 0x76, 0x61, 0x75, 0x6c, 0x74, 0x1a, 0xac, 0x01, 0x0a, 0x10, 0x41, 0x57,
	0x53, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x12, 0x39,
	0x0a, 0x05, 0x63, 0x72, 0x65, 0x64, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x23, 0x2e,
	0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x2e, 0x41, 0x57, 0x53, 0x53,
	0x65, 0x63, 0x72, 0x65, 0x74, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x2e, 0x43, 0x72, 0x65,
	0x64, 0x73, 0x52, 0x05, 0x63, 0x72, 0x65, 0x64, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x72, 0x65, 0x67,
	0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x72, 0x65, 0x67, 0x69, 0x6f,
	0x6e, 0x1a, 0x45, 0x0a, 0x05, 0x43, 0x72, 0x65, 0x64, 0x73, 0x12, 0x1d, 0x0a, 0x0a, 0x61, 0x63,
	0x63, 0x65, 0x73, 0x73, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09,
	0x61, 0x63, 0x63, 0x65, 0x73, 0x73, 0x4b, 0x65, 0x79, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x65, 0x63,
	0x72, 0x65, 0x74, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x73,
	0x65, 0x63, 0x72, 0x65, 0x74, 0x4b, 0x65, 0x79, 0x1a, 0x56, 0x0a, 0x05, 0x56, 0x61, 0x75, 0x6c,
	0x74, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65,
	0x73, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73,
	0x73, 0x12, 0x1d, 0x0a, 0x0a, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x50, 0x61, 0x74, 0x68,
	0x42, 0x09, 0x0a, 0x07, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x42, 0x46, 0x5a, 0x44, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x6c,
	0x6f, 0x6f, 0x70, 0x2d, 0x64, 0x65, 0x76, 0x2f, 0x62, 0x65, 0x64, 0x72, 0x6f, 0x63, 0x6b, 0x2f,
	0x61, 0x70, 0x70, 0x2f, 0x61, 0x72, 0x74, 0x69, 0x66, 0x61, 0x63, 0x74, 0x2d, 0x63, 0x61, 0x73,
	0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x3b, 0x63,
	0x6f, 0x6e, 0x66, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_conf_proto_rawDescOnce sync.Once
	file_conf_proto_rawDescData = file_conf_proto_rawDesc
)

func file_conf_proto_rawDescGZIP() []byte {
	file_conf_proto_rawDescOnce.Do(func() {
		file_conf_proto_rawDescData = protoimpl.X.CompressGZIP(file_conf_proto_rawDescData)
	})
	return file_conf_proto_rawDescData
}

var file_conf_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_conf_proto_goTypes = []interface{}{
	(*Bootstrap)(nil),                          // 0: Bootstrap
	(*Server)(nil),                             // 1: Server
	(*Auth)(nil),                               // 2: Auth
	(*Credentials)(nil),                        // 3: Credentials
	(*Bootstrap_Observability)(nil),            // 4: Bootstrap.Observability
	(*Bootstrap_Observability_Sentry)(nil),     // 5: Bootstrap.Observability.Sentry
	(*Server_HTTP)(nil),                        // 6: Server.HTTP
	(*Server_GRPC)(nil),                        // 7: Server.GRPC
	(*Credentials_AWSSecretManager)(nil),       // 8: Credentials.AWSSecretManager
	(*Credentials_Vault)(nil),                  // 9: Credentials.Vault
	(*Credentials_AWSSecretManager_Creds)(nil), // 10: Credentials.AWSSecretManager.Creds
	(*durationpb.Duration)(nil),                // 11: google.protobuf.Duration
}
var file_conf_proto_depIdxs = []int32{
	1,  // 0: Bootstrap.server:type_name -> Server
	2,  // 1: Bootstrap.auth:type_name -> Auth
	4,  // 2: Bootstrap.observability:type_name -> Bootstrap.Observability
	3,  // 3: Bootstrap.credentials_service:type_name -> Credentials
	6,  // 4: Server.http:type_name -> Server.HTTP
	7,  // 5: Server.grpc:type_name -> Server.GRPC
	6,  // 6: Server.http_metrics:type_name -> Server.HTTP
	8,  // 7: Credentials.aws_secret_manager:type_name -> Credentials.AWSSecretManager
	9,  // 8: Credentials.vault:type_name -> Credentials.Vault
	5,  // 9: Bootstrap.Observability.sentry:type_name -> Bootstrap.Observability.Sentry
	11, // 10: Server.HTTP.timeout:type_name -> google.protobuf.Duration
	11, // 11: Server.GRPC.timeout:type_name -> google.protobuf.Duration
	10, // 12: Credentials.AWSSecretManager.creds:type_name -> Credentials.AWSSecretManager.Creds
	13, // [13:13] is the sub-list for method output_type
	13, // [13:13] is the sub-list for method input_type
	13, // [13:13] is the sub-list for extension type_name
	13, // [13:13] is the sub-list for extension extendee
	0,  // [0:13] is the sub-list for field type_name
}

func init() { file_conf_proto_init() }
func file_conf_proto_init() {
	if File_conf_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_conf_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Bootstrap); i {
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
		file_conf_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Server); i {
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
		file_conf_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Auth); i {
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
		file_conf_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Credentials); i {
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
		file_conf_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Bootstrap_Observability); i {
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
		file_conf_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Bootstrap_Observability_Sentry); i {
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
		file_conf_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Server_HTTP); i {
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
		file_conf_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Server_GRPC); i {
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
		file_conf_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Credentials_AWSSecretManager); i {
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
		file_conf_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Credentials_Vault); i {
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
		file_conf_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Credentials_AWSSecretManager_Creds); i {
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
	file_conf_proto_msgTypes[3].OneofWrappers = []interface{}{
		(*Credentials_AwsSecretManager)(nil),
		(*Credentials_Vault_)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_conf_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_conf_proto_goTypes,
		DependencyIndexes: file_conf_proto_depIdxs,
		MessageInfos:      file_conf_proto_msgTypes,
	}.Build()
	File_conf_proto = out.File
	file_conf_proto_rawDesc = nil
	file_conf_proto_goTypes = nil
	file_conf_proto_depIdxs = nil
}
