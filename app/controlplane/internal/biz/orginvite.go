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

	"github.com/chainloop-dev/chainloop/internal/servicelogger"
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
	PendingInvites(ctx context.Context, receiverEmail string) ([]*OrgInvite, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListBySender(ctx context.Context, sender uuid.UUID) ([]*OrgInvite, error)
	ChangeStatus(ctx context.Context, ID uuid.UUID, status OrgInviteStatus) error
}

func NewOrgInviteUseCase(r OrgInviteRepo, mRepo MembershipRepo, uRepo UserRepo, l log.Logger) (*OrgInviteUseCase, error) {
	return &OrgInviteUseCase{
		logger: servicelogger.ScopedHelper(l, "biz/orginvite"),
		repo:   r, mRepo: mRepo, userRepo: uRepo,
	}, nil
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

	// 3 - The receiver does not exist in the org already
	memberships, err := uc.mRepo.FindByOrg(ctx, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("error finding memberships for user %s: %w", senderUUID.String(), err)
	}

	for _, m := range memberships {
		if m.UserEmail == receiverEmail {
			return nil, NewErrValidationStr("user already exists in the org")
		}
	}

	// 4 - Check if the user has permissions to invite to the org
	memberships, err = uc.mRepo.FindByUser(ctx, senderUUID)
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

	// 5 - Check if there is already an invite for this user for this org
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

	// We care only about invites that are pending and sent by the user
	m, err := uc.repo.FindByID(ctx, inviteUUID)
	if err != nil {
		return fmt.Errorf("error finding invite %s: %w", inviteID, err)
	} else if m == nil || m.SenderID != senderUUID {
		return NewErrNotFound("invite")
	}

	if m.Status != OrgInviteStatusPending {
		return NewErrValidationStr("invite is not in pending state")
	}

	return uc.repo.SoftDelete(ctx, inviteUUID)
}

// AcceptPendingInvites accepts all pending invites for a given user email
func (uc *OrgInviteUseCase) AcceptPendingInvites(ctx context.Context, receiverEmail string) error {
	user, err := uc.userRepo.FindByEmail(ctx, receiverEmail)
	if err != nil {
		return fmt.Errorf("error finding user %s: %w", receiverEmail, err)
	} else if user == nil {
		return NewErrNotFound("user")
	}

	userUUID, err := uuid.Parse(user.ID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// Find all memberships for the user and all pending invites
	memberships, err := uc.mRepo.FindByUser(ctx, userUUID)
	if err != nil {
		return fmt.Errorf("error finding memberships for user %s: %w", receiverEmail, err)
	}

	invites, err := uc.repo.PendingInvites(ctx, receiverEmail)
	if err != nil {
		return fmt.Errorf("error finding pending invites for user %s: %w", receiverEmail, err)
	}

	uc.logger.Infow("msg", "Checking pending invites", "user_id", user.ID, "invites", len(invites))

	// Iterate on the invites and create the membership if it doesn't exist
	for _, invite := range invites {
		var alreadyMember bool
		for _, m := range memberships {
			if m.OrganizationID == invite.OrgID {
				alreadyMember = true
			}
		}

		// user is not a member of the org, create the membership
		if !alreadyMember {
			uc.logger.Infow("msg", "Adding member", "invite_id", invite.ID.String(), "org_id", invite.OrgID.String(), "user_id", user.ID)
			if _, err := uc.mRepo.Create(ctx, invite.OrgID, userUUID, false); err != nil {
				return fmt.Errorf("error creating membership for user %s: %w", receiverEmail, err)
			}
		}

		uc.logger.Infow("msg", "Accepting invite", "invite_id", invite.ID.String(), "org_id", invite.OrgID.String(), "user_id", user.ID)
		// change the status of the invite
		if err := uc.repo.ChangeStatus(ctx, invite.ID, OrgInviteStatusAccepted); err != nil {
			return fmt.Errorf("error changing status of invite %s: %w", invite.ID.String(), err)
		}
	}

	return nil
}

func (uc *OrgInviteUseCase) AcceptInvite(ctx context.Context, inviteID string) error {
	inviteUUID, err := uuid.Parse(inviteID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	return uc.repo.ChangeStatus(ctx, inviteUUID, OrgInviteStatusAccepted)
}

func (uc *OrgInviteUseCase) FindByID(ctx context.Context, inviteID string) (*OrgInvite, error) {
	inviteUUID, err := uuid.Parse(inviteID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	invite, err := uc.repo.FindByID(ctx, inviteUUID)
	if err != nil {
		return nil, fmt.Errorf("error finding invite %s: %w", inviteID, err)
	} else if invite == nil {
		return nil, NewErrNotFound("invite")
	}

	return invite, nil
}

type OrgInviteStatus string

var (
	OrgInviteStatusPending  OrgInviteStatus = "pending"
	OrgInviteStatusAccepted OrgInviteStatus = "accepted"
)

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (OrgInviteStatus) Values() (kinds []string) {
	for _, s := range []OrgInviteStatus{OrgInviteStatusAccepted, OrgInviteStatusPending} {
		kinds = append(kinds, string(s))
	}

	return
}
