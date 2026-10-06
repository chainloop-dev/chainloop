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

package biz

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// ErrAPITokenClaimsMismatch marks a token whose row disagrees with its signed claims, or whose
// claims are malformed. Something wrote the row or the claims incorrectly. It is a security event,
// not an expired or old credential. The caller sees only this message, so it names no detail.
var ErrAPITokenClaimsMismatch = errors.New("API token could not be verified")

var apiTokenTracer = otelx.Tracer("chainloop-controlplane", "biz/apitoken")

type APITokenJWTConfig struct {
	SymmetricHmacKey string
}

// orgLevelTokenPolicies are granted only to org-level tokens. RegisteredIntegrationAdd is here because
// IntegrationsService/Register creates an org-level object and performs no project-scope check.
// RegisteredIntegrationList/Read gate ListRegistrations/DescribeRegistration, which enumerate and read
// any registered integration in the organization — including its configuration — with no project filter.
var orgLevelTokenPolicies = []*authz.Policy{
	authz.PolicyAPITokenCreate,
	authz.PolicyAPITokenList,
	authz.PolicyAPITokenRevoke,
	authz.PolicyRegisteredIntegrationAdd,
	authz.PolicyRegisteredIntegrationList,
	authz.PolicyRegisteredIntegrationRead,
}

// IsOrgLevelTokenPolicy reports whether a policy is one only an organization-wide token holds.
// A token confined to a project or to a product never carries one.
func IsOrgLevelTokenPolicy(p *authz.Policy) bool {
	if p == nil {
		return false
	}

	return slices.ContainsFunc(orgLevelTokenPolicies, func(o *authz.Policy) bool {
		return o.Resource == p.Resource && o.Action == p.Action
	})
}

// defaultAuthzPolicies are granted to every token regardless of scope, so each entry must be safe
// for a caller confined to a single project. Org-wide capabilities go in orgLevelTokenPolicies.
var defaultAuthzPolicies = []*authz.Policy{
	// Add permissions to workflow run
	authz.PolicyWorkflowRunList, authz.PolicyWorkflowRunRead,
	// To read, list and create workflows
	authz.PolicyWorkflowRead, authz.PolicyWorkflowList, authz.PolicyWorkflowCreate,
	// Add permissions to workflow contract management
	authz.PolicyWorkflowContractList, authz.PolicyWorkflowContractRead, authz.PolicyWorkflowContractUpdate, authz.PolicyWorkflowContractCreate,
	// to download artifacts and list referrers
	authz.PolicyArtifactDownload, authz.PolicyReferrerRead,
	authz.PolicyOrganizationRead,

	// to attach integrations
	authz.PolicyAvailableIntegrationRead,
	authz.PolicyAvailableIntegrationList,
	authz.PolicyAttachedIntegrationList,
	authz.PolicyAttachedIntegrationAttach,

	// to upload CAS artifacts
	authz.PolicyArtifactUpload,
}

// APIToken is used for unattended access to the control plane API.
type APIToken struct {
	ID          uuid.UUID
	Name        string
	Description string
	// This is the JWT value returned only during creation
	JWT string
	// Tokens are scoped to organizations
	OrganizationID   uuid.UUID
	OrganizationName string
	CreatedAt        *time.Time
	// When the token expires
	ExpiresAt *time.Time
	// When the token was manually revoked
	RevokedAt  *time.Time
	LastUsedAt *time.Time
	// If the token is scoped to a project
	ProjectID   *uuid.UUID
	ProjectName *string
	// If the token is scoped to a specific workflow within a project
	WorkflowID   *uuid.UUID
	WorkflowName *string
	// What the token is scoped to: organization, project, instance or product. Every row records
	// it, rows from before these columns through the scope backfill migration; a row recording
	// none is confined to nothing. A product's name is not stored: it belongs to whoever owns
	// the product.
	Scope   *authz.ResourceType
	ScopeID *uuid.UUID
	// ProjectIDs are the projects a product token reaches, kept consolidated by the Chainloop
	// platform. Never nil on a product token, nil on every other kind.
	ProjectIDs []uuid.UUID
	// ACL policies for this token
	Policies []*authz.Policy
	// IsSystem marks tokens minted by internal code paths; these are hidden from the public API.
	IsSystem bool
}

