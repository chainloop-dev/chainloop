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
	"crypto/ecdsa"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"regexp"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/getsentry/sentry-go"
	"github.com/go-kratos/kratos/v2/errors"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/genproto/googleapis/bytestream"

	"buf.build/go/protovalidate"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	protovalidateMiddleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	grpcselector "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpcLib "google.golang.org/grpc"
)

// Opts provides the shared service implementations and transport settings.
type Opts struct {
	ServerConfig *conf.Server
	AuthConfig   *conf.Auth
	ByteService  *service.ByteStreamService
	ResourceSvc  *service.ResourceService
	DownloadSvc  *service.DownloadService
	Providers    backend.Providers
	Validator    protovalidate.Validator
	Logger       log.Logger
}

// GRPCServers keeps the shared endpoint's private native gRPC server separate
// from the optional legacy listener. Both run the same services and
// interceptors. Private binds an ephemeral loopback listener that the shared
// endpoint's reverse proxy dials; it is independent of the legacy listener's
// address, TLS configuration, and flag, and exists whether or not the legacy
// listener is enabled.
type GRPCServers struct {
	Private     *grpc.Server
	PrivateAddr string
	// PrivateLis holds the private listener so a caller that never starts
	// the server can still close the socket: before Start registers the
	// listener with grpc-go, Private.Stop does not close it.
	PrivateLis net.Listener
	Legacy     *grpc.Server
}

// Close releases the private socket without serving. Callers that hand the
// servers to the application lifecycle must not call it; construction-failure
// paths use it to avoid leaking the bound address.
func (s *GRPCServers) Close() error {
	if s.PrivateLis == nil {
		return nil
	}
	err := s.PrivateLis.Close()
	s.PrivateLis = nil
	return err
}

func legacyGRPCListenerEnabled(c *conf.Server) bool {
	return c.LegacyGrpcListener == nil || *c.LegacyGrpcListener
}

// maxConcurrentStreamsPrivate caps the concurrent HTTP/2 streams the shared
// endpoint's reverse proxy runs on a single upstream connection to the private
// gRPC server. grpc-go sets no limit by default, which lets net/http's HTTP/2
// client funnel every proxied stream onto one upstream connection whose ~1 GiB
// flow-control window then becomes a shared, exhaustible resource: a
// stalled-reader flood exhausts it and wedges gRPC for every tenant (PFM-7978 /
// M1). Capping it makes the client spread streams across several connections,
// each with its own window, so no single shared window can be exhausted. It
// applies only to the private server behind the proxy; direct clients of the
// legacy listener are unaffected.
const maxConcurrentStreamsPrivate = 100

// NewGRPCServers builds the private native server for the shared endpoint and
// the optional legacy listener, with the same services and interceptors.
func NewGRPCServers(opts *Opts) (*GRPCServers, error) {
	// Bind the private listener here so the shared endpoint's proxy knows its
	// address at construction. Loopback-only, ephemeral: never public, never
	// configurable, and independent of server.grpc.addr (which may carry TLS).
	privateLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("binding the shared endpoint's private gRPC listener: %w", err)
	}
	// Cap concurrent streams per upstream connection so the proxy cannot funnel
	// every tenant onto one connection and exhaust its shared flow-control
	// window (PFM-7978 / M1).
	private, err := newGRPCServer(opts,
		[]grpcLib.ServerOption{grpcLib.MaxConcurrentStreams(maxConcurrentStreamsPrivate)},
		grpc.Listener(privateLis))
	if err != nil {
		_ = privateLis.Close()
		return nil, err
	}
	servers := &GRPCServers{Private: private, PrivateAddr: privateLis.Addr().String(), PrivateLis: privateLis}
	if legacyGRPCListenerEnabled(opts.ServerConfig) {
		servers.Legacy, err = NewGRPCServer(opts)
		if err != nil {
			private.Server.Stop()
			_ = servers.Close()
			return nil, err
		}
	}
	return servers, nil
}

