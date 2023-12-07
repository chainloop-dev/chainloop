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

package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/conf"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/apitoken"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// API Token is used for unattended access to the control plane API.
type APIToken struct {
	ID          uuid.UUID
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
	Create(ctx context.Context, description *string, expiresAt *time.Time, organizationID uuid.UUID) (*APIToken, error)
}

type APITokenUseCase struct {
	apiTokenRepo   APITokenRepo
	membershipRepo MembershipRepo
	logger         *log.Helper
	jwtBuilder     *apitoken.Builder
}

func NewAPITokenUseCase(apiTokenRepo APITokenRepo, mRepo MembershipRepo, conf *conf.Auth, logger log.Logger) (*APITokenUseCase, error) {
	uc := &APITokenUseCase{
		apiTokenRepo:   apiTokenRepo,
		membershipRepo: mRepo,
		logger:         log.NewHelper(logger),
	}

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

// This is the minimum duration that a token can be created for
const minDuration = 24 * time.Hour

// expires in is a string that can be parsed by time.ParseDuration
func (uc *APITokenUseCase) Create(ctx context.Context, description *string, expiresIn *time.Duration, userID, orgID string) (*APIToken, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Make sure that the organization exists and that the user is a member of it
	membership, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgUUID, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to find memberships: %w", err)
	} else if membership == nil {
		return nil, NewErrNotFound("organization")
	}

	// If expiration is provided we store it
	// we also validate that it's at least 24 hours and valid string format
	var expiresAt *time.Time
	if expiresIn != nil {
		if *expiresIn < minDuration {
			return nil, NewErrValidation(fmt.Errorf("expiration must be at least %s", minDuration))
		}

		expiresAt = new(time.Time)
		*expiresAt = time.Now().Add(*expiresIn)
	}

	// NOTE: the expiration time is stored just for reference, it's also encoded in the JWT
	// We store it since Chainloop will not have access to the JWT to check the expiration once created
	token, err := uc.apiTokenRepo.Create(ctx, description, expiresAt, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("storing token: %w", err)
	}

	// generate the JWT
	token.JWT, err = uc.jwtBuilder.GenerateJWT(orgID, token.ID.String(), expiresAt)
	if err != nil {
		return nil, fmt.Errorf("generating jwt: %w", err)
	}

	return token, nil
}
