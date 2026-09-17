//
// Copyright 2024-2026 The Chainloop Authors.
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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"
	middlewares_http "github.com/chainloop-dev/chainloop/pkg/middlewares/http"
	"github.com/golang-jwt/jwt/v5"

	"buf.build/go/protovalidate"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	h "net/http"

	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(opts *Opts, grpcSrv *grpc.Server) (*http.Server, error) {
	var serverOpts = []http.ServerOption{
		http.Middleware(httpMiddlewares(opts)...),
	}
	// Prevent unmatched routes from falling through to http.DefaultServeMux,
	// which would otherwise expose net/http/pprof's /debug/pprof/* endpoints
	// unauthenticated on the public API listener.
	serverOpts = append(serverOpts, hardenedRouteOptions()...)

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
	// Relax secure cookie settings in development mode
	opts.AuthSvc.SetDevMode(Version == "dev")
	// NOTE: these non-grpc transcoded methods DO NOT RUN the middlewares, so
	// they are wrapped with otelhttp individually to still produce server spans.
	httpSrv.Handle(service.AuthLoginPath,
		otelhttp.NewHandler(middlewares_http.Logging(opts.Logger, opts.AuthSvc.RegisterLoginHandler()), "HTTP "+service.AuthLoginPath))
	httpSrv.Handle(service.AuthCallbackPath,
		otelhttp.NewHandler(middlewares_http.Logging(opts.Logger, opts.AuthSvc.RegisterCallbackHandler()), "HTTP "+service.AuthCallbackPath))
	httpSrv.Handle(service.PrometheusMetricsPath,
		otelhttp.NewHandler(
			middlewares_http.Logging(opts.Logger,
				middlewares_http.AuthFromAuthorizationHeader(
					loadJWTKeyFunc(opts.AuthConfig.GetGeneratedJwsHmacSecret()),
					apiTokenCustomClaims(),
					apitoken.SigningMethod,
					opts.PrometheusSvc,
				),
			), "HTTP "+service.PrometheusMetricsPath))
	statusSvc := service.NewStatusService(opts.AuthSvc.AuthURLs.Login, Version, opts.CASClientUseCase, opts.BootstrapConfig)
	v1.RegisterStatusServiceHTTPServer(httpSrv, statusSvc)
	v1.RegisterReferrerServiceHTTPServer(httpSrv, service.NewReferrerService(opts.ReferrerUseCase))

	// Wrap http server to handle grpc-web calls and we will return this new server
	wrappedServer := http.NewServer(serverOpts...)
	wrappedGrpc := grpcweb.WrapServer(grpcSrv.Server,
		// Be permissive about cors
		grpcweb.WithOriginFunc(func(_ string) bool { return true }),
	)

	r := httpSrv.Route("/")
	r.GET("/download/{digest}", opts.CASRedirectSvc.HTTPDownload)

	// Include the OpenAPI spec handler
	r.GET("/openapi.yaml", statusSvc.HandleOpenAPISpec)

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

// httpMiddlewares returns the middleware chain for the HTTP server.
// Transport-level tracing comes first: the gRPC server is instrumented via the
// otelgrpc stats handler, but transcoded HTTP requests get no server span
// otherwise, and tracing must be the outermost middleware so every other span
// (middlewares, biz, data) nests under it. Note that the handlers registered
// via httpSrv.Handle (auth login/callback, metrics) bypass this chain too, so
// they are wrapped with otelhttp individually instead. /openapi.yaml and
// /download/{digest} are kratos Context handlers that bypass the chain and
// remain untraced (low-value static/redirect endpoints).
func httpMiddlewares(opts *Opts) []middleware.Middleware {
	middlewares := append([]middleware.Middleware{tracing.Server()}, craftMiddleware(opts)...)
	// important, the validation middleware should be the last one
	return append(middlewares, protoValidateHTTPMiddleware(opts.Validator))
}

// Custom kraos middleware based on the protovalidate middleware
// https://pkg.go.dev/github.com/grpc-ecosystem/go-grpc-middleware/v2@v2.1.0/interceptors/protovalidate#UnaryServerInterceptor
// but tailored specifically for the http server
func protoValidateHTTPMiddleware(validator protovalidate.Validator) middleware.Middleware {
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

// loadJWTKeyFunc returns a Keyfunc that returns the raw key
func loadJWTKeyFunc(rawKey string) jwt.Keyfunc {
	return func(_ *jwt.Token) (interface{}, error) {
		return []byte(rawKey), nil
	}
}

// apiTokenCustomClaims returns a ClaimsFunc that returns a custom claims struct
func apiTokenCustomClaims() middlewares_http.ClaimsFunc {
	return func() jwt.Claims {
		return &apitoken.CustomClaims{}
	}
}
