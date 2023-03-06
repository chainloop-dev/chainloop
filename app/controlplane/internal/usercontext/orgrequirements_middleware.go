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

	v1 "github.com/chainloop-dev/bedrock/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz"
	"github.com/go-kratos/kratos/v2/middleware"
)

func CheckOrgRequirements(uc biz.OCIRepositoryReader, logger *log.Helper) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			org := CurrentOrg(ctx)
			if org == nil {
				// Make sure that this middleware is ran after WithCurrentUser
				return nil, errors.New("organization not found")
			}

			// 1 - Figure out main repository for this organization
			repo, err := uc.FindMainRepo(ctx, org.ID)
			if err != nil {
				return nil, fmt.Errorf("checking for repositories in the org: %w", err)
			} else if repo == nil {
				return nil, v1.ErrorOciRepositoryErrorReasonRequired("your organization does not have an OCI repository configured yet")
			}

			// 2 - Perform a validation if needed
			if shouldRevalidate(repo) {
				repo, err = validateRepo(ctx, uc, repo)
				if err != nil {
					return nil, fmt.Errorf("validating repository: %w", err)
				}
			}

			// 2 - compare the status
			if repo.ValidationStatus != biz.OCIRepoValidationOK {
				return nil, v1.ErrorOciRepositoryErrorReasonInvalid("your OCI repository can't be reached")
			}

			return handler(ctx, req)
		}
	}
}

// validateRepoIfNeeded will re-run a validation and return the updated repository
func validateRepo(ctx context.Context, uc biz.OCIRepositoryReader, repo *biz.OCIRepository) (*biz.OCIRepository, error) {
	// re-run the validation
	if err := uc.PerformValidation(ctx, repo.ID); err != nil {
		return nil, fmt.Errorf("performing validation: %w", err)
	}

	// Reload repository to get the updated validation status
	repo, err := uc.FindByID(ctx, repo.ID)
	if err != nil {
		return nil, fmt.Errorf("reloading repository: %w", err)
	}

	return repo, nil
}

const validationTimeOffset = 5 * time.Minute

// Since this check happens synchronously on every request it has a big performance impact
// that's why we run it only in refresh windows
func shouldRevalidate(repo *biz.OCIRepository) bool {
	// If the validation is currently failed we want to make sure we re-validate
	if repo.ValidationStatus == biz.OCIRepoValidationFailed {
		return true
	}

	// if it has been more than validationTimeOffset since the last validation
	return repo.ValidatedAt.Before(time.Now().Add(-validationTimeOffset))
}
