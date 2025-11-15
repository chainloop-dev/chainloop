//
// Copyright 2025 The Chainloop Authors.
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
	"strings"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type AuthzUseCase struct {
	log *log.Helper
	// actual CASBIN enforcer
	enforcer            *authz.Enforcer
	apiTokenRepo        APITokenRepo
	RestrictOrgCreation bool
}

type AuthzUseCaseConfig struct {
	Enforcer            *authz.Enforcer
	APITokenRepo        APITokenRepo
	RestrictOrgCreation bool
	Logger              log.Logger
}

func NewAuthzUseCase(config *AuthzUseCaseConfig) *AuthzUseCase {
	return &AuthzUseCase{
		log:                 log.NewHelper(log.With(config.Logger, "component", "biz/authz")),
		apiTokenRepo:        config.APITokenRepo,
		enforcer:            config.Enforcer,
		RestrictOrgCreation: config.RestrictOrgCreation,
	}
}

// Wrapper around the Enforcer.Enforce method that takes into account some of our nuances
// with regards to policies retrieval and handling for API tokens.
func (e *AuthzUseCase) Enforce(ctx context.Context, sub string, p *authz.Policy) (ok bool, err error) {
	defer e.log.Infow("msg", "policy enforcement result", "sub", sub, "policy", p, "result", ok)

	// Check if this is an API token (subject starts with "api-token:")
	if strings.HasPrefix(sub, "api-token:") {
		// load the token using the ID that's the second part of the subject
		tokenID := strings.Split(sub, ":")[1]
		tokenIDUUID, err := uuid.Parse(tokenID)
		if err != nil {
			return false, err
		}

		token, err := e.apiTokenRepo.FindByID(ctx, tokenIDUUID)
		if err != nil {
			return false, err
		}

		if token == nil {
			return false, NewErrNotFound("API token")
		}

		for _, allowed := range token.Policies {
			if allowed.Resource == p.Resource && allowed.Action == p.Action {
				return true, nil
			}
		}

		// Tokens for now only support ACL-based authorization
		// in the future we'll continue to use the enforcer to check if the policy is allowed for the subject
		return false, nil
	}

	return e.enforcer.Enforce(sub, p)
}
