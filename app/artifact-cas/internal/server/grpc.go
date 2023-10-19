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

package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"regexp"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/conf"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/getsentry/sentry-go"
	"github.com/go-kratos/kratos/v2/errors"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/genproto/googleapis/bytestream"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, authConf *conf.Auth, byteService *service.ByteStreamService, rSvc *service.ResourceService, providers backend.Providers, logger log.Logger) (*grpc.Server, error) {
	log := log.NewHelper(logger)
	// Load the key on initialization instead of on every request
	// TODO: implement jwks endpoint
	publicKeyPath := authConf.GetPublicKeyPath()
	if publicKeyPath == "" {
		// Maintain backwards compatibility
		publicKeyPath = authConf.RobotAccountPublicKeyPath
	}

	log.Debugw("msg", "loading public key from file", "file", publicKeyPath)

	rawKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load public key: %w", err)
	}

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
					loadPublicKey(rawKey),
					jwtMiddleware.WithSigningMethod(casJWT.SigningMethod),
					jwtMiddleware.WithClaims(func() jwt.Claims { return &casJWT.Claims{} })),
			).Match(requireAuthentication()).Build(),
			validate.Validator(),
		),

		// Streaming interceptors
		grpc.StreamInterceptor(
			grpc_auth.StreamServerInterceptor(jwtAuthFunc(loadPublicKey(rawKey), casJWT.SigningMethod)),
			// grpc prometheus metrics
			grpc_prometheus.StreamServerInterceptor,
		),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
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

func requireAuthentication() selector.MatchFunc {
	// Skip authentication on the status grpc service
	const skipRegexp = "(cas.v1.StatusService/.*)"

	return func(ctx context.Context, operation string) bool {
		r := regexp.MustCompile(skipRegexp)
		return !r.MatchString(operation)
	}
}

// load key for verification
func loadPublicKey(rawKey []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		return jwt.ParseECPublicKeyFromPEM(rawKey)
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
		var ve *jwt.ValidationError
		if !errors.As(err, &ve) {
			return nil, errors.Unauthorized("UNAUTHORIZED", err.Error())
		}

		if ve.Errors&jwt.ValidationErrorMalformed != 0 {
			return nil, jwtMiddleware.ErrTokenInvalid
		}

		if ve.Errors&(jwt.ValidationErrorExpired) != 0 {
			return nil, jwtMiddleware.ErrTokenExpired
		}

		if ve.Errors&(jwt.ValidationErrorNotValidYet) != 0 {
			return nil, jwtMiddleware.ErrTokenExpired
		}

		return nil, err
	}

	if !tokenInfo.Valid {
		return nil, jwtMiddleware.ErrTokenInvalid
	}

	if tokenInfo.Method != signingMethod {
		return nil, jwtMiddleware.ErrUnSupportSigningMethod
	}

	return claims, nil
}
