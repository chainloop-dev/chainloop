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
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type Membership struct {
	ID, UserID, OrganizationID uuid.UUID
	UserEmail                  string
	Current                    bool
	CreatedAt, UpdatedAt       *time.Time
	Org                        *Organization
}

type MembershipRepo interface {
	FindByUser(ctx context.Context, userID uuid.UUID) ([]*Membership, error)
	FindByOrg(ctx context.Context, orgID uuid.UUID) ([]*Membership, error)
	FindByIDInUser(ctx context.Context, userID, ID uuid.UUID) (*Membership, error)
	SetCurrent(ctx context.Context, ID uuid.UUID) (*Membership, error)
	Create(ctx context.Context, orgID, userID uuid.UUID, current bool) (*Membership, error)
	Delete(ctx context.Context, ID uuid.UUID) error
}

type MembershipUseCase struct {
	repo   MembershipRepo
	logger *log.Helper
}

func NewMembershipUseCase(repo MembershipRepo, logger log.Logger) *MembershipUseCase {
	return &MembershipUseCase{repo, log.NewHelper(logger)}
}

func (uc *MembershipUseCase) Delete(ctx context.Context, id string) error {
	uuid, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	return uc.repo.Delete(ctx, uuid)
}

func (uc *MembershipUseCase) Create(ctx context.Context, orgID, userID string, current bool) (*Membership, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.repo.Create(ctx, orgUUID, userUUID, current)
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
