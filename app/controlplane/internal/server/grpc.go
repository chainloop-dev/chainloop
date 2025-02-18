//
// Copyright 2024-2025 The Chainloop Authors.
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
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/sentrycontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	authzMiddleware "github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz/middleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"

	"github.com/bufbuild/protovalidate-go"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/getsentry/sentry-go"
	"github.com/golang-jwt/jwt/v4"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	protovalidateMiddleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
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
	APITokenUseCase     *biz.APITokenUseCase
	OrganizationUseCase *biz.OrganizationUseCase
	WorkflowUseCase     *biz.WorkflowUseCase
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
	APITokenSvc         *service.APITokenService
	AttestationStateSvc *service.AttestationStateService
	UserSvc             *service.UserService
	SigningSvc          *service.SigningService
	PrometheusSvc       *service.PrometheusService
	// Utils
	Logger          log.Logger
	ServerConfig    *conf.Server
	AuthConfig      *conf.Auth
	FederatedConfig *conf.FederatedVerification
	Credentials     credentials.ReaderWriter
	Enforcer        *authz.Enforcer
	Validator       *protovalidate.Validator
}

// NewGRPCServer new a gRPC server.
func NewGRPCServer(opts *Opts) (*grpc.Server, error) {
	// Opt-in histogram metrics for the interceptor
	grpc_prometheus.EnableHandlingTimeHistogram()

	// NOTE: kratos middlewares will always be run before plain grpc unary interceptors
	// If you want to change the behavior, please refer to wrappers.go and the Prometheus wrapper as example
	var serverOpts = []grpc.ServerOption{
		grpc.Middleware(craftMiddleware(opts)...),
		grpc.UnaryInterceptor(
			protovalidateMiddleware.UnaryServerInterceptor(opts.Validator),
		),
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
	v1.RegisterAPITokenServiceServer(srv, opts.APITokenSvc)
	v1.RegisterAttestationStateServiceServer(srv, opts.AttestationStateSvc)
	v1.RegisterUserServiceServer(srv, opts.UserSvc)
	v1.RegisterSigningServiceServer(srv, opts.SigningSvc)

	// Register Prometheus metrics
	grpc_prometheus.Register(srv.Server)

	return srv, nil
}

func craftMiddleware(opts *Opts) []middleware.Middleware {
	middlewares := []middleware.Middleware{
		recovery.Recovery(
			recovery.WithHandler(func(_ context.Context, req, err interface{}) error {
				sentry.CaptureMessage(fmt.Sprintf("%v", err))
				return errors.InternalServer("internal error", "there was an internal error")
			}),
		),
		logging.Server(opts.Logger),
	}

	logHelper := log.NewHelper(opts.Logger)

	// User authentication
	middlewares = append(middlewares,
		usercontext.Prometheus(),
		// If we require a logged in user we
		selector.Server(
			// 1 - Extract the currentUser/API token from the JWT
			// NOTE: this works because both currentUser and API tokens JWT use the same signing method and secret
			jwtMiddleware.Server(func(_ *jwt.Token) (interface{}, error) {
				return []byte(opts.AuthConfig.GeneratedJwsHmacSecret), nil
			},
				jwtMiddleware.WithSigningMethod(user.SigningMethod),
			),
			// 2.a - Set its API token and organization as alternative to the user
			usercontext.WithCurrentAPITokenAndOrgMiddleware(opts.APITokenUseCase, opts.OrganizationUseCase, logHelper),
			// 2.b - Set its user
			usercontext.WithCurrentUserMiddleware(opts.UserUseCase, logHelper),
			selector.Server(
				// 2.c - Set its organization
				usercontext.WithCurrentOrganizationMiddleware(opts.UserUseCase, logHelper),
				// 3 - Check user/token authorization
				authzMiddleware.WithAuthzMiddleware(opts.Enforcer, logHelper),
			).Match(requireAllButOrganizationOperationsMatcher()).Build(),
			// 4 - Make sure the account is fully functional
			selector.Server(
				usercontext.CheckUserInAllowList(opts.AuthConfig.AllowList),
			).Match(allowListEnabled()).Build(),
			selector.Server(
				usercontext.CheckOrgRequirements(opts.CASBackendUseCase),
			).Match(requireFullyConfiguredOrgMatcher()).Build(),
		).Match(requireCurrentUserMatcher()).Build(),
	)

	// attestation robot account authentication
	middlewares = append(middlewares,
		// if we require a robot account
		selector.Server(
			// 1 - Extract information from the JWT by using the claims
			attjwtmiddleware.WithJWTMulti(
				opts.Logger,
				// Robot account provider
				attjwtmiddleware.NewRobotAccountProvider(opts.AuthConfig.GeneratedJwsHmacSecret),
				// API Token provider
				attjwtmiddleware.NewAPITokenProvider(opts.AuthConfig.GeneratedJwsHmacSecret),
				// Delegated Federated provider
				attjwtmiddleware.WithFederatedProvider(opts.FederatedConfig),
			),
			// 2.a - Set its workflow and organization in the context
			usercontext.WithAttestationContextFromRobotAccount(opts.RobotAccountUseCase, opts.OrganizationUseCase, logHelper),
			// 2.b - Set its API token and Robot Account as alternative to the user
			usercontext.WithAttestationContextFromAPIToken(opts.APITokenUseCase, opts.OrganizationUseCase, logHelper),
			// 2.c - Set its robot account from federated delegation
			usercontext.WithAttestationContextFromFederatedInfo(opts.OrganizationUseCase, logHelper),
		).Match(requireRobotAccountMatcher()).Build(),
	)

	// Include the Sentry Context Interceptor
	middlewares = append(middlewares, sentrycontext.NewSentryContext())

	return middlewares
}

// If we should load the user
func requireCurrentUserMatcher() selector.MatchFunc {
	// Skip authentication on the status grpc service
	const skipRegexp = "(controlplane.v1.AttestationService/.*|controlplane.v1.StatusService/.*|controlplane.v1.ReferrerService/DiscoverPublicShared|controlplane.v1.AttestationStateService|controlplane.v1.SigningService)"
	return func(ctx context.Context, operation string) bool {
		r := regexp.MustCompile(skipRegexp)
		return !r.MatchString(operation)
	}
}

func requireFullyConfiguredOrgMatcher() selector.MatchFunc {
	// We do not need to remove other endpoints since this matcher is called once the requireCurrentUserMatcher one has passed
	const skipRegexp = "controlplane.v1.OCIRepositoryService/.*|controlplane.v1.ContextService/Current|/controlplane.v1.OrganizationService/.*|/controlplane.v1.AuthService/DeleteAccount|controlplane.v1.CASBackendService/.*|/controlplane.v1.UserService/.*|controlplane.v1.SigningService/.*"
	return func(ctx context.Context, operation string) bool {
		r := regexp.MustCompile(skipRegexp)
		return !r.MatchString(operation)
	}
}

func allowListEnabled() selector.MatchFunc {
	// the allow list should not affect the ability to know who you are and delete your account
	const skipRegexp = "controlplane.v1.ContextService/Current|/controlplane.v1.AuthService/DeleteAccount"
	return func(ctx context.Context, operation string) bool {
		r := regexp.MustCompile(skipRegexp)
		return !r.MatchString(operation)
	}
}

func requireRobotAccountMatcher() selector.MatchFunc {
	const requireMatcher = "controlplane.v1.AttestationService/.*|controlplane.v1.AttestationStateService/.*|controlplane.v1.SigningService/GenerateSigningCert"
	return func(ctx context.Context, operation string) bool {
		r := regexp.MustCompile(requireMatcher)
		return r.MatchString(operation)
	}
}

// Matches all operations that require to have a current organization
func requireAllButOrganizationOperationsMatcher() selector.MatchFunc {
	const skipRegexp = "/controlplane.v1.OrganizationService/Create|/controlplane.v1.UserService/ListMemberships|/controlplane.v1.ContextService/Current|/controlplane.v1.AuthService/DeleteAccount"
	return func(ctx context.Context, operation string) bool {
		r := regexp.MustCompile(skipRegexp)
		return !r.MatchString(operation)
	}
}