// scopeView is the token's scope as the request context carries it, so that a token read here and
// one read off the context classify a scope the same way.
func (t *APIToken) scopeView() *entities.APIToken {
	if t == nil {
		return nil
	}

	return &entities.APIToken{Scope: t.Scope, ScopeID: t.ScopeID}
}

// IsProductScoped reports whether the token is confined to a product: it reaches the projects in
// its ProjectIDs rather than a project of its own. It keys on the scope kind, never on scope_id,
// which organization and project tokens carry too.
func (t *APIToken) IsProductScoped() bool {
	return t.scopeView().IsProductScoped()
}

// ResourceScope returns the resource the token is confined to, its project or its product, so
// callers can render and authorize it without naming its kind. ok is false for a token acting
// for its whole organization or instance, and for one recording no scope or no scope id.
func (t *APIToken) ResourceScope() (kind authz.ResourceType, id uuid.UUID, ok bool) {
	return t.scopeView().ResourceScope()
}

// IsInstanceScoped reports whether the token acts for the whole instance, with no organization of
// its own. It keys on the recorded scope alone: a row recording none is not an instance token.
func (t *APIToken) IsInstanceScoped() bool {
	return t.scopeView().IsInstanceScoped()
}

// IsOrgScoped reports whether the token acts for its whole organization. A token recording no
// scope is not.
func (t *APIToken) IsOrgScoped() bool {
	return t.scopeView().IsOrgScoped()
}

// VerifyClaims checks that the token's row matches the signed claims. The row must have the same
// scope, organization, project and workflow. A project or workflow that only the row or only the
// claims name is also a difference. Any difference refuses the token, so a wrong row can never
// widen the token or move it elsewhere.
//
// A mismatch error wraps ErrAPITokenClaimsMismatch. Two expected states of older tokens get an
// error without it:
//   - a row that records no scope, when the claims name no scope either.
//   - a product token minted before the scope claims existed. Its owner must create a new token.
func (t *APIToken) VerifyClaims(claims *apitoken.CustomClaims) error {
	if t == nil {
		return errors.New("API token not found")
	}

	if claims == nil {
		return errors.New("API token has no claims")
	}

	if t.Scope == nil {
		// Claims that name a scope show that the token had a scope when it was minted. So something
		// cleared the row's scope later. Legacy claims name no scope, and a row from before the
		// scope columns records no scope.
		err := errors.New("API token records no scope")
		if claims.HasScopeClaims() {
			err = fmt.Errorf("%w: %w", err, ErrAPITokenClaimsMismatch)
		}

		return err
	}

	// This check runs before GetScope on purpose. An older product token's claims name only its
	// organization. GetScope reads such claims as an organization scope. The comparison below
	// would then log a security event instead of asking for a new token.
	if !claims.HasScopeClaims() && t.IsProductScoped() {
		return errors.New("API token was minted before its scope was signed, create a new one")
	}

	kind, id, err := claims.GetScope()
	if err != nil {
		return fmt.Errorf("API token scope claims: %w: %w", err, ErrAPITokenClaimsMismatch)
	}

	if *t.Scope != kind || !sameScopeID(t.ScopeID, id) {
		return fmt.Errorf("API token scope mismatch: %w", ErrAPITokenClaimsMismatch)
	}

	// An instance token has no organization, and its org_id claim is empty
	orgID := ""
	if t.OrganizationID != uuid.Nil {
		orgID = t.OrganizationID.String()
	}

	if claims.OrgID != orgID {
		return fmt.Errorf("API token organization mismatch: %w", ErrAPITokenClaimsMismatch)
	}

	if !sameClaimedID(t.ProjectID, claims.ProjectID) {
		return fmt.Errorf("API token project mismatch: %w", ErrAPITokenClaimsMismatch)
	}

	if !sameClaimedID(t.WorkflowID, claims.WorkflowID) {
		return fmt.Errorf("API token workflow mismatch: %w", ErrAPITokenClaimsMismatch)
	}

	return nil
}

