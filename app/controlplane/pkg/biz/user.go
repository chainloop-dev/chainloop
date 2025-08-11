//
// Copyright 2024-2025 The Chainloop Authors.
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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	config "github.com/chainloop-dev/chainloop/app/controlplane/pkg/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type User struct {
	ID                  string
	FirstName           string
	LastName            string
	Email               string
	CreatedAt           *time.Time
	UpdatedAt           *time.Time
	HasRestrictedAccess *bool
}

type UserRepo interface {
	CreateByEmail(ctx context.Context, email string, firstName, lastName *string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, userID uuid.UUID) (*User, error)
	Delete(ctx context.Context, userID uuid.UUID) error
	FindAll(ctx context.Context, pagination *pagination.OffsetPaginationOpts) ([]*User, int, error)
	UpdateAccess(ctx context.Context, userID uuid.UUID, isAccessRestricted bool) (*User, error)
	UpdateNameAndLastName(ctx context.Context, userID uuid.UUID, firstName, lastName *string) (*User, error)
	HasUsersWithAccessPropertyNotSet(ctx context.Context) (bool, error)
	FindUsersWithAccessPropertyNotSet(ctx context.Context) ([]*User, error)
}

type UserOrgFinder interface {
	FindByID(ctx context.Context, userID string) (*User, error)
	CurrentMembership(ctx context.Context, userID string) (*Membership, error)
	MembershipInOrg(ctx context.Context, userID string, orgName string) (*Membership, error)
}

type UserUseCase struct {
	userRepo            UserRepo
	logger              *log.Helper
	membershipUseCase   *MembershipUseCase
	organizationUseCase *OrganizationUseCase
	onboardingConfig    []*config.OnboardingSpec
	auditorUC           *AuditorUseCase
	userAccessSyncer    *UserAccessSyncerUseCase
}

type NewUserUseCaseParams struct {
	UserRepo            UserRepo
	MembershipUseCase   *MembershipUseCase
	OrganizationUseCase *OrganizationUseCase
	OnboardingConfig    []*config.OnboardingSpec
	Logger              log.Logger
	AuditorUseCase      *AuditorUseCase
	UserAccessSyncer    *UserAccessSyncerUseCase
}

func NewUserUseCase(opts *NewUserUseCaseParams) *UserUseCase {
	return &UserUseCase{
		userRepo:            opts.UserRepo,
		membershipUseCase:   opts.MembershipUseCase,
		organizationUseCase: opts.OrganizationUseCase,
		onboardingConfig:    opts.OnboardingConfig,
		logger:              log.NewHelper(opts.Logger),
		auditorUC:           opts.AuditorUseCase,
		userAccessSyncer:    opts.UserAccessSyncer,
	}
}

// DeleteUser deletes the user, related memberships and organization if needed
// Safe approach: blocks deletion if user is sole owner of any organizations
func (uc *UserUseCase) DeleteUser(ctx context.Context, userID string) error {
	uc.logger.Infow("msg", "Deleting Account", "user_id", userID)

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	memberships, err := uc.membershipUseCase.ByUser(ctx, userID)
	if err != nil {
		return err
	}

	// Check if user is the sole owner of any organizations
	soleOwnerOrgs := []string{}
	for _, m := range memberships {
		isSoleOwner, err := uc.membershipUseCase.isUserSoleOwner(ctx, m.OrganizationID, userUUID)
		if err != nil {
			return fmt.Errorf("failed to check ownership for org %s: %w", m.Org.Name, err)
		}
		if isSoleOwner {
			soleOwnerOrgs = append(soleOwnerOrgs, m.Org.Name)
		}
	}

	// Block deletion if user is sole owner of any organizations
	if len(soleOwnerOrgs) > 0 {
		return NewErrValidationStr(fmt.Sprintf("cannot delete account: you are the sole owner of organizations (%s). Please transfer ownership or delete these organizations first", strings.Join(soleOwnerOrgs, ", ")))
	}

	// Safely leave all organizations (user is not sole owner of any)
	for _, m := range memberships {
		if err := uc.membershipUseCase.Leave(ctx, userID, m.ID.String()); err != nil {
			return fmt.Errorf("failed to leave organization %s: %w", m.Org.Name, err)
		}
	}

	uc.logger.Infow("msg", "User deleted", "user_id", userID)
	return uc.userRepo.Delete(ctx, userUUID)
}

