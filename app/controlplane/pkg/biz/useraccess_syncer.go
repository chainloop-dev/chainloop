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
	"time"

	"github.com/go-kratos/kratos/v2/log"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/google/uuid"
)

type UserAccessSyncerUseCase struct {
	logger *log.Helper
	// Repositories
	userRepo UserRepo
	// Configuration
	allowList *conf.Auth_AllowList
}

func NewUserAccessSyncerUseCase(logger log.Logger, userRepo UserRepo, allowList *conf.Auth_AllowList) *UserAccessSyncerUseCase {
	return &UserAccessSyncerUseCase{
		userRepo:  userRepo,
		allowList: allowList,
		logger:    log.NewHelper(log.With(logger, "component", "biz/user_access_syncer")),
	}
}

// StartSyncingUserAccess starts syncing the access restriction status of all users based on the allowlist
func (u *UserAccessSyncerUseCase) StartSyncingUserAccess(ctx context.Context) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			u.logger.Infow("msg", "stopping user access sync")
			return nil
		case <-ticker.C:
			u.logger.Infow("msg", "Syncing user access")

			// Count the number of users with restricted access
			usersWithRestrictedAccess, err := u.userRepo.CountUsersWithRestrictedAccess(ctx)
			if err != nil {
				return fmt.Errorf("count users with restricted access: %w", err)
			}

			// Update the access restriction status of all users based on the allowlist
			if err := u.updateUserAccessBasedOnAllowList(ctx, usersWithRestrictedAccess); err != nil {
				return fmt.Errorf("update user access based on allow list: %w", err)
			}

			u.logger.Infow("msg", "User access synced")
		}
	}
}

// updateUserAccessBasedOnAllowList updates the access restriction status of all users based on the allowlist
func (u *UserAccessSyncerUseCase) updateUserAccessBasedOnAllowList(ctx context.Context, usersWithRestrictedAccess int) error {
	// If the allowlist is empty and there are users with restricted access, give access to those users
	if u.allowList != nil && len(u.allowList.GetRules()) == 0 && usersWithRestrictedAccess > 0 {
		if err := u.userRepo.UpdateAllUsersAccess(ctx, false); err != nil {
			return fmt.Errorf("update all users access: %w", err)
		}
	} else {
		// Sync the access restriction status of all users based on the allowlist
		if err := u.syncUserAccess(ctx); err != nil {
			return fmt.Errorf("sync user access: %w", err)
		}
	}

	return nil
}

// syncUserAccess syncs the access restriction status of all users based on the allowlist
func (u *UserAccessSyncerUseCase) syncUserAccess(ctx context.Context) error {
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

		// If the allowlist is empty, we deactivate the access restriction for all users
		isAllowListDeactivated := u.allowList == nil || len(u.allowList.GetRules()) == 0

		for _, user := range users {
			if err := u.updateUserAccessRestriction(ctx, user, isAllowListDeactivated); err != nil {
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

// updateUserAccessRestriction updates the access restriction status of a user
func (u *UserAccessSyncerUseCase) updateUserAccessRestriction(ctx context.Context, user *User, isAllowListDeactivated bool) error {
	allow, err := UserEmailInAllowlist(u.allowList.GetRules(), user.Email)
	if err != nil {
		return fmt.Errorf("error checking user in allowList: %w", err)
	}

	isAccessRestricted := !allow
	if isAllowListDeactivated {
		isAccessRestricted = false
	}

	parsedUserUUID, err := uuid.Parse(user.ID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	if err := u.userRepo.UpdateAccess(ctx, parsedUserUUID, isAccessRestricted); err != nil {
		return fmt.Errorf("failed to update user access: %w", err)
	}

	return nil
}

// UserEmailInAllowlist checks if the user email is in the allowlist
func UserEmailInAllowlist(allowList []string, email string) (bool, error) {
	for _, allowListEntry := range allowList {
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
