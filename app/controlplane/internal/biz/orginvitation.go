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

type OrgInvitationUseCase struct {
	logger   *log.Helper
	repo     OrgInvitationRepo
	mRepo    MembershipRepo
	userRepo UserRepo
}

type OrgInvitation struct {
	ID            uuid.UUID
	Org           *Organization
	Sender        *User
	ReceiverEmail string
	CreatedAt     *time.Time
	Status        OrgInvitationStatus
}

type OrgInvitationRepo interface {
	Create(ctx context.Context, orgID, senderID uuid.UUID, receiverEmail string) (*OrgInvitation, error)
	FindByID(ctx context.Context, ID uuid.UUID) (*OrgInvitation, error)
	PendingInvitation(ctx context.Context, orgID uuid.UUID, receiverEmail string) (*OrgInvitation, error)
	PendingInvitations(ctx context.Context, receiverEmail string) ([]*OrgInvitation, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListBySender(ctx context.Context, sender uuid.UUID) ([]*OrgInvitation, error)
	ChangeStatus(ctx context.Context, ID uuid.UUID, status OrgInvitationStatus) error
}

func NewOrgInvitationUseCase(r OrgInvitationRepo, mRepo MembershipRepo, uRepo UserRepo, l log.Logger) (*OrgInvitationUseCase, error) {
	return &OrgInvitationUseCase{
		logger: servicelogger.ScopedHelper(l, "biz/orgInvitation"),
		repo:   r, mRepo: mRepo, userRepo: uRepo,
	}, nil
}

func (uc *OrgInvitationUseCase) Create(ctx context.Context, orgID, senderID, receiverEmail string) (*OrgInvitation, error) {
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

	// 5 - Check if there is already an invitation for this user for this org
	m, err := uc.repo.PendingInvitation(ctx, orgUUID, receiverEmail)
	if err != nil {
		return nil, fmt.Errorf("error finding invitation for org %s and receiver %s: %w", orgID, receiverEmail, err)
	}

	if m != nil {
		return nil, NewErrValidationStr("invitation already exists for this user and org")
	}

	// 5 - Create the invitation
	invitation, err := uc.repo.Create(ctx, orgUUID, senderUUID, receiverEmail)
	if err != nil {
		return nil, fmt.Errorf("error creating invitation: %w", err)
	}

	return invitation, nil
}

func (uc *OrgInvitationUseCase) ListBySender(ctx context.Context, senderID string) ([]*OrgInvitation, error) {
	senderUUID, err := uuid.Parse(senderID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.ListBySender(ctx, senderUUID)
}

// Revoke an invitation by ID only if the user is the one who created it
func (uc *OrgInvitationUseCase) Revoke(ctx context.Context, senderID, invitationID string) error {
	invitationUUID, err := uuid.Parse(invitationID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// We care only about invitations that are pending and sent by the user
	m, err := uc.repo.FindByID(ctx, invitationUUID)
	if err != nil {
		return fmt.Errorf("error finding invitation %s: %w", invitationID, err)
	} else if m == nil || m.Sender.ID != senderID {
		return NewErrNotFound("invitation")
	}

	if m.Status != OrgInvitationStatusPending {
		return NewErrValidationStr("invitation is not in pending state")
	}

	return uc.repo.SoftDelete(ctx, invitationUUID)
}

// AcceptPendingInvitations accepts all pending invitations for a given user email
func (uc *OrgInvitationUseCase) AcceptPendingInvitations(ctx context.Context, receiverEmail string) error {
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

	// Find all memberships for the user and all pending invitations
	memberships, err := uc.mRepo.FindByUser(ctx, userUUID)
	if err != nil {
		return fmt.Errorf("error finding memberships for user %s: %w", receiverEmail, err)
	}

	invitations, err := uc.repo.PendingInvitations(ctx, receiverEmail)
	if err != nil {
		return fmt.Errorf("error finding pending invitations for user %s: %w", receiverEmail, err)
	}

	uc.logger.Infow("msg", "Checking pending invitations", "user_id", user.ID, "invitations", len(invitations))

	// Iterate on the invitations and create the membership if it doesn't exist
	for _, invitation := range invitations {
		var alreadyMember bool
		for _, m := range memberships {
			if m.OrganizationID.String() == invitation.Org.ID {
				alreadyMember = true
			}
		}

		orgUUID, err := uuid.Parse(invitation.Org.ID)
		if err != nil {
			return NewErrInvalidUUID(err)
		}

		// user is not a member of the org, create the membership
		if !alreadyMember {
			uc.logger.Infow("msg", "Adding member", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", user.ID)
			if _, err := uc.mRepo.Create(ctx, orgUUID, userUUID, false); err != nil {
				return fmt.Errorf("error creating membership for user %s: %w", receiverEmail, err)
			}
		}

		uc.logger.Infow("msg", "Accepting invitation", "invitation_id", invitation.ID.String(), "org_id", invitation.Org.ID, "user_id", user.ID)
		// change the status of the invitation
		if err := uc.repo.ChangeStatus(ctx, invitation.ID, OrgInvitationStatusAccepted); err != nil {
			return fmt.Errorf("error changing status of invitation %s: %w", invitation.ID.String(), err)
		}
	}

	return nil
}

func (uc *OrgInvitationUseCase) AcceptInvitation(ctx context.Context, invitationID string) error {
	invitationUUID, err := uuid.Parse(invitationID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	return uc.repo.ChangeStatus(ctx, invitationUUID, OrgInvitationStatusAccepted)
}

func (uc *OrgInvitationUseCase) FindByID(ctx context.Context, invitationID string) (*OrgInvitation, error) {
	invitationUUID, err := uuid.Parse(invitationID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	invitation, err := uc.repo.FindByID(ctx, invitationUUID)
	if err != nil {
		return nil, fmt.Errorf("error finding invitation %s: %w", invitationID, err)
	} else if invitation == nil {
		return nil, NewErrNotFound("invitation")
	}

	return invitation, nil
}

type OrgInvitationStatus string

var (
	OrgInvitationStatusPending  OrgInvitationStatus = "pending"
	OrgInvitationStatusAccepted OrgInvitationStatus = "accepted"
)

// Implements https://pkg.go.dev/entgo.io/ent/schema/field#EnumValues
func (OrgInvitationStatus) Values() (kinds []string) {
	for _, s := range []OrgInvitationStatus{OrgInvitationStatusAccepted, OrgInvitationStatusPending} {
		kinds = append(kinds, string(s))
	}

	return
}
