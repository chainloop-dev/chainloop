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
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/v2/log"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/google/uuid"
)

type UserAccessSyncerUseCase struct {
	logger *log.Helper
	// Repositories
	userRepo UserRepo
	// Configuration
	allowList *conf.AllowList
}

func NewUserAccessSyncerUseCase(logger log.Logger, userRepo UserRepo, allowList *conf.AllowList) *UserAccessSyncerUseCase {
	return &UserAccessSyncerUseCase{
		userRepo:  userRepo,
		allowList: allowList,
		logger:    log.NewHelper(log.With(logger, "component", "biz/user_access_syncer")),
	}
}

// SyncUserAccess syncs the access restriction status of all users based on the allowlist into their DB entries
// If allowDbOverrides is true, the access restriction status of users that have the access property set to null will be updated
// If allowDbOverrides is true, the DB entries of all users will be updated to match the allowlist
func (u *UserAccessSyncerUseCase) SyncUserAccess(ctx context.Context) error {
	if u.allowList.GetAllowDbOverrides() {
		return u.reconciliateUsersWithAccessNull(ctx)
	}

	return u.reconciliateAllUsersAccess(ctx)
}

func (u *UserAccessSyncerUseCase) reconciliateUsersWithAccessNull(ctx context.Context) error {
	if hasUsersWithAccessPropertyNull, err := u.userRepo.HasUsersWithAccessPropertyNotSet(ctx); err != nil {
		return fmt.Errorf("count users with access: %w", err)
	} else if !hasUsersWithAccessPropertyNull {
		return nil
	}

	users, err := u.userRepo.FindUsersWithAccessPropertyNotSet(ctx)
	if err != nil {
		return fmt.Errorf("failed to list users: %w", err)
	}

	for _, user := range users {
		if _, err := u.UpdateUserAccessRestriction(ctx, user); err != nil {
			return fmt.Errorf("failed to update user access: %w", err)
		}
	}

	return nil
}

// reconciliateAllUsersAccess syncs the DB entry with the allowlist for all users
func (u *UserAccessSyncerUseCase) reconciliateAllUsersAccess(ctx context.Context) error {
	var (
		offset = 1
		limit  = 50
	)

	for {
		pgOpts, err := pagination.NewOffsetPaginationOpts(offset, limit)
		if err != nil {
			return fmt.Errorf("failed to create pagination options: %w", err)
		}

		users, _, err := u.userRepo.FindAll(ctx, pgOpts)
		if err != nil {
			return fmt.Errorf("failed to list users: %w", err)
		}

		for _, user := range users {
			if _, err := u.UpdateUserAccessRestriction(ctx, user); err != nil {
				return fmt.Errorf("failed to update user access: %w", err)
			}
		}

		if len(users) < limit {
			break
		}

		offset++
	}

	return nil
}

// UpdateUserAccessRestriction updates the access restriction status of a user
func (u *UserAccessSyncerUseCase) UpdateUserAccessRestriction(ctx context.Context, user *User) (*User, error) {
	isAllowListDeactivated := u.allowList == nil || len(u.allowList.GetRules()) == 0

	var hasRestrictedAccess bool

	// If the allowlist is empty, we deactivate the access restriction for all users
	if isAllowListDeactivated {
		hasRestrictedAccess = false
	} else {
		// Check if the user email is in the allowlist and update the access restriction status accordingly
		allow, err := userEmailInAllowlist(u.allowList, user.Email)
		if err != nil {
			return nil, fmt.Errorf("error checking user in allowList: %w", err)
		}

		hasRestrictedAccess = !allow
	}

	parsedUserUUID, err := uuid.Parse(user.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	updatedUser, err := u.userRepo.UpdateAccess(ctx, parsedUserUUID, hasRestrictedAccess)
	if err != nil {
		return nil, fmt.Errorf("failed to update user access: %w", err)
	}

	return updatedUser, nil
}

// userEmailInAllowlist checks if the user email is in the allowlist
func userEmailInAllowlist(allowList *conf.AllowList, email string) (bool, error) {
	for _, allowListEntry := range allowList.GetRules() {
		// it's a direct email match
		if allowListEntry == email {
			return true, nil
		}

		// Check if the entry is a domain and the email is part of it
		// extract the domain from the allowList entry
		// i.e if the entry is @cyberdyne.io, we get cyberdyne.io
		domainComponent := strings.Split(allowListEntry, "@")
		if len(domainComponent) != 2 {
			return false, fmt.Errorf("invalid domain entry: %q", allowListEntry)
		}

		// it's not a domain since it contains an username, then continue
		if domainComponent[0] != "" {
			continue
		}

		// Compare the domains
		emailComponents := strings.Split(email, "@")
		if len(emailComponents) != 2 {
			return false, fmt.Errorf("invalid email: %q", email)
		}

		// check if against a potential domain entry in the allowList
		if emailComponents[1] == domainComponent[1] {
			return true, nil
		}
	}

	return false, nil
}
