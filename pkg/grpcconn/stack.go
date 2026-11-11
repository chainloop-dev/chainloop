//
// Copyright 2026 The Chainloop Authors.
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

package grpcconn

import (
	"context"
	"io"

	"github.com/chainloop-dev/chainloop/pkg/errorsx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// StackUnaryClientInterceptor records the stack of the caller on each error of
// a unary call, so Sentry groups a failed call by the code that made it.
// The error keeps its message and its gRPC status.
func StackUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return errorsx.WithStack(invoker(ctx, method, req, reply, cc, opts...))
	}
}

// StackStreamClientInterceptor records the stack of the caller on each error of
// a streaming call: on stream creation, Header, SendMsg, RecvMsg and CloseSend.
// It does not wrap io.EOF, because callers compare it with ==.
func StackStreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		cs, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return nil, errorsx.WithStack(err)
		}

		return &stackClientStream{ClientStream: cs}, nil
	}
}

// stackClientStream is a client stream that records the stack of the caller on each error.
type stackClientStream struct {
	grpc.ClientStream
}

func (s *stackClientStream) SendMsg(m any) error {
	return streamErr(s.ClientStream.SendMsg(m))
}

func (s *stackClientStream) RecvMsg(m any) error {
	return streamErr(s.ClientStream.RecvMsg(m))
}

func (s *stackClientStream) CloseSend() error {
	return streamErr(s.ClientStream.CloseSend())
}

func (s *stackClientStream) Header() (metadata.MD, error) {
	md, err := s.ClientStream.Header()
	return md, streamErr(err)
}

// streamErr adds a stack to err, except to io.EOF, which ends a stream normally.
func streamErr(err error) error {
	if err == io.EOF { //nolint:errorlint // gRPC returns io.EOF unwrapped and callers compare it with ==.
		return err
	}

	return errorsx.WithStack(err)
}
