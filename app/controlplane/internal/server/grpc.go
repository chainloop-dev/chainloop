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
	"regexp"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/robotaccount"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/user"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/getsentry/sentry-go"
	jwt "github.com/golang-jwt/jwt/v4"

	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
)

type Opts struct {
	// UseCases
	UserUseCase         *biz.UserUseCase
	RobotAccountUseCase *biz.RobotAccountUseCase
	CASBackendUseCase   *biz.CASBackendUseCase
	CASClientUseCase    *biz.CASClientUseCase
	IntegrationUseCase  *biz.IntegrationUseCase
	ReferrerUseCase     *biz.ReferrerUseCase
	// Services
	WorkflowSvc         *service.WorkflowService
	AuthSvc             *service.AuthService
	RobotAccountSvc     *service.RobotAccountService
	WorkflowRunSvc      *service.WorkflowRunService
	AttestationSvc      *service.AttestationService
	WorkflowContractSvc *service.WorkflowContractService
	ContextSvc          *service.ContextService
	CASCredsSvc         *service.CASCredentialsService
	OrgMetricsSvc       *service.OrgMetricsService
	IntegrationsSvc     *service.IntegrationsService
	OrganizationSvc     *service.OrganizationService
	CASBackendSvc       *service.CASBackendService
	CASRedirectSvc      *service.CASRedirectService
	OrgInvitationSvc    *service.OrgInvitationService
	ReferrerSvc         *service.ReferrerService
	// Utils
	Logger       log.Logger
	ServerConfig *conf.Server
	AuthConfig   *conf.Auth
	Credentials  credentials.ReaderWriter
}

// NewGRPCServer new a gRPC server.
func NewGRPCServer(opts *Opts) (*grpc.Server, error) {
	// Opt-in histogram metrics for the interceptor
	grpc_prometheus.EnableHandlingTimeHistogram()

	var serverOpts = []grpc.ServerOption{
		grpc.Middleware(craftMiddleware(opts)...),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
	}

	if v := opts.ServerConfig.Grpc.Network; v != "" {
		serverOpts = append(serverOpts, grpc.Network(v))
	}
	if v := opts.ServerConfig.Grpc.Addr; v != "" {
		serverOpts = append(serverOpts, grpc.Address(v))
	}
	if v := opts.ServerConfig.Grpc.Timeout; v != nil {
		serverOpts = append(serverOpts, grpc.Timeout(v.AsDuration()))
	}
	if tlsConf := opts.ServerConfig.Grpc.GetTlsConfig(); tlsConf != nil {
		cert := tlsConf.GetCertificate()
		privKey := tlsConf.GetPrivateKey()
		if cert != "" && privKey != "" {
			cert, err := tls.LoadX509KeyPair(cert, privKey)
			if err != nil {
				return nil, fmt.Errorf("loading gRPC server TLS certificate: %w", err)
			}
			serverOpts = append(serverOpts, grpc.TLSConfig(&tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12, // gosec complains about insecure minimum version we use default value
			}))
		}
	}

	srv := grpc.NewServer(serverOpts...)
	v1.RegisterWorkflowServiceServer(srv, opts.WorkflowSvc)
	v1.RegisterStatusServiceServer(srv, service.NewStatusService(opts.AuthSvc.AuthURLs.Login, Version, opts.CASClientUseCase))
	v1.RegisterRobotAccountServiceServer(srv, opts.RobotAccountSvc)
	v1.RegisterWorkflowRunServiceServer(srv, opts.WorkflowRunSvc)
	v1.RegisterAttestationServiceServer(srv, opts.AttestationSvc)
	v1.RegisterWorkflowContractServiceServer(srv, opts.WorkflowContractSvc)
	v1.RegisterCASCredentialsServiceServer(srv, opts.CASCredsSvc)
	v1.RegisterContextServiceServer(srv, opts.ContextSvc)
	v1.RegisterOrgMetricsServiceServer(srv, opts.OrgMetricsSvc)
	v1.RegisterIntegrationsServiceServer(srv, opts.IntegrationsSvc)
	v1.RegisterOrganizationServiceServer(srv, opts.OrganizationSvc)
	v1.RegisterAuthServiceServer(srv, opts.AuthSvc)
	v1.RegisterCASBackendServiceServer(srv, opts.CASBackendSvc)
	v1.RegisterCASRedirectServiceServer(srv, opts.CASRedirectSvc)
	v1.RegisterOrgInvitationServiceServer(srv, opts.OrgInvitationSvc)
	v1.RegisterReferrerServiceServer(srv, opts.ReferrerSvc)

	// Register Prometheus metrics
	grpc_prometheus.Register(srv.Server)

	return srv, nil
}

