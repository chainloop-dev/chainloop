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
	"errors"
	"time"

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
	CurrentOrg(ctx context.Context, userID string) (*Organization, error)
}

type UserUseCase struct {
	userRepo            UserRepo
	logger              *log.Helper
	membershipUseCase   *MembershipUseCase
	organizationUseCase *OrganizationUseCase
}

type NewUserUseCaseParams struct {
	UserRepo            UserRepo
	MembershipUseCase   *MembershipUseCase
	OrganizationUseCase *OrganizationUseCase
	Logger              log.Logger
}

func NewUserUseCase(opts *NewUserUseCaseParams) *UserUseCase {
	return &UserUseCase{
		userRepo:            opts.UserRepo,
		membershipUseCase:   opts.MembershipUseCase,
		organizationUseCase: opts.OrganizationUseCase,
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
		membershipsInOrg, err := uc.membershipUseCase.ByOrg(ctx, m.OrganizationID.String())
		if err != nil {
			return err
		}

		uc.logger.Infow("msg", "Deleting membership", "user_id", userID, "membership_id", m.ID.String())
		if err := uc.membershipUseCase.Delete(ctx, m.ID.String()); err != nil {
			return err
		}

		// Check number of members in the org
		// If it's the only one, delete the org
		if len(membershipsInOrg) == 1 {
			// Delete the org
			uc.logger.Infow("msg", "Deleting organization", "organization_id", m.OrganizationID.String())
			if err := uc.organizationUseCase.Delete(ctx, m.OrganizationID.String()); err != nil {
				return err
			}
		}
	}

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	uc.logger.Infow("msg", "User deleted", "user_id", userID)
	return uc.userRepo.Delete(ctx, userUUID)
}

func (uc *UserUseCase) FindOrCreateByEmail(ctx context.Context, email string) (*User, error) {
	u, err := uc.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	} else if u != nil {
		return u, nil
	}

	return uc.userRepo.CreateByEmail(ctx, email)
}

func (uc *UserUseCase) FindByID(ctx context.Context, userID string) (*User, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}
	return uc.userRepo.FindByID(ctx, userUUID)
}

// Find the organization associated with the user that's marked as current
func (uc *UserUseCase) CurrentOrg(ctx context.Context, userID string) (*Organization, error) {
	memberships, err := uc.membershipUseCase.ByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(memberships) == 0 {
		return nil, errors.New("user does not have any organization associated")
	}

	// By default we set the first one
	currentOrg := memberships[0].OrganizationID
	for _, m := range memberships {
		// Override if it's being explicitly selected
		if m.Current {
			currentOrg = m.OrganizationID
			break
		}
	}

	return uc.organizationUseCase.FindByID(ctx, currentOrg.String())
}
