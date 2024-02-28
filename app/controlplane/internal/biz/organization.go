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
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/validation"
)

type Organization struct {
	ID, Name  string
	CreatedAt *time.Time
}

type OrganizationRepo interface {
	FindByID(ctx context.Context, orgID uuid.UUID) (*Organization, error)
	Create(ctx context.Context, name string) (*Organization, error)
	Update(ctx context.Context, id uuid.UUID, name *string) (*Organization, error)
	Delete(ctx context.Context, ID uuid.UUID) error
}

type OrganizationUseCase struct {
	orgRepo           OrganizationRepo
	logger            *log.Helper
	casBackendUseCase *CASBackendUseCase
	integrationUC     *IntegrationUseCase
	membershipRepo    MembershipRepo
}

func NewOrganizationUseCase(repo OrganizationRepo, repoUC *CASBackendUseCase, iUC *IntegrationUseCase, mRepo MembershipRepo, logger log.Logger) *OrganizationUseCase {
	return &OrganizationUseCase{orgRepo: repo,
		logger:            log.NewHelper(logger),
		casBackendUseCase: repoUC,
		integrationUC:     iUC,
		membershipRepo:    mRepo,
	}
}

const OrganizationRandomNameMaxTries = 10

func (uc *OrganizationUseCase) CreateWithRandomName(ctx context.Context) (*Organization, error) {
	// Try 10 times to create a random name
	for i := 0; i < OrganizationRandomNameMaxTries; i++ {
		// Create a random name
		name, err := generateRandomName()
		if err != nil {
			return nil, fmt.Errorf("failed to generate random name: %w", err)
		}

		org, err := uc.doCreate(ctx, name)
		if err != nil {
			// We retry if the organization already exists
			if errors.Is(err, ErrAlreadyExists) {
				uc.logger.Debugw("msg", "Org exists!", "name", name)
				continue
			}
			uc.logger.Debugw("msg", "BOOM", "name", name, "err", err)
			return nil, err
		}

		return org, nil
	}

	return nil, errors.New("failed to create a random organization name")
}

// Create an organization with the given name
func (uc *OrganizationUseCase) Create(ctx context.Context, name string) (*Organization, error) {
	org, err := uc.doCreate(ctx, name)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return nil, NewErrValidationStr("organization already exists")
		}

		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return org, nil
}

var errOrgName = errors.New("org names must only contain lowercase letters, numbers, or hyphens. Examples of valid org names are \"myorg\", \"myorg-123\"")

func (uc *OrganizationUseCase) doCreate(ctx context.Context, name string) (*Organization, error) {
	uc.logger.Infow("msg", "Creating organization", "name", name)

	if err := ValidateOrgName(name); err != nil {
		return nil, NewErrValidation(errOrgName)
	}

	org, err := uc.orgRepo.Create(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	// Create inline CAS-backend
	if _, err := uc.casBackendUseCase.CreateInlineFallbackBackend(ctx, org.ID); err != nil {
		return nil, fmt.Errorf("failed to create fallback backend: %w", err)
	}

	return org, nil
}

func ValidateOrgName(name string) error {
	// The same validation done by Kubernetes for their namespace name
	// https://github.com/kubernetes/apimachinery/blob/fa98d6eaedb4caccd69fc07d90bbb6a1e551f00f/pkg/api/validation/generic.go#L63
	err := validation.IsDNS1123Label(name)
	if len(err) > 0 {
		errMsg := ""
		for _, e := range err {
			errMsg += e + "\n"
		}

		return errors.New(errMsg)
	}

	return nil
}

func (uc *OrganizationUseCase) Update(ctx context.Context, userID, orgID string, name *string) (*Organization, error) {
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// We validate the name to get ready for the name to become identifiers
	if name != nil {
		if err := ValidateOrgName(*name); err != nil {
			return nil, NewErrValidation(errOrgName)
		}
	}

	// Make sure that the organization exists and that the user is a member of it
	membership, err := uc.membershipRepo.FindByOrgAndUser(ctx, orgUUID, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to find memberships: %w", err)
	} else if membership == nil {
		return nil, NewErrNotFound("organization")
	}

	// Perform the update
	org, err := uc.orgRepo.Update(ctx, orgUUID, name)
	if err != nil {
		if errors.Is(err, ErrAlreadyExists) {
			return nil, NewErrValidationStr("an organization with that name already exists")
		}

		return nil, fmt.Errorf("failed to update organization: %w", err)
	} else if org == nil {
		return nil, NewErrNotFound("organization")
	}

	return org, nil
}

func (uc *OrganizationUseCase) FindByID(ctx context.Context, id string) (*Organization, error) {
	orgUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	org, err := uc.orgRepo.FindByID(ctx, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to find organization: %w", err)
	} else if org == nil {
		return nil, NewErrNotFound("organization")
	}

	return org, nil
}

// Delete deletes an organization and all relevant data
// This includes:
// - The organization
// - The associated repositories
// - The associated integrations
// The reason for just deleting these two associated components only is because
// they have external secrets that need to be deleted as well, and for that we leverage their own delete methods
// The rest of the data gets removed by the database cascade delete
func (uc *OrganizationUseCase) Delete(ctx context.Context, id string) error {
	orgUUID, err := uuid.Parse(id)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	org, err := uc.orgRepo.FindByID(ctx, orgUUID)
	if err != nil {
		return err
	} else if org == nil {
		return NewErrNotFound("organization")
	}

	// Delete all the integrations
	integrations, err := uc.integrationUC.List(ctx, id)
	if err != nil {
		return err
	}

	for _, i := range integrations {
		if err := uc.integrationUC.Delete(ctx, id, i.ID.String()); err != nil {
			return err
		}
	}

	backends, err := uc.casBackendUseCase.List(ctx, org.ID)
	if err != nil {
		return fmt.Errorf("failed to list backends: %w", err)
	}

	for _, b := range backends {
		if err := uc.casBackendUseCase.Delete(ctx, b.ID.String()); err != nil {
			return fmt.Errorf("failed to delete backend: %w", err)
		}
	}

	// Delete the organization
	return uc.orgRepo.Delete(ctx, orgUUID)
}
