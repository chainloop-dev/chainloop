//
// Copyright 2023-2026 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
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
	authz          *biz.AuthzUseCase
	projectUseCase *biz.ProjectUseCase
	groupUseCase   *biz.GroupUseCase
}

type NewOpt func(s *service)

func WithLogger(logger log.Logger) NewOpt {
	return func(s *service) {
		s.log = servicelogger.ScopedHelper(logger, "service")
	}
}

func WithEnforcer(authzUC *biz.AuthzUseCase) NewOpt {
	return func(s *service) {
		s.authz = authzUC
	}
}

func WithProjectUseCase(projectUseCase *biz.ProjectUseCase) NewOpt {
	return func(s *service) {
		s.projectUseCase = projectUseCase
	}
}

func WithGroupUseCase(groupUseCase *biz.GroupUseCase) NewOpt {
	return func(s *service) {
		s.groupUseCase = groupUseCase
	}
}

type authorizeResourceOpts struct {
	forceRBAC bool
}

type AuthorizeResourceOpt func(*authorizeResourceOpts)

// withForceRBAC forces RBAC checks even for admin roles that would normally skip RBAC
func withForceRBAC() AuthorizeResourceOpt {
	return func(opts *authorizeResourceOpts) {
		opts.forceRBAC = true
	}
}

// authorizeResource is a helper that checks if the user has a particular `op` permission policy on a particular resource
// For example: `s.authorizeResource(ctx, authz.PolicyAttachedIntegrationDetach, authz.ResourceTypeProject, projectUUID);`
// checks if the user has a role in the project that allows to detach integrations on it.
// This method is available to every service that embeds `service`
// It goes through all the memberships of the user, direct memberships and indirect memberships (Groups)
// and checks if the user has any role that allows the operation on the resourceType and resourceID.
func (s *service) authorizeResource(ctx context.Context, op *authz.Policy, resourceType authz.ResourceType, resourceID uuid.UUID, opts ...AuthorizeResourceOpt) error {
	options := &authorizeResourceOpts{}
	for _, opt := range opts {
		opt(options)
	}

	if !options.forceRBAC && !rbacEnabled(ctx) {
		return nil
	}

	// 1 - Authorize using an API token confined to a single project
	// NOTE we do not run s.enforcer here because such tokens do not have roles associated with
	// resourceTypes: the authorization has happened at the API level and we do not have
	// attribute-based policies in casbin yet. A token confined to a resource outside this
	// database has no such column to answer from, so it falls through to the membership walk
	// below, exactly as a person does.
	if token := entities.CurrentAPIToken(ctx); token != nil && !token.IsResourceScoped() {
		if resourceType == authz.ResourceTypeProject && token.ProjectID != nil && *token.ProjectID == resourceID {
			s.log.Debugw("msg", "authorized using API token", "resource_id", resourceID.String(), "resource_type", resourceType, "token_name", token.Name, "token_id", token.ID)
			return nil
		}

		// ProjectName is nil for any token not confined to a project, so never dereference
		// it blind: a panic recovered into a 500 tells the caller nothing.
		confinedTo := "this organization"
		if token.ProjectName != nil {
			confinedTo = fmt.Sprintf("the project %q", *token.ProjectName)
		}

		return errors.Forbidden("forbidden", fmt.Sprintf("operation not allowed: this auth token is valid only with %s", confinedTo))
	}

	var defaultMessage = fmt.Sprintf("you do not have permissions to access the %q with id %q", resourceType, resourceID.String())
	// 2 - We are a user, or a token scoped to a resource outside this database.
	// find the resource membership that matches the resource type and ID
	// for example admin in project1, then apply RBAC enforcement
	m := entities.CurrentMembership(ctx)
	if m == nil {
		return errors.Forbidden("forbidden", defaultMessage)
	}

	// A scoped token authorizes from a membership the Chainloop platform wrote, so the role on
	// that membership must never widen what the token itself carries. The attestation
	// endpoints are skipped by the authz middleware, making this the only place the token's
	// ACL is consulted on that path — and RoleProjectAdmin grants PolicyAPITokenCreate and
	// PolicyAPITokenRevoke, precisely the organization-level policies a scoped token is
	// denied. On the paths the middleware covers this check is a no-op, and it keeps
	// authorizeResource's answer in step with projectsAllowing, which enforces the same
	// subject.
	if token := entities.CurrentAPIToken(ctx); token != nil {
		allowed, err := s.authz.Enforce(ctx, usercontext.CurrentAuthzSubject(ctx), op)
		if err != nil {
			return handleUseCaseErr(err, s.log)
		}

		if !allowed {
			return errors.Forbidden("forbidden", defaultMessage)
		}
	}

	var matchingResources []*entities.ResourceMembership
	var foundRoles []string
	// First, collect all memberships that match the requested resource type and ID
	for _, rm := range m.Resources {
		if rm.ResourceType == resourceType && rm.ResourceID == resourceID {
			matchingResources = append(matchingResources, rm)
			foundRoles = append(foundRoles, string(rm.Role))
		}
	}

	// If no matching resources were found, return forbidden error
	if len(matchingResources) == 0 {
		// A scoped token is refused for a reason worth naming: the resource is outside its
		// scope, not a permission it lacks.
		if token := entities.CurrentAPIToken(ctx); token.IsResourceScoped() {
			return errors.Forbidden("forbidden", outOfScopeMessage(token, resourceType))
		}

		return errors.Forbidden("forbidden", defaultMessage)
	}

	defaultMessage = fmt.Sprintf("%s, roles=%v", defaultMessage, foundRoles)

	// Try to enforce the policy with each matching role
	// If any role passes, authorize the request
	for _, rm := range matchingResources {
		pass, err := s.authz.Enforce(ctx, string(rm.Role), op)
		if err != nil {
			return handleUseCaseErr(err, s.log)
		}

		if pass {
			s.log.Debugw("msg", "authorized using membership", "resource_id", resourceID.String(), "resource_type", resourceType, "role", rm.Role, "membership_id", rm.MembershipID, "member_type", m.MemberType, "member_id", m.MemberID)
			return nil
		}
	}

	// If none of the roles pass, return forbidden error
	return errors.Forbidden("forbidden", defaultMessage)
}

