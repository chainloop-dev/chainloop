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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type Membership struct {
	ID, OrganizationID   uuid.UUID
	Current              bool
	CreatedAt, UpdatedAt *time.Time
	Org                  *Organization
	User                 *User
	Role                 authz.Role
}

type MembershipRepo interface {
	FindByUser(ctx context.Context, userID uuid.UUID) ([]*Membership, error)
	FindByOrg(ctx context.Context, orgID uuid.UUID) ([]*Membership, error)
	FindByIDInUser(ctx context.Context, userID, ID uuid.UUID) (*Membership, error)
	FindByIDInOrg(ctx context.Context, orgID, ID uuid.UUID) (*Membership, error)
	FindByOrgAndUser(ctx context.Context, orgID, userID uuid.UUID) (*Membership, error)
	SetCurrent(ctx context.Context, ID uuid.UUID) (*Membership, error)
	SetRole(ctx context.Context, ID uuid.UUID, role authz.Role) (*Membership, error)
	Create(ctx context.Context, orgID, userID uuid.UUID, current bool, role authz.Role) (*Membership, error)
	Delete(ctx context.Context, ID uuid.UUID) error
}

type MembershipUseCase struct {
	repo       MembershipRepo
	orgUseCase *OrganizationUseCase
	logger     *log.Helper
}

func NewMembershipUseCase(repo MembershipRepo, orgUC *OrganizationUseCase, logger log.Logger) *MembershipUseCase {
	return &MembershipUseCase{repo, orgUC, log.NewHelper(logger)}
}

// LeaveAndDeleteOrg deletes a membership (and the org i) from the database associated with the current user
// and the associated org if the user is the only member
func (uc *MembershipUseCase) LeaveAndDeleteOrg(ctx context.Context, userID, membershipID string) error {
	membershipUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// Check that the provided membershipID in fact belongs to a membership from the user
	m, err := uc.repo.FindByIDInUser(ctx, userUUID, membershipUUID)
	if err != nil {
		return fmt.Errorf("failed to find membership: %w", err)
	} else if m == nil {
		return NewErrNotFound("membership")
	}

	uc.logger.Infow("msg", "Deleting membership", "user_id", userID, "membership_id", m.ID.String())
	if err := uc.repo.Delete(ctx, membershipUUID); err != nil {
		return fmt.Errorf("failed to delete membership: %w", err)
	}

	// Check number of members in the org
	// If it's the only one, delete the org
	membershipsInOrg, err := uc.repo.FindByOrg(ctx, m.OrganizationID)
	if err != nil {
		return fmt.Errorf("failed to find memberships in org: %w", err)
	}

	if len(membershipsInOrg) == 0 {
		// Delete the org
		uc.logger.Infow("msg", "Deleting organization", "organization_id", m.OrganizationID.String())
		if err := uc.orgUseCase.Delete(ctx, m.OrganizationID.String()); err != nil {
			return fmt.Errorf("failed to delete org: %w", err)
		}
	}

	return nil
}

// DeleteOther just deletes a membership from the database
// but ensures that the user is not deleting itself from the org
func (uc *MembershipUseCase) DeleteOther(ctx context.Context, orgID, userID, membershipID string) error {
	membershipUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	m, err := uc.repo.FindByIDInOrg(ctx, orgUUID, membershipUUID)
	if err != nil {
		return fmt.Errorf("failed to find membership: %w", err)
	} else if m == nil {
		return NewErrNotFound("membership")
	}

	if m.User.ID == userID {
		return NewErrValidationStr("cannot delete yourself from the org")
	}

	uc.logger.Infow("msg", "Deleting membership", "org_id", orgID, "membership_id", m.ID.String())
	if err := uc.repo.Delete(ctx, membershipUUID); err != nil {
		return fmt.Errorf("failed to delete membership: %w", err)
	}

	return nil
}

func (uc *MembershipUseCase) UpdateRole(ctx context.Context, orgID, userID, membershipID string, role authz.Role) (*Membership, error) {
	// If it has ben overrode by the user, validate it
	if role == "" {
		return nil, NewErrValidationStr("role is required")
	}

	membershipUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	m, err := uc.repo.FindByIDInOrg(ctx, orgUUID, membershipUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to find membership: %w", err)
	} else if m == nil {
		return nil, NewErrNotFound("membership")
	}

	if m.User.ID == userID {
		return nil, NewErrValidationStr("cannot update yourself")
	}

	return uc.repo.SetRole(ctx, membershipUUID, role)
}

type membershipCreateOpts struct {
	current bool
	role    authz.Role
}

type MembershipCreateOpt func(*membershipCreateOpts)

func WithCurrentMembership() MembershipCreateOpt {
	return func(o *membershipCreateOpts) {
		o.current = true
	}
}

func WithMembershipRole(r authz.Role) MembershipCreateOpt {
	return func(o *membershipCreateOpts) {
		o.role = r
	}
}

func (uc *MembershipUseCase) Create(ctx context.Context, orgID, userID string, opts ...MembershipCreateOpt) (*Membership, error) {
	cp := &membershipCreateOpts{
		// Default role
		role: authz.RoleViewer,
	}

	for _, o := range opts {
		o(cp)
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	m, err := uc.repo.Create(ctx, orgUUID, userUUID, cp.current, cp.role)
	if err != nil {
		return nil, fmt.Errorf("failed to create membership: %w", err)
	}

	if !cp.current {
		return m, nil
	}

	// Set the current membership again to make sure we uncheck the previous ones
	return uc.repo.SetCurrent(ctx, m.ID)
}

func (uc *MembershipUseCase) ByUser(ctx context.Context, userID string) ([]*Membership, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.FindByUser(ctx, userUUID)
}

func (uc *MembershipUseCase) ByOrg(ctx context.Context, orgID string) ([]*Membership, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.FindByOrg(ctx, orgUUID)
}

// SetCurrent sets the current membership for the user
// and unsets the previous one
func (uc *MembershipUseCase) SetCurrent(ctx context.Context, userID, membershipID string) (*Membership, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	mUUID, err := uuid.Parse(membershipID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Check that the provided membershipID in fact refers to one from this user
	if m, err := uc.repo.FindByIDInUser(ctx, userUUID, mUUID); err != nil {
		return nil, err
	} else if m == nil {
		return nil, NewErrNotFound("membership")
	}

	return uc.repo.SetCurrent(ctx, mUUID)
}

func (uc *MembershipUseCase) FindByOrgAndUser(ctx context.Context, orgID, userID string) (*Membership, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	m, err := uc.repo.FindByOrgAndUser(ctx, orgUUID, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to find membership: %w", err)
	} else if m == nil {
		return nil, NewErrNotFound("membership")
	}

	return m, nil
}
