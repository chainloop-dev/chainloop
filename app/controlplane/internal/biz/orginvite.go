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

package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OrgInviteUseCase struct {
	logger   *log.Helper
	repo     OrgInviteRepo
	mRepo    MembershipRepo
	userRepo UserRepo
}

type OrgInvite struct {
	ID            uuid.UUID
	OrgID         uuid.UUID
	SenderID      uuid.UUID
	ReceiverEmail string
	CreatedAt     *time.Time
	Status        OrgInviteStatus
}

type OrgInviteRepo interface {
	Create(ctx context.Context, orgID, senderID uuid.UUID, receiverEmail string) (*OrgInvite, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*OrgInvite, error)
	PendingInvite(ctx context.Context, orgID uuid.UUID, receiverEmail string) (*OrgInvite, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListBySender(ctx context.Context, sender uuid.UUID) ([]*OrgInvite, error)
}

func NewOrgInviteUseCase(r OrgInviteRepo, mRepo MembershipRepo, uRepo UserRepo, l log.Logger) (*OrgInviteUseCase, error) {
	return &OrgInviteUseCase{logger: log.NewHelper(l), repo: r, mRepo: mRepo, userRepo: uRepo}, nil
}

func (uc *OrgInviteUseCase) Create(ctx context.Context, orgID, senderID, receiverEmail string) (*OrgInvite, error) {
	// 1 - Static Validation
	if receiverEmail == "" {
		return nil, NewErrValidationStr("receiver email is required")
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	senderUUID, err := uuid.Parse(senderID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// 2 - the sender exists and it's not the same than the receiver of the invitation
	sender, err := uc.userRepo.FindByID(ctx, senderUUID)
	if err != nil {
		return nil, fmt.Errorf("error finding sender %s: %w", senderUUID.String(), err)
	} else if sender == nil {
		return nil, NewErrNotFound("sender")
	}

	if sender.Email == receiverEmail {
		return nil, NewErrValidationStr("sender and receiver emails cannot be the same")
	}

	// 3 - Check if the user has permissions to invite to the org
	memberships, err := uc.mRepo.FindByUser(ctx, senderUUID)
	if err != nil {
		return nil, fmt.Errorf("error finding memberships for user %s: %w", senderUUID.String(), err)
	}

	var hasPermission bool
	for _, m := range memberships {
		if m.OrganizationID == orgUUID {
			// User has permission to invite to this org
			hasPermission = true
			break
		}
	}

	if !hasPermission {
		return nil, NewErrNotFound("user does not have permission to invite to this org")
	}

	// 4 - Check if there is already an invite for this user for this org
	m, err := uc.repo.PendingInvite(ctx, orgUUID, receiverEmail)
	if err != nil {
		return nil, fmt.Errorf("error finding invite for org %s and receiver %s: %w", orgID, receiverEmail, err)
	}

	if m != nil {
		return nil, NewErrValidationStr("invite already exists for this user and org")
	}

	// 5 - Create the invite
	invite, err := uc.repo.Create(ctx, orgUUID, senderUUID, receiverEmail)
	if err != nil {
		return nil, fmt.Errorf("error creating invite: %w", err)
	}

	return invite, nil
}

func (uc *OrgInviteUseCase) ListBySender(ctx context.Context, senderID string) ([]*OrgInvite, error) {
	senderUUID, err := uuid.Parse(senderID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.ListBySender(ctx, senderUUID)
}

// Revoke an invite by ID only if the user is the one who created it
func (uc *OrgInviteUseCase) Revoke(ctx context.Context, senderID, inviteID string) error {
	inviteUUID, err := uuid.Parse(inviteID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	senderUUID, err := uuid.Parse(senderID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	m, err := uc.repo.FindByID(ctx, inviteUUID)
	if err != nil {
		return fmt.Errorf("error finding invite %s: %w", inviteID, err)
	} else if m == nil || m.SenderID != senderUUID {
		return NewErrNotFound("invite")
	}

	return uc.repo.SoftDelete(ctx, inviteUUID)
}

type OrgInviteStatus string

var (
	OrgInviteStatusPending  OrgInviteStatus = "pending"
	OrgInviteStatusAccepted OrgInviteStatus = "accepted"
	OrgInviteStatusDeclined OrgInviteStatus = "declined"
)

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (OrgInviteStatus) Values() (kinds []string) {
	for _, s := range []OrgInviteStatus{OrgInviteStatusAccepted, OrgInviteStatusDeclined, OrgInviteStatusPending} {
		kinds = append(kinds, string(s))
	}

	return
}