// sameClaimedID reports whether an optional id of the row equals its claim. An unset id equals
// an empty claim.
func sameClaimedID(row *uuid.UUID, claim string) bool {
	if row == nil {
		return claim == ""
	}

	return row.String() == claim
}

// sameScopeID reports whether two optional scope ids are equal. Two unset ids are equal.
func sameScopeID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// APITokenCreateOpts is everything the repository persists for a new token.
type APITokenCreateOpts struct {
	Name           string
	Description    *string
	ExpiresAt      *time.Time
	OrganizationID *uuid.UUID
	ProjectID      *uuid.UUID
	WorkflowID     *uuid.UUID
	// Scope records what the token is scoped to. ScopeID names that resource, and is unset only
	// for an instance-level token.
	Scope   *authz.ResourceType
	ScopeID *uuid.UUID
	// ProjectIDs is set on a product token only
	ProjectIDs []uuid.UUID
	Policies   []*authz.Policy
	IsSystem   bool
}

type APITokenRepo interface {
	Create(ctx context.Context, opts *APITokenCreateOpts) (*APIToken, error)
	List(ctx context.Context, orgID *uuid.UUID, filters *APITokenListFilters) ([]*APIToken, error)
	Revoke(ctx context.Context, orgID *uuid.UUID, ID uuid.UUID) error
	// FindInactive returns tokens in an organization that have been inactive since the given cutoff time.
	// A token is considered inactive if its last_used_at (or created_at, if never used) is before inactiveSince.
	FindInactive(ctx context.Context, orgID uuid.UUID, inactiveSince time.Time) ([]*APIToken, error)
	UpdateExpiration(ctx context.Context, ID uuid.UUID, expiresAt time.Time) error
	UpdateLastUsedAt(ctx context.Context, ID uuid.UUID, lastUsedAt time.Time) error
	FindByID(ctx context.Context, ID uuid.UUID) (*APIToken, error)
	FindByIDInOrg(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*APIToken, error)
	FindByNameInOrg(ctx context.Context, orgID uuid.UUID, name string) (*APIToken, error)
}

type APITokenUseCase struct {
	apiTokenRepo         APITokenRepo
	logger               *log.Helper
	jwtBuilder           *apitoken.Builder
	authz                *AuthzUseCase
	DefaultAuthzPolicies []*authz.Policy
	// Use Cases
	orgUseCase *OrganizationUseCase
	auditorUC  *AuditorUseCase
}

func NewAPITokenUseCase(apiTokenRepo APITokenRepo, jwtConfig *APITokenJWTConfig, authzUC *AuthzUseCase, orgUseCase *OrganizationUseCase, auditorUC *AuditorUseCase, logger log.Logger) (*APITokenUseCase, error) {
	uc := &APITokenUseCase{
		apiTokenRepo:         apiTokenRepo,
		orgUseCase:           orgUseCase,
		auditorUC:            auditorUC,
		logger:               servicelogger.ScopedHelper(logger, "biz/APITokenUseCase"),
		authz:                authzUC,
		DefaultAuthzPolicies: defaultAuthzPolicies,
	}

	// Create the JWT builder for the API token
	b, err := apitoken.NewBuilder(
		apitoken.WithIssuer(jwt.DefaultIssuer),
		apitoken.WithKeySecret(jwtConfig.SymmetricHmacKey),
	)
	if err != nil {
		return nil, fmt.Errorf("creating jwt builder: %w", err)
	}

	uc.jwtBuilder = b

	return uc, nil
}

