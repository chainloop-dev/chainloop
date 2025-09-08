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

package usercontext

import (
	"context"
	"errors"
	"fmt"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/go-kratos/kratos/v2/middleware"
)

const validationTimeOffset = 5 * time.Minute

// ValidateCASBackend checks that the current organization has a valid CAS Backend configured
// If the last validation happened more than validationTimeOffset ago it will re-run the validation
// This middleware does not block the request if the CAS Backend is not valid
func ValidateCASBackend(uc biz.CASBackendReader) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			org := entities.CurrentOrg(ctx)
			if org == nil {
				// Make sure that this middleware is ran after WithCurrentUser
				return nil, errors.New("organization not found")
			}

			// 1 - Figure out main repository for this organization
			repo, err := uc.FindDefaultBackend(ctx, org.ID)
			if err != nil && !biz.IsNotFound(err) {
				return nil, fmt.Errorf("checking for CAS backends in the org: %w", err)
			} else if repo == nil {
				return nil, v1.ErrorCasBackendErrorReasonRequired("your organization does not have a CAS Backend configured yet")
			}

			// 2 - Perform a validation if needed
			if shouldRevalidate(repo) {
				repo, err = validateCASBackend(ctx, uc, repo)
				if err != nil {
					return nil, fmt.Errorf("validating CAS backend: %w", err)
				}
			}

			return handler(ctx, req)
		}
	}
}

// validateRepoIfNeeded will re-run a validation and return the updated repository
func validateCASBackend(ctx context.Context, uc biz.CASBackendReader, repo *biz.CASBackend) (*biz.CASBackend, error) {
	// re-run the validation
	if err := uc.PerformValidation(ctx, repo.ID.String()); err != nil {
		return nil, fmt.Errorf("performing validation: %w", err)
	}

	// Reload repository to get the updated validation status
	repo, err := uc.FindByIDInOrg(ctx, repo.OrganizationID.String(), repo.ID.String())
	if err != nil {
		return nil, fmt.Errorf("reloading CAS backend: %w", err)
	}

	return repo, nil
}

// shouldRevalidate returns true if the repo needs to be re-validated
// Currently, we only re-validate if the last status was "failed"
// or if the last validation was more than validationTimeOffset ago
func shouldRevalidate(repo *biz.CASBackend) bool {
	// If the validation is currently failed we want to make sure we re-validate
	if repo.ValidationStatus == biz.CASBackendValidationFailed {
		return true
	}

	// if it has been more than validationTimeOffset since the last validation
	return repo.ValidatedAt.Before(time.Now().Add(-validationTimeOffset))
}

// BlockIfCASBackendNotValid checks that the current organization has a valid CAS Backend configured
// If the CAS Backend is not valid it will block the request
func BlockIfCASBackendNotValid(uc biz.CASBackendReader) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			org := entities.CurrentOrg(ctx)
			if org == nil {
				// Make sure that this middleware is ran after WithCurrentUser
				return nil, errors.New("organization not found")
			}

			// 1 - Figure out main repository for this organization
			repo, err := uc.FindDefaultBackend(ctx, org.ID)
			if err != nil && !biz.IsNotFound(err) {
				return nil, fmt.Errorf("checking for CAS backends in the org: %w", err)
			} else if repo == nil {
				return nil, v1.ErrorCasBackendErrorReasonRequired("your organization does not have a CAS Backend configured yet")
			}

			// 2 - compare the status
			if repo.ValidationStatus != biz.CASBackendValidationOK {
				return nil, v1.ErrorCasBackendErrorReasonInvalid("your CAS backend can't be reached")
			}
			return handler(ctx, req)
		}
	}
}