// outOfScopeMessage explains that the token's scope does not include the resource, so a
// refused caller can tell that apart from a permission it lacks. The scope name is
// display-only and may be stale after a rename, hence the fallback to the id.
func outOfScopeMessage(token *entities.APIToken, resourceType authz.ResourceType) string {
	// The name is written by the Chainloop platform, so do not trust it to be present: an
	// empty one is as good as absent and the id is what actually identifies the scope.
	name := token.ScopeID.String()
	if token.ScopeName != nil && *token.ScopeName != "" {
		name = *token.ScopeName
	}

	kind := "resource"
	if token.Scope != nil {
		kind = string(*token.Scope)
	}

	return fmt.Sprintf("operation not allowed: this token is scoped to %s %q, which does not include this %s", kind, name, resourceType)
}

// projectsAllowing reports, per project ID, whether the caller may perform op
// on it. It answers for a whole listing in one pass, so a client does not have
// to offer an action that would be refused the moment it is taken.
//
// It mirrors the two checks the operation itself would face. First the
// API-level one in the authz middleware, against the caller's organization
// role. Then, when RBAC applies, authorizeResource: a caller can hold several
// roles on the same project, directly and through products, and any one of them
// granting the permission is enough. Each distinct role is enforced once, not
// once per project.
func (s *service) projectsAllowing(ctx context.Context, op *authz.Policy, projects []*biz.Project) (map[uuid.UUID]bool, error) {
	allowed := make(map[uuid.UUID]bool, len(projects))

	// The organization role gates every path, so a role without the permission
	// allows nothing. An organization viewer, for instance, may list projects
	// but not create a workflow in any of them. Administrators skip the policy
	// check here for the same reason the middleware skips it: their policies are
	// not spelled out in full yet.
	subject := usercontext.CurrentAuthzSubject(ctx)
	if !authz.Role(subject).IsAdmin() {
		orgAllows, err := s.authz.Enforce(ctx, subject, op)
		if err != nil {
			return nil, err
		}

		if !orgAllows {
			return allowed, nil
		}
	}

	// The organization role carries the permission and RBAC does not narrow it
	// per project, so every visible project is fair game.
	if !rbacEnabled(ctx) {
		for _, p := range projects {
			allowed[p.ID] = true
		}

		return allowed, nil
	}

	// An API token confined to a single project answers from the token itself, and reaching
	// here means the API-level check already accepted the operation for it. A token scoped to
	// a resource outside this database falls through to the membership walk below, the same
	// way authorizeResource does — the two must agree, or a listing offers an action the call
	// refuses, or hides one it would allow.
	if token := entities.CurrentAPIToken(ctx); token != nil && !token.IsResourceScoped() {
		for _, p := range projects {
			allowed[p.ID] = token.ProjectID != nil && *token.ProjectID == p.ID
		}

		return allowed, nil
	}

	m := entities.CurrentMembership(ctx)
	if m == nil {
		return allowed, nil
	}

	// roleGrants caches the enforcer's answer per role, so a caller with many
	// project memberships still enforces each distinct role only once.
	roleGrants := make(map[authz.Role]bool)
	for _, rm := range m.Resources {
		if rm.ResourceType != authz.ResourceTypeProject {
			continue
		}

		if _, seen := roleGrants[rm.Role]; !seen {
			grants, err := s.authz.Enforce(ctx, string(rm.Role), op)
			if err != nil {
				return nil, err
			}

			roleGrants[rm.Role] = grants
		}

		if roleGrants[rm.Role] {
			allowed[rm.ResourceID] = true
		}
	}

	return allowed, nil
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

func (s *service) userCanCreateProject(ctx context.Context) error {
	pass, err := s.canCreateProject(ctx)
	if err != nil {
		return handleUseCaseErr(err, s.log)
	}

	if !pass {
		return errors.Forbidden("unauthorized", "you are not allowed to create projects")
	}

	return nil
}

// canCreateProject reports what userCanCreateProject enforces, so a listing can
// tell a client whether creating one is worth offering. The two must answer
// alike: an option that the create call then refuses is the dead end the answer
// exists to avoid. Its error is raw, for the caller to convert at the boundary.
func (s *service) canCreateProject(ctx context.Context) (bool, error) {
	if !rbacEnabled(ctx) {
		// An API token outside RBAC acts for the whole organization.
		if token := entities.CurrentAPIToken(ctx); token != nil {
			return true, nil
		}

		// The roles outside RBAC are the administrators and the organization
		// viewer, and only the former may create anything. Answering yes for a
		// viewer would offer an option the API-level check refuses.
		return authz.Role(usercontext.CurrentAuthzSubject(ctx)).IsAdmin(), nil
	}

	// Only org tokens can create projects. A token confined to a project or to a resource
	// outside this database is refused explicitly, rather than relying on its policy list
	// happening not to carry PolicyProjectCreate.
	if token := entities.CurrentAPIToken(ctx); token != nil && !token.IsOrgWide() {
		return false, nil
	}

	orgRole := usercontext.CurrentAuthzSubject(ctx)

	return s.authz.Enforce(ctx, orgRole, authz.PolicyProjectCreate)
}

// visibleProjects returns projects where the user has any role (currently ProjectAdmin and ProjectViewer)
func (s *service) visibleProjects(ctx context.Context) []uuid.UUID {
	if !rbacEnabled(ctx) {
		// returning a NIL slice to denote that RBAC has not been applied, to differentiate from the empty slice case
		return nil
	}

	projects := make([]uuid.UUID, 0)

	// 1 - An API token confined to a single project answers from the token itself
	if token := entities.CurrentAPIToken(ctx); token != nil && !token.IsResourceScoped() {
		if token.ProjectID != nil {
			projects = append(projects, *token.ProjectID)
		}
		return projects
	}

	// 2 - A user, or a token scoped to a resource outside this database: both answer from
	// their memberships
	m := entities.CurrentMembership(ctx)
	if m == nil {
		return projects
	}

	for _, rm := range m.Resources {
		if rm.ResourceType == authz.ResourceTypeProject {
			projects = append(projects, rm.ResourceID)
		}
	}

	return projects
}

// rbacScopesForOrg returns the resource-level RBAC scopes of the current caller within the given
// organization, ready to be passed to the use cases that honour them. When RBAC applies to the
// caller the organization is present in the result, limited to the caller's visible projects;
// when it does not (legacy roles, org-scoped API tokens) the organization is absent from the
// result, meaning the whole organization is reachable.
func (s *service) rbacScopesForOrg(ctx context.Context, orgID uuid.UUID) biz.RBACScopes {
	scopes := make(biz.RBACScopes)
	if visibleProjects := s.visibleProjects(ctx); visibleProjects != nil {
		scopes[orgID] = biz.RBACScope{ProjectIDs: visibleProjects}
	}

	return scopes
}

// checkPolicy Checks a policy against a user or a token
func (s *service) checkPolicy(ctx context.Context, policy *authz.Policy) error {
	// Token case
	sub := usercontext.CurrentAuthzSubject(ctx)
	if sub != "" {
		ok, err := s.authz.Enforce(ctx, sub, policy)
		if err != nil {
			return handleUseCaseErr(err, s.log)
		}
		if ok {
			return nil
		}
	}

	// Other cases
	m := entities.CurrentMembership(ctx)
	if m == nil {
		return errors.Forbidden("forbidden", "not allowed")
	}
	for _, rm := range m.Resources {
		pass, err := s.authz.Enforce(ctx, string(rm.Role), authz.PolicyOrganizationCreate)
		if err != nil {
			return handleUseCaseErr(err, s.log)
		}
		if pass {
			return nil
		}
	}

	return errors.Forbidden("forbidden", "not allowed")
}

// isUserOrgAdmin checks if the current user is an org admin or owner
func isUserOrgAdmin(ctx context.Context) bool {
	userRole := usercontext.CurrentAuthzSubject(ctx)
	return authz.Role(userRole).IsAdmin()
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

// RBAC applies to a token confined to a project or to a resource outside this database, and
// to a user whose organization role has it enabled.
func rbacEnabled(ctx context.Context) bool {
	// it's an API token
	token := entities.CurrentAPIToken(ctx)
	if token != nil {
		return !token.IsOrgWide()
	}

	// we have an user
	currentSubject := usercontext.CurrentAuthzSubject(ctx)
	return authz.Role(currentSubject).RBACEnabled()
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
	case biz.IsErrReleasedVersionImmutable(err):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		// Client errors already converted by this function can be processed again
		// (e.g. AttestationService.Store wraps storeAttestation, which converts internally).
		// Propagate them instead of masking and reporting them to Sentry.
		// We extract the status via GRPCStatus() instead of status.FromError because the latter
		// rewrites the message of wrapped errors with the "rpc error: ..." prefix.
		var gs interface{ GRPCStatus() *status.Status }
		if errors.As(err, &gs) {
			if s := gs.GRPCStatus(); isClientErrorCode(s.Code()) {
				return s.Err()
			}
		}

		return servicelogger.LogAndMaskErr(err, l)
	}
}

// isClientErrorCode returns true for the gRPC client-error codes that handleUseCaseErr
// produces, making its conversion idempotent: server-side codes keep being masked
func isClientErrorCode(c codes.Code) bool {
	switch c {
	case codes.Canceled, codes.InvalidArgument, codes.NotFound, codes.PermissionDenied,
		codes.Unimplemented, codes.AlreadyExists, codes.FailedPrecondition:
		return true
	default:
		return false
	}
}
