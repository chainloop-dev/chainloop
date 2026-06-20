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

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, authConf *conf.Auth, byteService *service.ByteStreamService, rSvc *service.ResourceService, providers backend.Providers, validator protovalidate.Validator, logger log.Logger) (*grpc.Server, error) {
	// Parse the public key once on initialization instead of on every request
	// TODO: implement jwks endpoint
	publicKey, err := parsePublicKey(authConf, logger)
	if err != nil {
		return nil, err
	}

	// Share a single keyfunc closure over the parsed key across all interceptors
	keyFunc := loadPublicKey(publicKey)

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
		grpc.Options(grpcLib.StatsHandler(otelgrpc.NewServerHandler())),
	}

	// Opt-in histogram metrics for the interceptor
	// Since we track uploads / downloads we'll increase the buckets
	grpc_prometheus.EnableHandlingTimeHistogram(grpc_prometheus.WithHistogramBuckets(prometheus.ExponentialBucketsRange(0.5, 60, 8)))

	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	if tlsConf := c.Grpc.GetTlsConfig(); tlsConf != nil {
		cert := tlsConf.GetCertificate()
		privKey := tlsConf.GetPrivateKey()
		if cert != "" && privKey != "" {
			cert, err := tls.LoadX509KeyPair(cert, privKey)
			if err != nil {
				return nil, fmt.Errorf("loading gRPC server TLS certificate: %w", err)
			}
			opts = append(opts, grpc.TLSConfig(&tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12, // gosec complains about insecure minimum version we use default value
			}))
		}
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