type apiTokenOptions struct {
	project    *Project
	workflow   *Workflow
	scope      *authz.ResourceType
	scopeID    *uuid.UUID
	projectIDs []uuid.UUID
	policies   []*authz.Policy
	isSystem   bool
}

type APITokenCreateOpt func(*apiTokenOptions)

func APITokenWithProject(project *Project) APITokenCreateOpt {
	return func(o *apiTokenOptions) {
		o.project = project
	}
}

// APITokenWithWorkflow scopes the token to a specific workflow within a project.
// Must be combined with APITokenWithProject; the workflow's project must match.
func APITokenWithWorkflow(workflow *Workflow) APITokenCreateOpt {
	return func(o *apiTokenOptions) {
		o.workflow = workflow
	}
}

func APITokenWithPolicies(policies []*authz.Policy) APITokenCreateOpt {
	return func(o *apiTokenOptions) {
		o.policies = policies
	}
}

// APITokenAsSystem marks the token as system-managed (internal). System tokens
// are hidden from the public API.
func APITokenAsSystem() APITokenCreateOpt {
	return func(o *apiTokenOptions) {
		o.isSystem = true
	}
}

// APITokenWithScope names what the token is scoped to. scopeID identifies that resource and is
// nil only for an instance scope. The scope must agree with the organization and project the
// token is created for; without this option it is derived from them.
func APITokenWithScope(scope authz.ResourceType, scopeID *uuid.UUID) APITokenCreateOpt {
	return func(o *apiTokenOptions) {
		o.scope = &scope
		o.scopeID = scopeID
	}
}

// APITokenWithProjectIDs sets the projects a product-scoped token reaches, and is refused with
// any other scope. A nil list reaches nothing, as an empty one does.
func APITokenWithProjectIDs(ids []uuid.UUID) APITokenCreateOpt {
	return func(o *apiTokenOptions) {
		o.projectIDs = CanonicalProjectIDs(ids)
	}
}

// CanonicalProjectIDs returns ids deduplicated and sorted, never nil, so that equal sets are
// stored as equal lists and compare equal.
func CanonicalProjectIDs(ids []uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	out = append(out, ids...)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })

	return slices.Compact(out)
}

// ValidateTokenShape reports whether a token row is coherent: an organization or project scope
// names the token's own organization or project, an instance scope has no id, a product scope
// belongs to an organization and never to a project, and only a product token carries a project
// list, which it always does. A row may record no scope, as one written by a control plane from
// before the scope columns does; it then carries no list either. Create checks it against the
// scope it settles on, and the repository before every write, whoever the writer is.
func ValidateTokenShape(scope *authz.ResourceType, scopeID, orgID, projectID *uuid.UUID, projectIDs []uuid.UUID) error {
	if (scope != nil && *scope == authz.ResourceTypeProduct) != (projectIDs != nil) {
		return NewErrValidationStr("only a product-scoped token carries a project list, and it always carries one")
	}

	if scope == nil {
		if scopeID != nil {
			return NewErrValidationStr("a scope id needs a scope kind")
		}

		return nil
	}

	same := func(a, b *uuid.UUID) bool { return a != nil && b != nil && *a == *b }

	var coherent bool
	switch *scope {
	case authz.ResourceTypeOrganization:
		coherent = projectID == nil && same(scopeID, orgID)
	case authz.ResourceTypeProject:
		coherent = same(scopeID, projectID)
	case authz.ResourceTypeInstance:
		coherent = scopeID == nil && orgID == nil && projectID == nil
	case authz.ResourceTypeProduct:
		coherent = scopeID != nil && orgID != nil && projectID == nil
	}

	if !coherent {
		return NewErrValidationStr(fmt.Sprintf("a %q scope does not agree with the token it is on", *scope))
	}

	return nil
}

