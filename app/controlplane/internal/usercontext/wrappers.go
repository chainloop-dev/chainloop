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

package usercontext

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"

	"google.golang.org/grpc"
)

// This package contains a set of wrappers that take grpc.UnaryInterceptors and translate them into Kratos middlewares.
// The reason for having these kind of wrappers is so we can sort them in the context of the rest of middlewares.
// Otherwise, plain grpc interceptors will be added after the chain of middlewares
// https://github.com/go-kratos/kratos/blob/f8b97f675b32dfad02edae12d83053c720720b5b/transport/grpc/server.go#L166
func Prometheus() middleware.Middleware {
	var interceptor = grpc_prometheus.UnaryServerInterceptor
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Extract gRPC metadata from the context
			info, ok := transport.FromServerContext(ctx)
			if !ok {
				// If gRPC metadata is not available, fallback to default handler
				return handler(ctx, req)
			}

			// Wrap the handler into a gRPC UnaryHandler
			grpcHandler := func(ctx context.Context, req interface{}) (interface{}, error) {
				return handler(ctx, req)
			}

			// Call the interceptor
			return interceptor(ctx, req, &grpc.UnaryServerInfo{
				Server:     nil, // Kratos doesn't provide the server instance directly
				FullMethod: info.Operation(),
			}, grpcHandler)
		}
	}
}
