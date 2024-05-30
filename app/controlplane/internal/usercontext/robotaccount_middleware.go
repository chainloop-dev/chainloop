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

package usercontext

import (
	"context"
	"errors"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/robotaccount"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/attjwtmiddleware"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
)

type RobotAccount struct {
	ID, WorkflowID, OrgID, ProviderKey string
}

func withRobotAccount(ctx context.Context, acc *RobotAccount) context.Context {
	return context.WithValue(ctx, currentRobotAccountCtxKey{}, acc)
}

func CurrentRobotAccount(ctx context.Context) *RobotAccount {
	res := ctx.Value(currentRobotAccountCtxKey{})
	if res == nil {
		return nil
	}

	return res.(*RobotAccount)
}

type currentRobotAccountCtxKey struct{}

// WithAttestationContextFromRobotAccount Middleware that injects the current user to the context
func WithAttestationContextFromRobotAccount(robotAccountUseCase *biz.RobotAccountUseCase, orgUseCase *biz.OrganizationUseCase, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			authInfo, ok := attjwtmiddleware.FromJWTAuthContext(ctx)
			// If not found means that there is no currentUser
			if !ok {
				logger.Warn("couldn't extract robot account, JWT parser middleware not running before this one?")
				return nil, errors.New("can't extract info from the token")
			}

			// If the token is not a robot account token, we don't need to do anything
			if authInfo.ProviderKey != attjwtmiddleware.RobotAccountProviderKey {
				return handler(ctx, req)
			}

			claims, ok := authInfo.Claims.(*robotaccount.CustomClaims)
			if !ok {
				return nil, errors.New("error mapping the claims")
			}

			// Do not accept tokens that are crafted for a different audience in this system
			// NOTE: we allow deprecated audience to not to break compatibility with previously issued robot-accounts
			if !claims.VerifyAudience(robotaccount.Audience, true) && !claims.VerifyAudience(robotaccount.DeprecatedAudience, true) {
				return nil, errors.New("unexpected token, invalid audience")
			}

			// Extract account ID
			robotAccountID := claims.ID
			if robotAccountID == "" {
				return nil, errors.New("error retrieving the key ID from the auth token")
			}

			// Check that the robot account exists and is not revoked
			account, err := robotAccountUseCase.FindByID(ctx, robotAccountID)
			if err != nil {
				return nil, err
			}

			if account == nil {
				logger.Infof("robot account not found with id %s", robotAccountID)
				return nil, errors.New("robot account not found")
			}

			if account.RevokedAt != nil {
				logger.Infof("robot account revoked %s", robotAccountID)
				return nil, errors.New("robot account revoked")
			}

			workflowID := claims.WorkflowID
			if workflowID == "" {
				return nil, errors.New("error retrieving the workflow from the auth token")
			}

			orgID := claims.OrgID
			if orgID == "" {
				return nil, errors.New("error retrieving the organization from the auth token")
			}

			org, err := orgUseCase.FindByID(ctx, orgID)
			if err != nil {
				return nil, err
			}

			ctx = WithCurrentOrg(ctx, &Org{Name: org.Name, ID: org.ID, CreatedAt: org.CreatedAt})

			// Check that the encoded workflow ID is the one associated with the robot account
			// NOTE: This in theory should not be necessary since currently we allow a robot account to be attached to ONLY ONE workflowID
			if account.WorkflowID.String() != workflowID {
				logger.Info("workflow mismatch")
				return nil, errors.New("workflow mismatch")
			}

			// Set the robot account in the context
			ctx = withRobotAccount(ctx, &RobotAccount{
				ID: account.ID.String(), WorkflowID: workflowID, OrgID: orgID, ProviderKey: authInfo.ProviderKey,
			})

			return handler(ctx, req)
		}
	}
}