// newTokenScope is the scope a new token records when none is named: its project, else its
// organization, else the instance. The scope backfill migration gave older rows the same one.
func newTokenScope(orgID, projectID *uuid.UUID) (*authz.ResourceType, *uuid.UUID) {
	switch {
	case projectID != nil:
		return ToPtr(authz.ResourceTypeProject), projectID
	case orgID != nil:
		return ToPtr(authz.ResourceTypeOrganization), orgID
	default:
		return ToPtr(authz.ResourceTypeInstance), nil
	}
}

// expires in is a string that can be parsed by time.ParseDuration
func (uc *APITokenUseCase) Create(ctx context.Context, name string, description *string, expiresIn *time.Duration, orgID *string, opts ...APITokenCreateOpt) (*APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenTracer, "APITokenUseCase.Create")
	defer span.End()

	options := &apiTokenOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Parse organization ID if provided
	var orgUUID *uuid.UUID
	var org *Organization
	if orgID != nil && *orgID != "" {
		parsed, err := uuid.Parse(*orgID)
		if err != nil {
			return nil, NewErrInvalidUUID(err)
		}
		orgUUID = &parsed

		// Retrieve the organization
		org, err = uc.orgUseCase.FindByID(ctx, *orgID)
		if err != nil {
			return nil, fmt.Errorf("finding organization: %w", err)
		}
	}

	if name == "" {
		return nil, NewErrValidationStr("name is required")
	}

	// validate format of the name and the project
	if err := ValidateIsDNS1123(name); err != nil {
		return nil, NewErrValidation(err)
	}

	// If expiration is provided we store it
	// we also validate that it's at least 24 hours and valid string format
	var expiresAt *time.Time
	if expiresIn != nil {
		expiresAt = new(time.Time)
		*expiresAt = time.Now().Add(*expiresIn)
	}

	// If a project is provided, we store it in the token
	var projectID *uuid.UUID
	if options.project != nil {
		projectID = ToPtr(options.project.ID)
	}

	// A project token belongs to an organization. Without one it would be written with no
	// organization and signed as an instance-level token.
	if projectID != nil && orgUUID == nil {
		return nil, NewErrValidationStr("a project-scoped token requires an organization")
	}

	var workflowID *uuid.UUID
	if options.workflow != nil {
		if options.project == nil {
			return nil, NewErrValidationStr("workflow scope requires a project scope")
		}
		if options.workflow.ProjectID != options.project.ID {
			return nil, NewErrValidationStr("workflow does not belong to the requested project")
		}
		workflowID = ToPtr(options.workflow.ID)
	}

	// The scope a new token of this shape records, unless one is named, which must agree with it
	scope, scopeID := newTokenScope(orgUUID, projectID)
	if options.scope != nil {
		scope, scopeID = options.scope, options.scopeID
	}

	if err := ValidateTokenShape(scope, scopeID, orgUUID, projectID, options.projectIDs); err != nil {
		return nil, err
	}

	// Use provided policies if present, otherwise use defaults
	policies := options.policies
	if policies == nil {
		policies = uc.DefaultAuthzPolicies
	}

	// A product token's policies come only from the platform, in-process: the refusals below
	// are defence in depth against the organization-level grant this function appends to
	// organization-wide tokens, not a classification of every policy. What confines the token
	// is its project list.
	if *scope == authz.ResourceTypeProduct {
		if len(policies) == 0 {
			return nil, NewErrValidationStr("a product-scoped token needs at least one policy")
		}

		if slices.Contains(policies, nil) {
			return nil, NewErrValidationStr("a product-scoped token cannot carry a nil policy")
		}

		if slices.ContainsFunc(policies, IsOrgLevelTokenPolicy) {
			return nil, NewErrValidationStr("a product-scoped token cannot carry organization-level policies")
		}
	}

	// Concat, not append: policies may alias the shared defaultAuthzPolicies slice.
	if *scope == authz.ResourceTypeOrganization {
		policies = slices.Concat(policies, orgLevelTokenPolicies)
	}

	// NOTE: the expiration time is stored just for reference, it's also encoded in the JWT
	// We store it since Chainloop will not have access to the JWT to check the expiration once created
	token, err := uc.apiTokenRepo.Create(ctx, &APITokenCreateOpts{
		Name:           name,
		Description:    description,
		ExpiresAt:      expiresAt,
		OrganizationID: orgUUID,
		ProjectID:      projectID,
		WorkflowID:     workflowID,
		Scope:          scope,
		ScopeID:        scopeID,
		ProjectIDs:     options.projectIDs,
		Policies:       policies,
		IsSystem:       options.isSystem,
	})
	if err != nil {
		if IsErrAlreadyExists(err) {
			return nil, NewErrAlreadyExistsStr("name already taken")
		}
		return nil, fmt.Errorf("storing token: %w", err)
	}

	generationOpts := &apitoken.GenerateJWTOptions{
		KeyID:     token.ID,
		KeyName:   name,
		ExpiresAt: expiresAt,
		// The JWT signs the scope. The row must then match it.
		Scope:   scope,
		ScopeID: scopeID,
	}

	// An instance-level token has no organization
	if org != nil {
		generationOpts.OrgID = &token.OrganizationID
		generationOpts.OrgName = &org.Name
	}

	if projectID != nil {
		generationOpts.ProjectID = ToPtr(options.project.ID)
		generationOpts.ProjectName = ToPtr(options.project.Name)
	}

	if options.workflow != nil {
		generationOpts.WorkflowID = ToPtr(options.workflow.ID)
		generationOpts.WorkflowName = ToPtr(options.workflow.Name)
	}

	// generate the JWT
	token.JWT, err = uc.jwtBuilder.GenerateJWT(generationOpts)
	if err != nil {
		return nil, fmt.Errorf("generating jwt: %w", err)
	}

	// Dispatch the event to the auditor to notify the creation of the token
	uc.auditorUC.Dispatch(ctx, &events.APITokenCreated{
		APITokenBase: &events.APITokenBase{
			APITokenID:   &token.ID,
			APITokenName: name,
			Scope:        scope,
			ScopeID:      scopeID,
		},
		APITokenDescription: description,
		ExpiresAt:           expiresAt,
	}, orgUUID)

	return token, nil
}