type UpsertByEmailOpts struct {
	// DisableAutoOnboarding, if set to true, will skip the auto-onboarding process
	DisableAutoOnboarding *bool
	FirstName             *string
	LastName              *string
	SSOGroups             []string
}

// UpsertByEmail finds or creates a user by email. By default, it will auto-onboard the user
// to the organizations defined in the configuration. If disableAutoOnboarding is set to true, it will
// skip the auto-onboarding process.
func (uc *UserUseCase) UpsertByEmail(ctx context.Context, email string, opts *UpsertByEmailOpts) (*User, error) {
	if opts == nil {
		opts = &UpsertByEmailOpts{}
	}
	// emails are case-insensitive
	email = strings.ToLower(email)
	u, err := uc.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	// Signup case
	if u == nil {
		u, err = uc.userRepo.CreateByEmail(ctx, email, opts.FirstName, opts.LastName)
		if err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}

		// set the context user so it can be used in the auditor
		ctx = entities.WithCurrentUser(ctx, &entities.User{Email: u.Email, ID: u.ID, FirstName: u.FirstName, LastName: u.LastName})
		uc.auditorUC.Dispatch(ctx, &events.UserSignedUp{
			UserBase: &events.UserBase{
				UserID:    ToPtr(uuid.MustParse(u.ID)),
				Email:     u.Email,
				SSOGroups: opts.SSOGroups,
			},
		}, nil)
	} else {
		// Login case
		ctx = entities.WithCurrentUser(ctx, &entities.User{Email: u.Email, ID: u.ID, FirstName: u.FirstName, LastName: u.LastName})
		uc.auditorUC.Dispatch(ctx, &events.UserLoggedIn{
			UserBase: &events.UserBase{
				UserID:    ToPtr(uuid.MustParse(u.ID)),
				Email:     u.Email,
				SSOGroups: opts.SSOGroups,
			},
			LoggedIn: time.Now(),
		}, nil)
	}

	// Update the user's first and last name if they differ from the provided options
	if (opts.FirstName != nil && u.FirstName != *opts.FirstName) || (opts.LastName != nil && u.LastName != *opts.LastName) {
		u, err = uc.userRepo.UpdateNameAndLastName(ctx, uuid.MustParse(u.ID), opts.FirstName, opts.LastName)
		if err != nil {
			return nil, fmt.Errorf("failed to update user name: %w", err)
		}
	}

	// Auto-onboard the user to the organizations defined in the configuration
	if opts.DisableAutoOnboarding == nil || !*opts.DisableAutoOnboarding {
		if err := uc.organizationUseCase.AutoOnboardOrganizations(ctx, u.ID); err != nil {
			return nil, fmt.Errorf("failed to auto-onboard user: %w", err)
		}
	}

	// Update the access restriction status initial value based on an pre-existing allowlist
	u, err = uc.userAccessSyncer.UpdateUserAccessRestriction(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("failed to update user access: %w", err)
	}

	return u, nil
}

func (uc *UserUseCase) FindByID(ctx context.Context, userID string) (*User, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}
	return uc.userRepo.FindByID(ctx, userUUID)
}

func (uc *UserUseCase) MembershipInOrg(ctx context.Context, userID string, orgName string) (*Membership, error) {
	m, err := uc.membershipUseCase.FindByOrgNameAndUser(ctx, orgName, userID)
	if err != nil {
		return nil, err
	} else if m == nil {
		return nil, NewErrNotFound("user does not have this org associated")
	}

	return m, nil
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
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_MEMBER:
		return authz.RoleOrgMember
	case pb.MembershipRole_MEMBERSHIP_ROLE_ORG_CONTRIBUTOR:
		return authz.RoleOrgContributor
	default:
		return ""
	}
}