// NewGRPCServer retains the existing native listener and its independent TLS.
func NewGRPCServer(opts *Opts) (*grpc.Server, error) {
	c := opts.ServerConfig.GetGrpc()
	var serverOpts []grpc.ServerOption
	if c.GetNetwork() != "" {
		serverOpts = append(serverOpts, grpc.Network(c.GetNetwork()))
	}
	if c.GetAddr() != "" {
		serverOpts = append(serverOpts, grpc.Address(c.GetAddr()))
	}
	if tlsConf := c.GetTlsConfig(); tlsConf != nil {
		cert, key := tlsConf.GetCertificate(), tlsConf.GetPrivateKey()
		// Keep the legacy behavior for partial TLS configuration.
		if cert != "" && key != "" {
			pair, err := tls.LoadX509KeyPair(cert, key)
			if err != nil {
				return nil, fmt.Errorf("loading gRPC server TLS certificate: %w", err)
			}
			serverOpts = append(serverOpts, grpc.TLSConfig(&tls.Config{
				Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12,
			}))
		}
	}
	return newGRPCServer(opts, nil, serverOpts...)
}

func newGRPCServer(settings *Opts, grpcServerOpts []grpcLib.ServerOption, extraOpts ...grpc.ServerOption) (*grpc.Server, error) {
	c, authConf := settings.ServerConfig, settings.AuthConfig
	byteService, rSvc := settings.ByteService, settings.ResourceSvc
	providers, validator, logger := settings.Providers, settings.Validator, settings.Logger
	// Parse the public key once on initialization instead of on every request
	// TODO: implement jwks endpoint
	publicKey, err := parsePublicKey(authConf, logger)
	if err != nil {
		return nil, err
	}

	// Share a single keyfunc closure over the parsed key across all interceptors
	keyFunc := loadPublicKey(publicKey)

	// Fold any caller-supplied grpc-go options (e.g. the private server's
	// concurrent-stream cap) into the single grpc.Options call below. kratos's
	// grpc.Options assigns rather than appends, so a second grpc.Options call
	// would silently drop the stats handler registered here.
	rawGRPCOpts := append([]grpcLib.ServerOption{
		grpcLib.StatsHandler(otelgrpc.NewServerHandler()),
	}, grpcServerOpts...)

	var opts = []grpc.ServerOption{
		// Kratos middleware are in practice unary interceptors
		grpc.Middleware(
			recovery.Recovery(
				recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
					sentry.CaptureMessage(fmt.Sprintf("%v", err))
					return errors.InternalServer("internal error", "there was an internal error")
				}),
			),
			logging.Server(logger),
			// NOTE: JWT middleware only works for unary requests
			// below you can see a re-implementation of the middleware as a stream interceptor
			// If we require a logged in user we
			selector.Server(
				jwtMiddleware.Server(
					keyFunc,
					jwtMiddleware.WithSigningMethod(casJWT.SigningMethod),
					jwtMiddleware.WithClaims(func() jwt.Claims { return &casJWT.Claims{} })),
			).Match(requireAuthentication()).Build(),
		),

		// Streaming interceptors
		grpc.StreamInterceptor(
			grpcselector.StreamServerInterceptor(
				grpc_auth.StreamServerInterceptor(jwtAuthFunc(keyFunc, casJWT.SigningMethod)),
				grpcselector.MatchFunc(allButReflectionAPI),
			),
			// grpc prometheus metrics
			grpc_prometheus.StreamServerInterceptor,
		),
		grpc.UnaryInterceptor(
			grpc_prometheus.UnaryServerInterceptor,
			protovalidateMiddleware.UnaryServerInterceptor(validator),
		),
		grpc.Options(rawGRPCOpts...),
	}

	// Opt-in histogram metrics for the interceptor
	// Since we track uploads / downloads we'll increase the buckets
	grpc_prometheus.EnableHandlingTimeHistogram(grpc_prometheus.WithHistogramBuckets(prometheus.ExponentialBucketsRange(0.5, 60, 8)))

	opts = append(opts, extraOpts...)
	if c.GetGrpc().GetTimeout() != nil {
		opts = append(opts, grpc.Timeout(c.GetGrpc().GetTimeout().AsDuration()))
	}

	srv := grpc.NewServer(opts...)

	bytestream.RegisterByteStreamServer(srv.Server, byteService)
	v1.RegisterResourceServiceServer(srv.Server, rSvc)
	v1.RegisterStatusServiceServer(srv.Server, service.NewStatusService(Version, providers))

	// Register and set metrics to 0
	grpc_prometheus.Register(srv.Server)

	return srv, nil
}

