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
	logger *log.Helper
	repo   OrgInviteRepo
	mRepo  MembershipRepo
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
	PendingInvite(ctx context.Context, orgID uuid.UUID, receiverEmail string) (*OrgInvite, error)
}

func NewOrgInviteUseCase(r OrgInviteRepo, mRepo MembershipRepo, l log.Logger) (*OrgInviteUseCase, error) {
	return &OrgInviteUseCase{logger: log.NewHelper(l), repo: r, mRepo: mRepo}, nil
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

	// 2 - Check if the user has permissions to invite to the org
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
		return nil, NewErrUnauthorizedStr("user does not have permission to invite to this org")
	}

	// 3 - Check if there is already an invite for this user for this org
	m, err := uc.repo.PendingInvite(ctx, orgUUID, receiverEmail)
	if err != nil {
		return nil, fmt.Errorf("error finding invite for org %s and receiver %s: %w", orgID, receiverEmail, err)
	}

	if m != nil {
		return nil, NewErrValidationStr("invite already exists for this user and org")
	}

	// 4 - Create the invite
	invite, err := uc.repo.Create(ctx, orgUUID, senderUUID, receiverEmail)
	if err != nil {
		return nil, fmt.Errorf("error creating invite: %w", err)
	}

	return invite, nil
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