func craftMiddleware(opts *Opts) []middleware.Middleware {
	middlewares := []middleware.Middleware{
		recovery.Recovery(
			recovery.WithHandler(func(ctx context.Context, req, err interface{}) error {
				sentry.CaptureMessage(fmt.Sprintf("%v", err))
				return errors.InternalServer("internal error", "there was an internal error")
			}),
		),
		logging.Server(opts.Logger),
	}

	logHelper := log.NewHelper(opts.Logger)

	// User authentication
	middlewares = append(middlewares,
		// If we require a logged in user we
		selector.Server(
			// 1 - Extract the currentUser from the JWT
			jwtMiddleware.Server(func(token *jwt.Token) (interface{}, error) {
				return []byte(opts.AuthConfig.GeneratedJwsHmacSecret), nil
			},
				jwtMiddleware.WithSigningMethod(user.SigningMethod),
				jwtMiddleware.WithClaims(func() jwt.Claims { return &user.CustomClaims{} }),
			),
			// 1 - Set its user and organization
			usercontext.WithCurrentUserAndOrgMiddleware(opts.UserUseCase, logHelper),
			// 3 - Make sure its account is fully functional
			selector.Server(
				usercontext.CheckUserInAllowList(opts.AuthConfig.AllowList),
				usercontext.CheckOrgRequirements(opts.CASBackendUseCase),
			).Match(requireFullyConfiguredOrgMatcher()).Build(),
		).Match(requireCurrentUserMatcher()).Build(),
	)

	// robot account authentication
	middlewares = append(middlewares,
		// if we require a robot account
		selector.Server(
			// 1 - Extract the robot account from the JWT
			jwtMiddleware.Server(func(token *jwt.Token) (interface{}, error) {
				// TODO: add support to multiple signing methods and keys
				return []byte(opts.AuthConfig.GeneratedJwsHmacSecret), nil
			},
				jwtMiddleware.WithSigningMethod(robotaccount.SigningMethod),
				jwtMiddleware.WithClaims(func() jwt.Claims { return &robotaccount.CustomClaims{} }),
			),
			// 2 - Set its workflow and organization in the context
			usercontext.WithCurrentRobotAccount(opts.RobotAccountUseCase, logHelper),
		).Match(requireRobotAccountMatcher()).Build(),
	)

	// Rest of middlewares
	middlewares = append(middlewares, validate.Validator())

	return middlewares
}

// If we should load the user
func requireCurrentUserMatcher() selector.MatchFunc {
	// Skip authentication on the status grpc service
	const skipRegexp = "(controlplane.v1.AttestationService/.*|controlplane.v1.StatusService/.*)"

	return func(ctx context.Context, operation string) bool {
		r := regexp.MustCompile(skipRegexp)
		return !r.MatchString(operation)
	}
}

func requireFullyConfiguredOrgMatcher() selector.MatchFunc {
	// We do not need to remove other endpoints since this matcher is called once the requireCurrentUserMatcher one has passed
	const skipRegexp = "controlplane.v1.OCIRepositoryService/.*|controlplane.v1.ContextService/Current|/controlplane.v1.OrganizationService/.*|/controlplane.v1.AuthService/DeleteAccount|controlplane.v1.CASBackendService/.*"

	return func(ctx context.Context, operation string) bool {
		r := regexp.MustCompile(skipRegexp)
		return !r.MatchString(operation)
	}
}

func requireRobotAccountMatcher() selector.MatchFunc {
	const requireMatcher = "controlplane.v1.AttestationService/.*"

	return func(ctx context.Context, operation string) bool {
		r := regexp.MustCompile(requireMatcher)
		return r.MatchString(operation)
	}
}
