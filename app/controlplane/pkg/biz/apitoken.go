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

package biz

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type APITokenJWTConfig struct {
	SymmetricHmacKey string
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
	// ACL policies for this token
	Policies []*authz.Policy
}

type APITokenRepo interface {
	Create(ctx context.Context, name string, description *string, expiresAt *time.Time, organizationID *uuid.UUID, projectID *uuid.UUID, policies []*authz.Policy) (*APIToken, error)
	List(ctx context.Context, orgID *uuid.UUID, filters *APITokenListFilters) ([]*APIToken, error)
	Revoke(ctx context.Context, orgID *uuid.UUID, ID uuid.UUID) error
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
		apiTokenRepo: apiTokenRepo,
		orgUseCase:   orgUseCase,
		auditorUC:    auditorUC,
		logger:       servicelogger.ScopedHelper(logger, "biz/APITokenUseCase"),
		authz:        authzUC,
		DefaultAuthzPolicies: []*authz.Policy{
			// Add permissions to workflow run
			authz.PolicyWorkflowRunList, authz.PolicyWorkflowRunRead,
			// To read, list and create workflows
			authz.PolicyWorkflowRead, authz.PolicyWorkflowList, authz.PolicyWorkflowCreate,
			// Add permissions to workflow contract management
			authz.PolicyWorkflowContractList, authz.PolicyWorkflowContractRead, authz.PolicyWorkflowContractUpdate, authz.PolicyWorkflowContractCreate,
			// to download artifacts and list referrers
			authz.PolicyArtifactDownload, authz.PolicyReferrerRead,
			authz.PolicyOrganizationRead,
			// to create robot accounts
			authz.PolicyRobotAccountCreate,

			// to attach integrations
			authz.PolicyAvailableIntegrationRead,
			authz.PolicyAvailableIntegrationList,
			authz.PolicyRegisteredIntegrationList,
			authz.PolicyRegisteredIntegrationRead,
			authz.PolicyRegisteredIntegrationAdd,
			authz.PolicyAttachedIntegrationList,
			authz.PolicyAttachedIntegrationAttach,

			// to upload CAS artifacts
			authz.PolicyArtifactUpload,
		},
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
	project              *Project
	showOnlySystemTokens bool
	policies             []*authz.Policy
}

type APITokenCreateOpt func(*apiTokenOptions)

func APITokenWithProject(project *Project) APITokenCreateOpt {
	return func(o *apiTokenOptions) {
		o.project = project
	}
}

func APITokenWithPolicies(policies []*authz.Policy) APITokenCreateOpt {
	return func(o *apiTokenOptions) {
		o.policies = policies
	}
}

// expires in is a string that can be parsed by time.ParseDuration
func (uc *APITokenUseCase) Create(ctx context.Context, name string, description *string, expiresIn *time.Duration, orgID *string, opts ...APITokenCreateOpt) (*APIToken, error) {
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

	// Use provided policies if present, otherwise use defaults
	policies := options.policies
	if policies == nil {
		policies = uc.DefaultAuthzPolicies
	}

	// NOTE: the expiration time is stored just for reference, it's also encoded in the JWT
	// We store it since Chainloop will not have access to the JWT to check the expiration once created
	token, err := uc.apiTokenRepo.Create(ctx, name, description, expiresAt, orgUUID, projectID, policies)
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
	}

	// Set org info if available or instance-level token scope
	if org != nil {
		generationOpts.OrgID = &token.OrganizationID
		generationOpts.OrgName = &org.Name
	} else {
		generationOpts.Scope = ToPtr(authz.ScopeInstanceAdmin)
	}

	if projectID != nil {
		generationOpts.ProjectID = ToPtr(options.project.ID)
		generationOpts.ProjectName = ToPtr(options.project.Name)
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
		},
		APITokenDescription: description,
		ExpiresAt:           expiresAt,
	}, orgUUID)

	return token, nil
}

// RegenerateJWT will regenerate a new JWT for the given token. Use with caution, since old JWTs are not invalidated.
func (uc *APITokenUseCase) RegenerateJWT(ctx context.Context, tokenID uuid.UUID, expiresIn time.Duration) (*APIToken, error) {
	if expiresIn == 0 {
		return nil, fmt.Errorf("expiresAt is mandatory")
	}

	expiresAt := time.Now().Add(expiresIn)

	token, err := uc.apiTokenRepo.FindByID(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("finding token: %w", err)
	}

	generationOpts := &apitoken.GenerateJWTOptions{
		KeyID:     token.ID,
		KeyName:   token.Name,
		ExpiresAt: &expiresAt,
	}

	// Check if this is an org-scoped or instance-level token
	if token.OrganizationID != uuid.Nil {
		// Org-scoped token
		org, err := uc.orgUseCase.FindByID(ctx, token.OrganizationID.String())
		if err != nil {
			return nil, fmt.Errorf("finding organization: %w", err)
		}
		generationOpts.OrgID = &token.OrganizationID
		generationOpts.OrgName = &org.Name
	} else {
		// Instance-level token
		generationOpts.Scope = ToPtr(authz.ScopeInstanceAdmin)
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

func WithAPITokenRevoked(includeRevoked bool) APITokenListOpt {
	return func(opts *APITokenListFilters) {
		opts.IncludeRevoked = includeRevoked
	}
}

func WithAPITokenScope(scope APITokenScope) APITokenListOpt {
	return func(opts *APITokenListFilters) {
		opts.FilterByScope = scope
	}
}

type APITokenScope string

const (
	APITokenScopeProject  APITokenScope = "project"
	APITokenScopeGlobal   APITokenScope = "global"
	APITokenScopeInstance APITokenScope = "instance"
)

var availableAPITokenScopes = []APITokenScope{
	APITokenScopeProject,
	APITokenScopeGlobal,
	APITokenScopeInstance,
}

type APITokenListFilters struct {
	// FilterByProjects is used to filter the result by a project list
	// If it's empty, no filter will be applied
	FilterByProjects []uuid.UUID
	// IncludeRevoked is used to include revoked tokens in the result
	IncludeRevoked bool
	// FilterByScope is used to filter the result by the scope of the token
	FilterByScope APITokenScope
}

func (uc *APITokenUseCase) List(ctx context.Context, orgID string, opts ...APITokenListOpt) ([]*APIToken, error) {
	filters := &APITokenListFilters{}
	for _, opt := range opts {
		opt(filters)
	}

	if filters.FilterByScope != "" && !slices.Contains(availableAPITokenScopes, filters.FilterByScope) {
		return nil, NewErrValidationStr(fmt.Sprintf("invalid scope %q, please chose one of: %v", filters.FilterByScope, availableAPITokenScopes))
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
		},
	}, orgUUID)

	return nil
}

func (uc *APITokenUseCase) FindByIDInOrg(ctx context.Context, orgID, id string) (*APIToken, error) {
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
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.apiTokenRepo.FindByNameInOrg(ctx, orgUUID, name)
}

func (uc *APITokenUseCase) UpdateLastUsedAt(ctx context.Context, tokenID string) error {
	id, err := uuid.Parse(tokenID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	if err := uc.apiTokenRepo.UpdateLastUsedAt(ctx, id, time.Now()); err != nil {
		return fmt.Errorf("updating last used at: %w", err)
	}

	return nil
}
