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
	"fmt"
	"io"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"

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
// It goes through all the memberships of the user, direct memberships and indirect memberships (Groups)
// and checks if the user has any role that allows the operation on the resourceType and resourceID.
func (s *service) authorizeResource(ctx context.Context, op *authz.Policy, resourceType authz.ResourceType, resourceID uuid.UUID) error {
	if !rbacEnabled(ctx) {
		return nil
	}

	// 1 - Authorize using API token
	// For now we only support API tokens to authorize project resourceTypes
	// NOTE we do not run s.enforcer here because API tokens do not have roles associated with resourceTypes
	// the authorization has happened at the API level and we do not have attribute-based policies in casbin yet
	if token := entities.CurrentAPIToken(ctx); token != nil {
		if resourceType == authz.ResourceTypeProject && token.ProjectID != nil && token.ProjectID.String() == resourceID.String() {
			s.log.Debugw("msg", "authorized using API token", "resource_id", resourceID.String(), "resource_type", resourceType, "token_name", token.Name, "token_id", token.ID)
			return nil
		}

		return errors.Forbidden("forbidden", fmt.Errorf("operation not allowed: This auth token is valid only with the project %q", *token.ProjectName).Error())
	}

	// 2 - We are a user
	// find the resource membership that matches the resource type and ID
	// for example admin in project1, then apply RBAC enforcement
	orgRole := usercontext.CurrentAuthzSubject(ctx)
	m := entities.CurrentMembership(ctx)

	// iterate through all resource memberships and find any that matches
	for _, rm := range m.Resources {
		if rm.ResourceType == resourceType && rm.ResourceID == resourceID &&
			// Org Viewers cannot become Project Admins. Skipping this item in case it's inherited from a group
			// nolint:staticcheck
			!(orgRole == string(authz.RoleViewer) && rm.Role == authz.RoleProjectAdmin) {

			pass, err := s.enforcer.Enforce(string(rm.Role), op)
			if err != nil {
				return handleUseCaseErr(err, s.log)
			}

			if pass {
				s.log.Debugw("msg", "authorized using user membership", "resource_id", resourceID.String(), "resource_type", resourceType, "role", rm.Role, "membership_id", rm.MembershipID, "user_id", m.UserID)
				return nil
			}
		}
	}

	var defaultMessage = fmt.Sprintf("you do not have permissions to access to the %s associated with this resource", resourceType)

	// If none of the roles pass, return forbidden error
	return errors.Forbidden("forbidden", defaultMessage)
}

// userHasPermissionOnProject is a helper method that checks if a policy can be applied to a project. It looks for a project
// by name in the given organization and ensures that the user has a role that allows that specific operation in the project.
// check authorizeResource method
// if it doesn't return an error, it means that the user has the permission and the project is returned
func (s *service) userHasPermissionOnProject(ctx context.Context, orgID string, ref *pb.IdentityReference, policy *authz.Policy) (*biz.Project, error) {
	// Parse entity ID and entity Name from the request
	entityID, entityName, err := ref.Parse()
	if err != nil {
		return nil, errors.BadRequest("invalid", fmt.Sprintf("invalid project reference: %s", err.Error()))
	}

	// Find the project by its reference
	p, err := s.projectUseCase.FindProjectByReference(ctx, orgID, &biz.IdentityReference{ID: entityID, Name: entityName})
	if err != nil {
		return nil, handleUseCaseErr(err, s.log)
	}

	// if RBAC is not enabled, we return the project
	if !rbacEnabled(ctx) {
		return p, nil
	}

	if err = s.authorizeResource(ctx, policy, authz.ResourceTypeProject, p.ID); err != nil {
		return nil, err
	}

	return p, nil
}

// visibleProjects returns projects where the user has any role (currently ProjectAdmin and ProjectViewer)
func (s *service) visibleProjects(ctx context.Context) []uuid.UUID {
	if !rbacEnabled(ctx) {
		// returning a NIL slice to denote that RBAC has not been applied, to differentiate from the empty slice case
		return nil
	}

	projects := make([]uuid.UUID, 0)

	// 1 - Check if we are using an API token
	if token := entities.CurrentAPIToken(ctx); token != nil {
		if token.ProjectID != nil {
			projects = append(projects, *token.ProjectID)
		}
		return projects
	}

	// 2 - We are a user
	m := entities.CurrentMembership(ctx)
	for _, rm := range m.Resources {
		if rm.ResourceType == authz.ResourceTypeProject {
			projects = append(projects, rm.ResourceID)
		}
	}

	return projects
}

// initializePaginationOpts initializes the pagination options with the provided request pagination options.
func initializePaginationOpts(reqPagination *pb.OffsetPaginationRequest) (*pagination.OffsetPaginationOpts, error) {
	// Initialize the pagination options, with default values
	paginationOpts := pagination.NewDefaultOffsetPaginationOpts()

	var err error
	// Override the pagination options if they are provided
	if reqPagination != nil {
		paginationOpts, err = pagination.NewOffsetPaginationOpts(
			int(reqPagination.GetPage()),
			int(reqPagination.GetPageSize()),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create pagination options: %w", err)
		}
	}

	return paginationOpts, nil
}

// RBAC feature is enabled if we are using a project scoped token or
// it is a user with org role member
func rbacEnabled(ctx context.Context) bool {
	// it's an API token
	token := entities.CurrentAPIToken(ctx)
	if token != nil {
		return token.ProjectID != nil
	}

	// we have an user
	currentSubject := usercontext.CurrentAuthzSubject(ctx)
	return currentSubject == string(authz.RoleOrgMember)
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
