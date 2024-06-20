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
	"fmt"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type User struct {
	ID        string
	Email     string
	CreatedAt *time.Time
}

type UserRepo interface {
	CreateByEmail(ctx context.Context, email string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, userID uuid.UUID) (*User, error)
	Delete(ctx context.Context, userID uuid.UUID) error
}

type UserOrgFinder interface {
	FindByID(ctx context.Context, userID string) (*User, error)
	CurrentMembership(ctx context.Context, userID string) (*Membership, error)
}

type UserUseCase struct {
	userRepo            UserRepo
	logger              *log.Helper
	membershipUseCase   *MembershipUseCase
	organizationUseCase *OrganizationUseCase
	onboardingConfig    []*conf.OnboardingSpec
}

type NewUserUseCaseParams struct {
	UserRepo            UserRepo
	MembershipUseCase   *MembershipUseCase
	OrganizationUseCase *OrganizationUseCase
	OnboardingConfig    []*conf.OnboardingSpec
	Logger              log.Logger
}

func NewUserUseCase(opts *NewUserUseCaseParams) *UserUseCase {
	return &UserUseCase{
		userRepo:            opts.UserRepo,
		membershipUseCase:   opts.MembershipUseCase,
		organizationUseCase: opts.OrganizationUseCase,
		onboardingConfig:    opts.OnboardingConfig,
		logger:              log.NewHelper(opts.Logger),
	}
}

// DeleteUser deletes the user, related memberships and organization if needed
func (uc *UserUseCase) DeleteUser(ctx context.Context, userID string) error {
	uc.logger.Infow("msg", "Deleting Account", "user_id", userID)
	memberships, err := uc.membershipUseCase.ByUser(ctx, userID)
	if err != nil {
		return err
	}

	// Iterate on user memberships, delete org if the user is the only member
	for _, m := range memberships {
		if err := uc.membershipUseCase.LeaveAndDeleteOrg(ctx, userID, m.ID.String()); err != nil {
			return fmt.Errorf("failed to delete membership: %w", err)
		}
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	uc.logger.Infow("msg", "User deleted", "user_id", userID)
	return uc.userRepo.Delete(ctx, userUUID)
}

// FindOrCreateByEmail finds or creates a user by email. By default, it will auto-onboard the user
// to the organizations defined in the configuration. If disableAutoOnboarding is set to true, it will
// skip the auto-onboarding process.
func (uc *UserUseCase) FindOrCreateByEmail(ctx context.Context, email string, disableAutoOnboarding ...bool) (*User, error) {
	u, err := uc.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	} else if u != nil {
		return u, nil
	}

	u, err = uc.userRepo.CreateByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Check if we should auto-onboard the user
	if disableAutoOnboarding == nil || (len(disableAutoOnboarding) > 0 && !disableAutoOnboarding[0]) {
		if err := uc.organizationUseCase.AutoOnboardOrganizations(ctx, u.ID); err != nil {
			return nil, fmt.Errorf("failed to auto-onboard user: %w", err)
		}
	}

	return u, err
}

func (uc *UserUseCase) FindByID(ctx context.Context, userID string) (*User, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}
	return uc.userRepo.FindByID(ctx, userUUID)
}

// Find the membership associated with the user that's marked as current
// If none is selected, it will pick the first one and set it as current
func (uc *UserUseCase) CurrentMembership(ctx context.Context, userID string) (*Membership, error) {
	memberships, err := uc.membershipUseCase.ByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// there is no current organization
	if len(memberships) == 0 {
		return nil, NewErrNotFound("user does not have any organization associated")
	}

	for _, m := range memberships {
		// Override if it's being explicitly selected
		if m.Current {
			return m, nil
		}
	}

	// If none is selected, we configure the first one
	m, err := uc.membershipUseCase.SetCurrent(ctx, userID, memberships[0].ID.String())
	if err != nil {
		return nil, fmt.Errorf("error setting current org: %w", err)
	}

	return m, nil
}

func PbRoleToBiz(r pb.MembershipRole) authz.Role {
	switch r {
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_OWNER:
		return authz.RoleOwner
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_ADMIN:
		return authz.RoleAdmin
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_VIEWER:
		return authz.RoleViewer
	default:
		return ""
	}
}
