//
// Copyright 2023-2025 The Chainloop Authors.
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

package service

import (
	"context"
	"io"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/google/wire"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewWorkflowService,
	NewAuthService,
	NewRobotAccountService,
	NewWorkflowRunService,
	NewAttestationService,
	NewWorkflowSchemaService,
	NewCASCredentialsService,
	NewContextService,
	NewOrgMetricsService,
	NewIntegrationsService,
	NewCASBackendService,
	NewCASRedirectService,
	NewOrganizationService,
	NewOrgInvitationService,
	NewReferrerService,
	NewAPITokenService,
	NewAttestationStateService,
	NewUserService,
	NewSigningService,
	NewPrometheusService,
	NewGroupService,
	NewProjectService,
	wire.Struct(new(NewWorkflowRunServiceOpts), "*"),
	wire.Struct(new(NewAttestationServiceOpts), "*"),
	wire.Struct(new(NewAttestationStateServiceOpt), "*"),
)

func requireCurrentUser(ctx context.Context) (*entities.User, error) {
	currentUser := entities.CurrentUser(ctx)
	if currentUser == nil {
		return nil, errors.NotFound("not found", "logged in user")
	}

	return currentUser, nil
}

func requireAPIToken(ctx context.Context) (*entities.APIToken, error) {
	token := entities.CurrentAPIToken(ctx)
	if token == nil {
		return nil, errors.NotFound("not found", "API token")
	}

	return token, nil
}

func requireCurrentUserOrAPIToken(ctx context.Context) (*entities.User, *entities.APIToken, error) {
	user, err := requireCurrentUser(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return nil, nil, err
	}

	apiToken, err := requireAPIToken(ctx)
	if err != nil && !errors.IsNotFound(err) {
		return nil, nil, err
	}

	if user == nil && apiToken == nil {
		return nil, nil, errors.Forbidden("authN required", "logged in user nor API token found")
	}

	return user, apiToken, nil
}

func requireCurrentOrg(ctx context.Context) (*entities.Org, error) {
	currentOrg := entities.CurrentOrg(ctx)
	if currentOrg == nil {
		return nil, errors.NotFound("not found", "current organization not set")
	}

	return currentOrg, nil
}

func requireCurrentAuthzSubject(ctx context.Context) (string, error) {
	sub := usercontext.CurrentAuthzSubject(ctx)
	if sub == "" {
		return "", errors.NotFound("not found", "authorization subject not set")
	}

	return sub, nil
}

func newService(opts ...NewOpt) *service {
	s := &service{
		log: log.NewHelper(log.NewStdLogger(io.Discard)),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

type service struct {
	log            *log.Helper
	enforcer       *authz.Enforcer
	projectUseCase *biz.ProjectUseCase
}

type NewOpt func(s *service)

func WithLogger(logger log.Logger) NewOpt {
	return func(s *service) {
		s.log = servicelogger.ScopedHelper(logger, "service")
	}
}

func WithEnforcer(enforcer *authz.Enforcer) NewOpt {
	return func(s *service) {
		s.enforcer = enforcer
	}
}

func WithProjectUseCase(projectUseCase *biz.ProjectUseCase) NewOpt {
	return func(s *service) {
		s.projectUseCase = projectUseCase
	}
}

// authorizeResource is a helper that checks if the user has a particular `op` permission policy on a particular resource
// For example: `s.authorizeResource(ctx, authz.PolicyAttachedIntegrationDetach, authz.ResourceTypeProject, projectUUID);`
// checks if the user has a role in the project that allows to detach integrations on it.
// This method is available to every service that embeds `service`
func (s *service) authorizeResource(ctx context.Context, op *authz.Policy, resourceType authz.ResourceType, resourceID uuid.UUID) error {
	if !rbacEnabled(ctx) {
		return nil
	}

	// Apply RBAC
	m := entities.CurrentMembership(ctx)
	// check for specific resource role
	for _, rm := range m.Resources {
		if rm.ResourceType == resourceType && rm.ResourceID == resourceID {
			pass, err := s.enforcer.Enforce(string(rm.Role), op)
			if err != nil {
				return handleUseCaseErr(err, s.log)
			}
			if !pass {
				return errors.Forbidden("forbidden", "operation not allowed")
			}
			return nil
		}
	}
	return errors.Forbidden("forbidden", "operation not allowed")
}

// userHasPermissionOnProject is a helper method that checks if a policy can be applied to a project. It looks for a project
// by name and ensures that the user has a role that allows that specific operation in the project.
// check authorizeResource method
func (s *service) userHasPermissionOnProject(ctx context.Context, orgID string, pName string, policy *authz.Policy) error {
	if !rbacEnabled(ctx) {
		return nil
	}

	p, err := s.projectUseCase.FindProjectByReference(ctx, orgID, &biz.EntityRef{Name: pName})
	if err != nil {
		return handleUseCaseErr(err, s.log)
	}

	return s.authorizeResource(ctx, policy, authz.ResourceTypeProject, p.ID)
}

// visibleProjects returns projects where the user has any role (currently ProjectAdmin and ProjectViewer)
func (s *service) visibleProjects(ctx context.Context) []uuid.UUID {
	if !rbacEnabled(ctx) {
		// returning a NIL slice to denote that RBAC has not been applied, to differentiate from the empty slice case
		return nil
	}

	projects := make([]uuid.UUID, 0)

	m := entities.CurrentMembership(ctx)
	for _, rm := range m.Resources {
		if rm.ResourceType == authz.ResourceTypeProject {
			projects = append(projects, rm.ResourceID)
		}
	}

	return projects
}

// RBAC feature is enabled if the user has the `Org Member` role.
func rbacEnabled(ctx context.Context) bool {
	return usercontext.CurrentAuthzSubject(ctx) == string(authz.RoleOrgMember)
}

// NOTE: some of these http errors get automatically translated to gRPC status codes
// because they implement the gRPC status error interface
// so it is safe to return either a gRPC status error or a kratos error
func handleUseCaseErr(err error, l *log.Helper) error {
	switch {
	case errors.Is(err, context.Canceled):
		return errors.ClientClosed("client closed", err.Error())
	case biz.IsErrValidation(err) || biz.IsErrInvalidUUID(err) || biz.IsErrInvalidTimeWindow(err) ||
		pagination.IsOffsetPaginationError(err) || pagination.IsCursorPaginationError(err):
		return errors.BadRequest("invalid", err.Error())
	case biz.IsNotFound(err):
		return errors.NotFound("not found", err.Error())
	case biz.IsErrUnauthorized(err):
		return errors.Forbidden("unauthorized", err.Error())
	case biz.IsErrNotImplemented(err):
		return status.Error(codes.Unimplemented, err.Error())
	case biz.IsErrAlreadyExists(err):
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return servicelogger.LogAndMaskErr(err, l)
	}
}
