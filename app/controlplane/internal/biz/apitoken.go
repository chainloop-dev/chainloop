//
// Copyright 2024 The Chainloop Authors.
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
	"errors"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/apitoken"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// API Token is used for unattended access to the control plane API.
type APIToken struct {
	ID          uuid.UUID
	Name        string
	Description string
	// This is the JWT value returned only during creation
	JWT string
	// Tokens are scoped to organizations
	OrganizationID uuid.UUID
	CreatedAt      *time.Time
	// When the token expires
	ExpiresAt *time.Time
	// When the token was manually revoked
	RevokedAt *time.Time
}

type APITokenRepo interface {
	Create(ctx context.Context, name string, description *string, expiresAt *time.Time, organizationID uuid.UUID) (*APIToken, error)
	// List all the tokens optionally filtering it by organization and including revoked tokens
	List(ctx context.Context, orgID *uuid.UUID, includeRevoked bool) ([]*APIToken, error)
	Revoke(ctx context.Context, orgID, ID uuid.UUID) error
	FindByID(ctx context.Context, ID uuid.UUID) (*APIToken, error)
}

type APITokenUseCase struct {
	apiTokenRepo         APITokenRepo
	logger               *log.Helper
	jwtBuilder           *apitoken.Builder
	enforcer             *authz.Enforcer
	DefaultAuthzPolicies []*authz.Policy
}

type APITokenSyncerUseCase struct {
	base *APITokenUseCase
}

func NewAPITokenUseCase(apiTokenRepo APITokenRepo, conf *conf.Auth, authzE *authz.Enforcer, logger log.Logger) (*APITokenUseCase, error) {
	uc := &APITokenUseCase{
		apiTokenRepo: apiTokenRepo,
		logger:       servicelogger.ScopedHelper(logger, "biz/APITokenUseCase"),
		enforcer:     authzE,
		DefaultAuthzPolicies: []*authz.Policy{
			// Add permissions to workflow run
			authz.PolicyWorkflowRunList, authz.PolicyWorkflowRunRead,
			// To read and create workflows
			authz.PolicyWorkflowRead, authz.PolicyWorkflowCreate,
			// Add permissions to workflow contract management
			authz.PolicyWorkflowContractList, authz.PolicyWorkflowContractRead, authz.PolicyWorkflowContractUpdate,
			// to download artifacts and list referrers
			authz.PolicyArtifactDownload, authz.PolicyReferrerRead,
			authz.PolicyOrganizationRead,
			// to create robot accounts
			authz.PolicyRobotAccountCreate,
		},
	}

	// Create the JWT builder for the API token
	b, err := apitoken.NewBuilder(
		apitoken.WithIssuer(jwt.DefaultIssuer),
		apitoken.WithKeySecret(conf.GeneratedJwsHmacSecret),
	)
	if err != nil {
		return nil, fmt.Errorf("creating jwt builder: %w", err)
	}

	uc.jwtBuilder = b

	return uc, nil
}

// expires in is a string that can be parsed by time.ParseDuration
func (uc *APITokenUseCase) Create(ctx context.Context, name string, description *string, expiresIn *time.Duration, orgID string) (*APIToken, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
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

	// NOTE: the expiration time is stored just for reference, it's also encoded in the JWT
	// We store it since Chainloop will not have access to the JWT to check the expiration once created
	token, err := uc.apiTokenRepo.Create(ctx, name, description, expiresAt, orgUUID)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return nil, NewErrValidationStr("name already taken")
		}
		return nil, fmt.Errorf("storing token: %w", err)
	}

	// generate the JWT
	token.JWT, err = uc.jwtBuilder.GenerateJWT(token.OrganizationID.String(), token.ID.String(), expiresAt)
	if err != nil {
		return nil, fmt.Errorf("generating jwt: %w", err)
	}

	// Add default policies to the enforcer
	if err := uc.enforcer.AddPolicies(&authz.SubjectAPIToken{ID: token.ID.String()}, uc.DefaultAuthzPolicies...); err != nil {
		return nil, fmt.Errorf("adding default policies: %w", err)
	}

	return token, nil
}

func (uc *APITokenUseCase) List(ctx context.Context, orgID string, includeRevoked bool) ([]*APIToken, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.apiTokenRepo.List(ctx, &orgUUID, includeRevoked)
}

func (uc *APITokenUseCase) Revoke(ctx context.Context, orgID, id string) error {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	uuid, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// clean policies
	if err := uc.enforcer.ClearPolicies(&authz.SubjectAPIToken{ID: id}); err != nil {
		return fmt.Errorf("removing policies: %w", err)
	}

	return uc.apiTokenRepo.Revoke(ctx, orgUUID, uuid)
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

func NewAPITokenSyncerUseCase(tokenUC *APITokenUseCase) *APITokenSyncerUseCase {
	return &APITokenSyncerUseCase{
		base: tokenUC,
	}
}

// Make sure all the API tokens contain the default policies
// NOTE: We'll remove this method once we have a proper policies management system where the user can add/remove policies
func (suc *APITokenSyncerUseCase) SyncPolicies() error {
	suc.base.logger.Info("syncing policies for all the API tokens")

	// List all the non-revoked tokens from all the orgs
	tokens, err := suc.base.apiTokenRepo.List(context.Background(), nil, false)
	if err != nil {
		return fmt.Errorf("listing tokens: %w", err)
	}

	for _, t := range tokens {
		// Add default policies to the enforcer
		if err := suc.base.enforcer.AddPolicies(&authz.SubjectAPIToken{ID: t.ID.String()}, suc.base.DefaultAuthzPolicies...); err != nil {
			return fmt.Errorf("adding default policies: %w", err)
		}
	}

	return nil
}