// RegenerateJWT signs a new JWT for the given token. Use it with caution: it does not invalidate
// old JWTs. The new JWT signs the scope that the row records. RegenerateJWT refuses a row with no
// scope, or with a scope that contradicts its other columns. It still signs a row that is
// consistent but wrong, so callers must be sure that it is the token they mean.
func (uc *APITokenUseCase) RegenerateJWT(ctx context.Context, tokenID uuid.UUID, expiresIn time.Duration) (*APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenTracer, "APITokenUseCase.RegenerateJWT")
	defer span.End()

	if expiresIn == 0 {
		return nil, fmt.Errorf("expiresAt is mandatory")
	}

	expiresAt := time.Now().Add(expiresIn)

	token, err := uc.apiTokenRepo.FindByID(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("finding token: %w", err)
	}

	// Never sign a row that records no scope, or a row whose scope contradicts its other columns.
	if token.Scope == nil {
		return nil, NewErrValidationStr("the token records no scope")
	}

	var rowOrgID *uuid.UUID
	if token.OrganizationID != uuid.Nil {
		rowOrgID = &token.OrganizationID
	}

	if err := ValidateTokenShape(token.Scope, token.ScopeID, rowOrgID, token.ProjectID, token.ProjectIDs); err != nil {
		return nil, err
	}

	generationOpts := &apitoken.GenerateJWTOptions{
		KeyID:     token.ID,
		KeyName:   token.Name,
		ExpiresAt: &expiresAt,
		Scope:     token.Scope,
		ScopeID:   token.ScopeID,
	}

	// An instance-level token has no organization
	if token.OrganizationID != uuid.Nil {
		org, err := uc.orgUseCase.FindByID(ctx, token.OrganizationID.String())
		if err != nil {
			return nil, fmt.Errorf("finding organization: %w", err)
		}
		generationOpts.OrgID = &token.OrganizationID
		generationOpts.OrgName = &org.Name
	}

	// Preserve project / workflow scope claims that the row carries.
	if token.ProjectID != nil {
		generationOpts.ProjectID = token.ProjectID
		generationOpts.ProjectName = token.ProjectName
	}
	if token.WorkflowID != nil {
		generationOpts.WorkflowID = token.WorkflowID
		generationOpts.WorkflowName = token.WorkflowName
	}

	// generate the JWT
	token.JWT, err = uc.jwtBuilder.GenerateJWT(generationOpts)
	if err != nil {
		return nil, fmt.Errorf("generating jwt: %w", err)
	}

	// update the token expiration in db
	if err = uc.apiTokenRepo.UpdateExpiration(ctx, tokenID, expiresAt); err != nil {
		return nil, fmt.Errorf("updating expiration for token: %w", err)
	}

	return token, nil
}

