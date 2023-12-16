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
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/golang-jwt/jwt/v4"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/jwt/user"
	"github.com/go-kratos/kratos/v2/middleware"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
)

// Utils to get and set information from context
type User struct {
	Email, ID string
	CreatedAt *time.Time
}

type Org struct {
	ID, Name  string
	CreatedAt *time.Time
}

func WithCurrentUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, currentUserCtxKey{}, user)
}

// RequestID tries to retrieve requestID from the given context.
// If it doesn't exist, an empty string is returned.
func CurrentUser(ctx context.Context) *User {
	res := ctx.Value(currentUserCtxKey{})
	if res == nil {
		return nil
	}
	return res.(*User)
}

func WithCurrentOrg(ctx context.Context, org *Org) context.Context {
	return context.WithValue(ctx, currentOrgCtxKey{}, org)
}

// RequestID tries to retrieve requestID from the given context.
// If it doesn't exist, an empty string is returned.
func CurrentOrg(ctx context.Context) *Org {
	res := ctx.Value(currentOrgCtxKey{})
	if res == nil {
		return nil
	}
	return res.(*Org)
}

type currentUserCtxKey struct{}
type currentOrgCtxKey struct{}

// Middleware that injects the current user + organization to the context
func WithCurrentUserAndOrgMiddleware(userUseCase biz.UserOrgFinder, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			rawClaims, ok := jwtMiddleware.FromContext(ctx)
			// If not found means that there is no currentUser set in the context
			if !ok {
				logger.Warn("couldn't extract org/user, JWT parser middleware not running before this one?")
				return nil, errors.New("can't extract JWT info from the context")
			}

			genericClaims, ok := rawClaims.(jwt.MapClaims)
			if !ok {
				return nil, errors.New("error mapping the claims")
			}

			// Check wether the token is for a user or an API-token and handle accordingly
			// We've received a token for a user
			if genericClaims.VerifyAudience(user.Audience, true) {
				userID, ok := genericClaims["user_id"].(string)
				if !ok || userID == "" {
					return nil, errors.New("error mapping the user claims")
				}

				var err error
				ctx, err = setCurrentOrgAndUser(ctx, userUseCase, userID, logger)
				if err != nil {
					return nil, fmt.Errorf("error setting current org and user: %w", err)
				}

				logger.Infow("msg", "[authN] processed credentials", "id", userID, "type", "user")
			}

			return handler(ctx, req)
		}
	}
}

// Find organization and user in DB
func setCurrentOrgAndUser(ctx context.Context, userUC biz.UserOrgFinder, userID string, logger *log.Helper) (context.Context, error) {
	u, err := userUC.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if u == nil {
		logger.Warnf("user with id %s not found", userID)
		return nil, errors.New("user not found")
	}

	// We load the current organization
	org, err := userUC.CurrentOrg(ctx, userID)
	if err != nil {
		return nil, err
	}

	if org == nil {
		logger.Warnf("user with id %s has no current organization", userID)
		return nil, errors.New("org not found")
	}

	ctx = WithCurrentOrg(ctx, &Org{Name: org.Name, ID: org.ID, CreatedAt: org.CreatedAt})
	ctx = WithCurrentUser(ctx, &User{Email: u.Email, ID: u.ID, CreatedAt: u.CreatedAt})

	return ctx, nil
}