var (
	statusServiceOperationRegexp = regexp.MustCompile("(cas.v1.StatusService/.*)")
	reflectionServiceRegexp      = regexp.MustCompile("(grpc.reflection.*)")
)

func requireAuthentication() selector.MatchFunc {
	// Skip authentication on the status grpc service
	return func(ctx context.Context, operation string) bool {
		return !statusServiceOperationRegexp.MatchString(operation)
	}
}

// Reflection API is called by clients like grpcurl to list services
// and without this selector check it would require authentication
func allButReflectionAPI(_ context.Context, callMeta interceptors.CallMeta) bool {
	return !reflectionServiceRegexp.MatchString(callMeta.Service)
}

// parsePublicKey resolves the configured public key path, reads the file and parses
// the EC public key once. A malformed key therefore fails at server construction
// instead of surfacing as a per-request authentication error.
func parsePublicKey(authConf *conf.Auth, logger log.Logger) (*ecdsa.PublicKey, error) {
	l := log.NewHelper(logger)

	publicKeyPath := authConf.GetPublicKeyPath()
	if publicKeyPath == "" {
		// Maintain backwards compatibility with the deprecated field.
		publicKeyPath = authConf.RobotAccountPublicKeyPath //nolint:staticcheck // intentional fallback to the deprecated field
	}

	l.Debugw("msg", "loading public key from file", "file", publicKeyPath)

	rawKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %w", err)
	}

	publicKey, err := jwt.ParseECPublicKeyFromPEM(rawKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return publicKey, nil
}

// loadPublicKey returns a jwt.Keyfunc that hands back the pre-parsed public key,
// avoiding a PEM re-parse on every request.
func loadPublicKey(publicKey *ecdsa.PublicKey) jwt.Keyfunc {
	return func(_ *jwt.Token) (interface{}, error) {
		return publicKey, nil
	}
}

// Reimplementation of the kratos jwt middleware suited as stream interceptor
func jwtAuthFunc(keyFunc jwt.Keyfunc, signingMethod jwt.SigningMethod) grpc_auth.AuthFunc {
	return func(ctx context.Context) (context.Context, error) {
		token, err := grpc_auth.AuthFromMD(ctx, "bearer")
		if err != nil {
			return nil, err
		}

		claims, err := verifyAndMarshalJWT(token, keyFunc, signingMethod)
		if err != nil {
			return nil, err
		}

		return jwtMiddleware.NewContext(ctx, claims), nil
	}
}

// verifyAndMarshalJWT verifies the token and returns the claims
func verifyAndMarshalJWT(token string, keyFunc jwt.Keyfunc, signingMethod jwt.SigningMethod) (*casJWT.Claims, error) {
	var tokenInfo *jwt.Token
	claims := &casJWT.Claims{}

	tokenInfo, err := jwt.ParseWithClaims(token, claims, keyFunc)
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenMalformed):
			return nil, jwtMiddleware.ErrTokenInvalid
		case errors.Is(err, jwt.ErrTokenExpired), errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, jwtMiddleware.ErrTokenExpired
		default:
			return nil, errors.Unauthorized("UNAUTHORIZED", err.Error())
		}
	}

	if !tokenInfo.Valid {
		return nil, jwtMiddleware.ErrTokenInvalid
	}

	if tokenInfo.Method != signingMethod {
		return nil, jwtMiddleware.ErrUnSupportSigningMethod
	}

	return claims, nil
}