type APITokenListOpt func(*APITokenListFilters)

func WithAPITokenProjectFilter(projectIDs []uuid.UUID) APITokenListOpt {
	return func(opts *APITokenListFilters) {
		opts.FilterByProjects = projectIDs
	}
}

func WithAPITokenStatusFilter(filter APITokenStatusFilter) APITokenListOpt {
	return func(opts *APITokenListFilters) {
		opts.StatusFilter = filter
	}
}

// WithAPITokenScope selects the tokens scoped to the given kind of resource. Organization
// selects the organization-wide tokens.
func WithAPITokenScope(scope authz.ResourceType) APITokenListOpt {
	return func(opts *APITokenListFilters) {
		opts.FilterByScope = scope
	}
}

// Deprecated: use authz.ResourceType. Kept so that callers built against the older API, such as
// the Chainloop platform's main branch, still compile against WithAPITokenScope.
type APITokenScope = authz.ResourceType

const (
	// Deprecated: use authz.ResourceTypeProject.
	APITokenScopeProject = authz.ResourceTypeProject
	// Deprecated: use authz.ResourceTypeOrganization.
	APITokenScopeGlobal = authz.ResourceTypeOrganization
	// Deprecated: use authz.ResourceTypeInstance.
	APITokenScopeInstance = authz.ResourceTypeInstance
)

// WithIncludeSystemTokens opts the listing in to also return system-managed tokens.
// By default, system tokens are hidden.
func WithIncludeSystemTokens() APITokenListOpt {
	return func(opts *APITokenListFilters) {
		opts.IncludeSystem = true
	}
}

// listableAPITokenScopes are the kinds of resource a token listing can be scoped to.
var listableAPITokenScopes = []authz.ResourceType{
	authz.ResourceTypeProject,
	authz.ResourceTypeProduct,
	authz.ResourceTypeOrganization,
	authz.ResourceTypeInstance,
}

// APITokenStatusFilter controls which tokens are returned based on their revocation status.
type APITokenStatusFilter int

const (
	// APITokenStatusFilterActive returns only active (non-revoked) tokens. This is the default.
	APITokenStatusFilterActive APITokenStatusFilter = iota
	// APITokenStatusFilterRevoked returns only revoked tokens.
	APITokenStatusFilterRevoked
	// APITokenStatusFilterAll returns all tokens regardless of revocation status.
	APITokenStatusFilterAll
)

type APITokenListFilters struct {
	// FilterByProjects narrows the result to the given projects. nil means no filter is applied;
	// a non-nil empty list matches no project at all.
	FilterByProjects []uuid.UUID
	// StatusFilter controls which tokens are returned based on revocation status.
	// Defaults to APITokenStatusFilterActive.
	StatusFilter APITokenStatusFilter
	// FilterByScope is used to filter the result by the scope of the token
	FilterByScope authz.ResourceType
	// IncludeSystem controls whether system-managed tokens are returned.
	// Defaults to false (system tokens are hidden).
	IncludeSystem bool
}

