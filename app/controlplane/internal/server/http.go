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

package server

import (
	"context"
	"errors"

	"github.com/bufbuild/protovalidate-go"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	h "net/http"

	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new a HTTP server.
func NewHTTPServer(opts *Opts, grpcSrv *grpc.Server) (*http.Server, error) {
	middlewares := craftMiddleware(opts)
	// important, the validation middleware should be the last one
	middlewares = append(middlewares, protoValidateHTTPMiddleware(opts.Validator))

	var serverOpts = []http.ServerOption{
		http.Middleware(middlewares...),
	}

	if v := opts.ServerConfig.Http.Network; v != "" {
		serverOpts = append(serverOpts, http.Network(v))
	}
	if v := opts.ServerConfig.Http.Addr; v != "" {
		serverOpts = append(serverOpts, http.Address(v))
	}
	if v := opts.ServerConfig.Http.Timeout; v != nil {
		serverOpts = append(serverOpts, http.Timeout(v.AsDuration()))
	}

	// initialize the underneath http server
	httpSrv := http.NewServer(serverOpts...)
	// NOTE: these non-grpc transcoded methods DO NOT RUN the middlewares
	httpSrv.Handle(service.AuthLoginPath, opts.AuthSvc.RegisterLoginHandler())
	httpSrv.Handle(service.AuthCallbackPath, opts.AuthSvc.RegisterCallbackHandler())
	v1.RegisterStatusServiceHTTPServer(httpSrv, service.NewStatusService(opts.AuthSvc.AuthURLs.Login, Version, opts.CASClientUseCase))
	v1.RegisterReferrerServiceHTTPServer(httpSrv, service.NewReferrerService(opts.ReferrerUseCase))

	// Wrap http server to handle grpc-web calls and we will return this new server
	wrappedServer := http.NewServer(serverOpts...)
	wrappedGrpc := grpcweb.WrapServer(grpcSrv.Server,
		// Be permissive about cors
		grpcweb.WithOriginFunc(func(_ string) bool { return true }),
	)

	r := httpSrv.Route("/")
	r.GET("/download/{digest}", opts.CASRedirectSvc.HTTPDownload)

	// Handle grpc-web requests or fallback
	wrappedServer.Handler = h.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		if wrappedGrpc.IsGrpcWebRequest(req) || wrappedGrpc.IsAcceptableGrpcCorsRequest(req) {
			wrappedGrpc.ServeHTTP(res, req)
			return
		}
		httpSrv.ServeHTTP(res, req)
	})

	return wrappedServer, nil
}

// Custom kraos middleware based on the protovalidate middleware
// https://pkg.go.dev/github.com/grpc-ecosystem/go-grpc-middleware/v2@v2.1.0/interceptors/protovalidate#UnaryServerInterceptor
// but tailored specifically for the http server
func protoValidateHTTPMiddleware(validator *protovalidate.Validator) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			switch msg := req.(type) {
			case proto.Message:
				if err = validator.Validate(msg); err != nil {
					return nil, status.Error(codes.InvalidArgument, err.Error())
				}
			default:
				return nil, errors.New("unsupported message type")
			}
			return handler(ctx, req)
		}
	}
}
