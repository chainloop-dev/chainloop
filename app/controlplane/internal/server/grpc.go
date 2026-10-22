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
	"crypto/tls"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/sentrycontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	authzMiddleware "github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz/middleware"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/user"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/chainloop-dev/chainloop/pkg/cache"

	"buf.build/go/protovalidate"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/service"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/golang-jwt/jwt/v5"

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
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	grpcLib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Opts struct {
	// UseCases
	AuthzUseCase        *biz.AuthzUseCase
	UserUseCase         *biz.UserUseCase
	RobotAccountUseCase *biz.RobotAccountUseCase
	CASBackendUseCase   *biz.CASBackendUseCase
	CASClientUseCase    *biz.CASClientUseCase
	IntegrationUseCase  *biz.IntegrationUseCase
	ReferrerUseCase     *biz.ReferrerUseCase
	APITokenUseCase     *biz.APITokenUseCase
	OrganizationUseCase *biz.OrganizationUseCase
	WorkflowUseCase     *biz.WorkflowUseCase
	MembershipUseCase   *biz.MembershipUseCase
	MembershipsCache    cache.Cache[*entities.Membership]
	ClaimsCache         cache.Cache[*jwt.MapClaims]
	// Services
	WorkflowSvc         *service.WorkflowService
	AuthSvc             *service.AuthService
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
	GroupSvc            *service.GroupService
	ProjectSvc          *service.ProjectService
	// Utils
	Logger              log.Logger
	ServerConfig        *conf.Server
	AuthConfig          *conf.Auth
	FederatedConfig     *conf.FederatedAuthentication
	OperationAuthConfig *conf.OperationAuthorizationProvider
	BootstrapConfig     *conf.Bootstrap
	Credentials         credentials.ReaderWriter
	Validator           protovalidate.Validator
}

var (
	currentUserSkipRegexp                  = regexp.MustCompile("(controlplane.v1.AttestationService/.*|controlplane.v1.StatusService/.*|/controlplane.v1.AuthService/ExchangeHandoff$|controlplane.v1.AttestationStateService|controlplane.v1.SigningService)")
	fullyConfiguredOrgSkipRegexp           = regexp.MustCompile("controlplane.v1.OCIRepositoryService/.*|controlplane.v1.ContextService/Current|/controlplane.v1.OrganizationService/.*|/controlplane.v1.AuthService/DeleteAccount|controlplane.v1.CASBackendService/.*|/controlplane.v1.UserService/.*|controlplane.v1.SigningService/.*")
	fullyConfiguredCASBackendRequireRegexp = regexp.MustCompile("/controlplane.v1.AttestationService.GetUploadCreds|/controlplane.v1.AttestationService.Init|/controlplane.v1.AttestationService.Store|/controlplane.v1.CASCredentialsService.Get")
	allowListSkipRegexp                    = regexp.MustCompile("controlplane.v1.ContextService/Current|/controlplane.v1.AuthService/DeleteAccount")
	robotAccountRequireRegexp              = regexp.MustCompile("controlplane.v1.AttestationService/.*|controlplane.v1.AttestationStateService/.*|controlplane.v1.SigningService/GenerateSigningCert")
	allButOrganizationOperationsSkipRegexp = regexp.MustCompile("/controlplane.v1.OrganizationService/Create|/controlplane.v1.UserService/ListMemberships|/controlplane.v1.ContextService/Current|/controlplane.v1.AuthService/DeleteAccount")
)

// GRPCServers separates the HTTP handler from the optional native listener.
// Only Legacy must be started as a transport server.
type GRPCServers struct {
	Handler *grpc.Server
	Legacy  *grpc.Server
}

// NewGRPCServers assembles the HTTP handler and optional native listener.
func NewGRPCServers(opts *Opts) (*GRPCServers, error) {
	servers := &GRPCServers{Handler: newGRPCHandler(opts)}
	if legacyGRPCListenerEnabled(opts.ServerConfig) {
		// Ignore unused legacy certificates when this listener is disabled.
		legacy, err := NewGRPCServer(opts)
		if err != nil {
			return nil, err
		}
		servers.Legacy = legacy
	}
	return servers, nil
}

// newGRPCHandler builds the handler served on the shared HTTP endpoint.
func newGRPCHandler(opts *Opts) *grpc.Server {
	// Kratos' stream interceptor needs a non-nil endpoint even when the gRPC
	// handler never starts a native listener. Supply it explicitly rather than
	// calling Endpoint(), which would bind an otherwise unused socket.
	scheme := "grpc"
	if opts.ServerConfig.GetHttp().GetTlsConfig().GetCertificate() != "" {
		scheme = "grpcs"
	}
	return newGRPCServer(opts, grpc.Endpoint(&url.URL{
		Scheme: scheme,
		Host:   opts.ServerConfig.GetHttp().GetAddr(),
	}))
}