func (uc *APITokenUseCase) List(ctx context.Context, orgID string, opts ...APITokenListOpt) ([]*APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenTracer, "APITokenUseCase.List")
	defer span.End()

	filters := &APITokenListFilters{}
	for _, opt := range opts {
		opt(filters)
	}

	if filters.FilterByScope != "" && !slices.Contains(listableAPITokenScopes, filters.FilterByScope) {
		return nil, NewErrValidationStr(fmt.Sprintf("invalid scope %q, please chose one of: %v", filters.FilterByScope, listableAPITokenScopes))
	}

	var orgUUID *uuid.UUID
	if orgID != "" {
		parsed, err := uuid.Parse(orgID)
		if err != nil {
			return nil, NewErrInvalidUUID(err)
		}
		orgUUID = &parsed
	}

	return uc.apiTokenRepo.List(ctx, orgUUID, filters)
}

func (uc *APITokenUseCase) Revoke(ctx context.Context, orgID, id string) error {
	ctx, span := otelx.Start(ctx, apiTokenTracer, "APITokenUseCase.Revoke")
	defer span.End()

	var orgUUID *uuid.UUID
	if orgID != "" {
		parsed, err := uuid.Parse(orgID)
		if err != nil {
			return NewErrInvalidUUID(err)
		}
		orgUUID = &parsed
	}

	tokenUUID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	token, err := uc.apiTokenRepo.FindByID(ctx, tokenUUID)
	if err != nil {
		return fmt.Errorf("finding token: %w", err)
	}

	if rvErr := uc.apiTokenRepo.Revoke(ctx, orgUUID, tokenUUID); rvErr != nil {
		return fmt.Errorf("revoking token: %w", rvErr)
	}

	// Dispatch the event to the auditor to notify the revocation of the token
	uc.auditorUC.Dispatch(ctx, &events.APITokenRevoked{
		APITokenBase: &events.APITokenBase{
			APITokenID:   &tokenUUID,
			APITokenName: token.Name,
			Scope:        token.Scope,
			ScopeID:      token.ScopeID,
		},
	}, orgUUID)

	return nil
}

func (uc *APITokenUseCase) FindByIDInOrg(ctx context.Context, orgID, id string) (*APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenTracer, "APITokenUseCase.FindByIDInOrg")
	defer span.End()

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	tokenUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	t, err := uc.apiTokenRepo.FindByIDInOrg(ctx, orgUUID, tokenUUID)
	if err != nil {
		return nil, fmt.Errorf("finding token: %w", err)
	}

	return t, nil
}

func (uc *APITokenUseCase) FindByID(ctx context.Context, id string) (*APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenTracer, "APITokenUseCase.FindByID")
	defer span.End()

	uuid, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	t, err := uc.apiTokenRepo.FindByID(ctx, uuid)
	if err != nil {
		return nil, fmt.Errorf("finding token: %w", err)
	} else if t == nil {
		return nil, NewErrNotFound("token")
	}

	return t, nil
}

func (uc *APITokenUseCase) FindByNameInOrg(ctx context.Context, orgID, name string) (*APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenTracer, "APITokenUseCase.FindByNameInOrg")
	defer span.End()

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.apiTokenRepo.FindByNameInOrg(ctx, orgUUID, name)
}

func (uc *APITokenUseCase) UpdateLastUsedAt(ctx context.Context, tokenID string) error {
	ctx, span := otelx.Start(ctx, apiTokenTracer, "APITokenUseCase.UpdateLastUsedAt")
	defer span.End()

	id, err := uuid.Parse(tokenID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	if err := uc.apiTokenRepo.UpdateLastUsedAt(ctx, id, time.Now()); err != nil {
		return fmt.Errorf("updating last used at: %w", err)
	}

	return nil
}