func legacyGRPCListenerEnabled(c *conf.Server) bool {
	return c.LegacyGrpcListener == nil || *c.LegacyGrpcListener
}

// NewGRPCServer builds the native gRPC listener with its existing configuration.
// A partial TLS configuration keeps this listener plaintext, as it always did.
func NewGRPCServer(opts *Opts) (*grpc.Server, error) {
	var serverOpts []grpc.ServerOption
	if v := opts.ServerConfig.Grpc.Network; v != "" {
		serverOpts = append(serverOpts, grpc.Network(v))
	}
	if v := opts.ServerConfig.Grpc.Addr; v != "" {
		serverOpts = append(serverOpts, grpc.Address(v))
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
	return newGRPCServer(opts, serverOpts...), nil
}

// newGRPCServer builds one gRPC server with the shared middleware chain,
// interceptors and service registrations, bound through the given extra
// options.
func newGRPCServer(opts *Opts, extraOpts ...grpc.ServerOption) *grpc.Server {
	// Opt-in histogram metrics for the interceptor.
	grpc_prometheus.EnableHandlingTimeHistogram()

	var serverOpts = append([]grpc.ServerOption{
		grpc.Middleware(craftMiddleware(opts)...),
		grpc.UnaryInterceptor(
			protovalidateMiddleware.UnaryServerInterceptor(opts.Validator),
		),
		// The control plane implements only unary RPCs. Reject any streaming
		// method other than gRPC reflection so the server does not dispatch
		// streaming methods it does not serve. Reflection is a local-development
		// convenience (e.g. grpcurl) and is disabled in production below.
		grpc.StreamInterceptor(rejectUnsupportedStreams),
	}, extraOpts...)

	// gRPC reflection advertises the already-public API schema and exists as a
	// convenience for local debugging tooling such as grpcurl. No product
	// client depends on it, so register it in development only; production
	// disables it.
	if Version != "dev" {
		serverOpts = append(serverOpts, grpc.DisableReflection())
	}

	if v := opts.ServerConfig.Grpc.Timeout; v != nil {
		serverOpts = append(serverOpts, grpc.Timeout(v.AsDuration()))
	}

	// Raw grpc-go server options must be passed in a single grpc.Options call:
	// kratos' grpc.Options assigns s.grpcOpts instead of appending, so a second
	// call (e.g. for MaxRecvMsgSize) would silently wipe the OTel stats handler.
	grpcOpts := []grpcLib.ServerOption{grpcLib.StatsHandler(otelgrpc.NewServerHandler())}
	if v := opts.ServerConfig.Grpc.GetMaxRecvMsgSize(); v > 0 {
		grpcOpts = append(grpcOpts, grpcLib.MaxRecvMsgSize(int(v)))
	}
	serverOpts = append(serverOpts, grpc.Options(grpcOpts...))

	srv := grpc.NewServer(serverOpts...)
	v1.RegisterWorkflowServiceServer(srv, opts.WorkflowSvc)
	v1.RegisterStatusServiceServer(srv, service.NewStatusService(opts.AuthSvc.AuthURLs.Login, Version, opts.CASClientUseCase, opts.BootstrapConfig))
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
	v1.RegisterGroupServiceServer(srv, opts.GroupSvc)
	v1.RegisterProjectServiceServer(srv, opts.ProjectSvc)

	// Register Prometheus metrics
	grpc_prometheus.Register(srv.Server)

	return srv
}

// rejectUnsupportedStreams blocks streaming RPCs that the control plane does not
// implement. The server exposes only unary RPCs; gRPC reflection is the single
// streaming service it serves (a local-development convenience, disabled in
// production). Any other streaming method returns Unimplemented instead of being
// dispatched to a handler.
func rejectUnsupportedStreams(srv interface{}, ss grpcLib.ServerStream, info *grpcLib.StreamServerInfo, handler grpcLib.StreamHandler) error {
	if strings.HasPrefix(info.FullMethod, "/grpc.reflection.") {
		return handler(srv, ss)
	}
	return status.Error(codes.Unimplemented, "streaming methods are not supported")
}

func craftMiddleware(opts *Opts) []middleware.Middleware {
	middlewares := []middleware.Middleware{
		// Give each request its own Sentry hub before anything can report to Sentry
		sentrycontext.NewSentryHub(),
		recovery.Recovery(
			recovery.WithHandler(func(ctx context.Context, _, err interface{}) error {
				// The hub is set by NewSentryHub, so the panic carries the scope of its request
				servicelogger.SentryHub(ctx).CaptureMessage(fmt.Sprintf("%v", err))
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
			// 2.b - Update API Token last usage
			usercontext.WithAPITokenUsageUpdater(opts.APITokenUseCase, logHelper),
			// 2.c - Set its user
			usercontext.WithCurrentUserMiddleware(opts.UserUseCase, logHelper),
			// Store all memberships in the context
			usercontext.WithCurrentMembershipsMiddleware(opts.MembershipUseCase, opts.MembershipsCache),
			// Operation authorization forward
			usercontext.WithOperationAuthorizationMiddleware(opts.OperationAuthConfig, logHelper),
			selector.Server(
				// 2.d- Set its organization
				usercontext.WithCurrentOrganizationMiddleware(opts.UserUseCase, opts.OrganizationUseCase, logHelper),
				// 2.e- Block all operations on suspended orgs
				usercontext.WithSuspensionMiddleware(),
				// 3 - Check user/token authorization
				authzMiddleware.WithAuthzMiddleware(opts.AuthzUseCase, logHelper),
			).Match(requireAllButOrganizationOperationsMatcher()).Build(),
			// 4 - Make sure the account is fully functional
			selector.Server(
				usercontext.CheckUserHasAccess(opts.AuthConfig.AllowList, opts.UserUseCase),
			).Match(allowListEnabled()).Build(),
			selector.Server(
				usercontext.ValidateCASBackend(opts.CASBackendUseCase),
			).Match(requireFullyConfiguredOrgMatcher()).Build(),
		).Match(requireCurrentUserMatcher()).Build(),
	)

	// attestation branch
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
				// User Token provider
				attjwtmiddleware.NewUserTokenProvider(opts.AuthConfig.GeneratedJwsHmacSecret),
				// Delegated Federated provider
				attjwtmiddleware.WithFederatedProvider(opts.FederatedConfig),
				attjwtmiddleware.WithClaimsCache(opts.ClaimsCache),
			),
			// 2.a - Set its workflow and organization in the context
			usercontext.WithAttestationContextFromRobotAccount(opts.RobotAccountUseCase, opts.OrganizationUseCase, logHelper),
			// 2.b - Set its API token and Robot Account as alternative to the user
			usercontext.WithAttestationContextFromAPIToken(opts.APITokenUseCase, opts.OrganizationUseCase, logHelper),
			// 2.c - Set Attestation context from user token
			usercontext.WithAttestationContextFromUser(opts.UserUseCase, opts.OrganizationUseCase, logHelper),
			// 2.d - Set its robot account from federated delegation
			usercontext.WithAttestationContextFromFederatedInfo(opts.OrganizationUseCase, logHelper),
			// Store all memberships in the context
			usercontext.WithCurrentMembershipsMiddleware(opts.MembershipUseCase, opts.MembershipsCache),
			// 2.e - Block all operations on suspended orgs
			usercontext.WithSuspensionMiddleware(),
			// 3 - Update API Token last usage
			usercontext.WithAPITokenUsageUpdater(opts.APITokenUseCase, logHelper),
			// 4 - Validate the CAS Backend is fully configured and valid
			selector.Server(
				usercontext.ValidateCASBackend(opts.CASBackendUseCase),
				usercontext.BlockIfCASBackendNotValid(opts.CASBackendUseCase),
			).Match(requireFullyConfiguredCASBackendMatcher()).Build(),
		).Match(requireRobotAccountMatcher()).Build(),
	)

	// Include the Sentry Context Interceptor
	middlewares = append(middlewares, sentrycontext.NewSentryContext())

	return middlewares
}

// If we should load the user
func requireCurrentUserMatcher() selector.MatchFunc {
	return func(_ context.Context, operation string) bool {
		return !currentUserSkipRegexp.MatchString(operation)
	}
}

func requireFullyConfiguredOrgMatcher() selector.MatchFunc {
	return func(_ context.Context, operation string) bool {
		return !fullyConfiguredOrgSkipRegexp.MatchString(operation)
	}
}

func requireFullyConfiguredCASBackendMatcher() selector.MatchFunc {
	return func(_ context.Context, operation string) bool {
		return fullyConfiguredCASBackendRequireRegexp.MatchString(operation)
	}
}

func allowListEnabled() selector.MatchFunc {
	return func(_ context.Context, operation string) bool {
		return !allowListSkipRegexp.MatchString(operation)
	}
}

func requireRobotAccountMatcher() selector.MatchFunc {
	return func(_ context.Context, operation string) bool {
		return robotAccountRequireRegexp.MatchString(operation)
	}
}

// Matches all operations that require to have a current organization
func requireAllButOrganizationOperationsMatcher() selector.MatchFunc {
	return func(_ context.Context, operation string) bool {
		return !allButOrganizationOperationsSkipRegexp.MatchString(operation)
	}
}
