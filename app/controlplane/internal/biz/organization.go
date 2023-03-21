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
	"github.com/moby/moby/pkg/namesgenerator"
)

type Organization struct {
	ID, Name  string
	CreatedAt *time.Time
}

type OrganizationRepo interface {
	FindByID(ctx context.Context, orgID uuid.UUID) (*Organization, error)
	Create(ctx context.Context, name string) (*Organization, error)
	Delete(ctx context.Context, ID uuid.UUID) error
}

type OrganizationUseCase struct {
	orgRepo       OrganizationRepo
	logger        *log.Helper
	ociUseCase    *OCIRepositoryUseCase
	integrationUC *IntegrationUseCase
}

func NewOrganizationUseCase(repo OrganizationRepo, repoUC *OCIRepositoryUseCase, iUC *IntegrationUseCase, logger log.Logger) *OrganizationUseCase {
	return &OrganizationUseCase{orgRepo: repo,
		logger:        log.NewHelper(logger),
		ociUseCase:    repoUC,
		integrationUC: iUC,
	}
}

func (uc *OrganizationUseCase) Create(ctx context.Context, name string) (*Organization, error) {
	// Create a random name if none is provided
	if name == "" {
		name = namesgenerator.GetRandomName(0)
	}

	return uc.orgRepo.Create(ctx, name)
}

func (uc *OrganizationUseCase) FindByID(ctx context.Context, id string) (*Organization, error) {
	orgUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.orgRepo.FindByID(ctx, orgUUID)
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

	// Delete the associated repository
	// Currently there is only one repository per organization
	ociRepository, err := uc.ociUseCase.FindMainRepo(ctx, org.ID)
	if err != nil {
		return err
	}

	if ociRepository != nil {
		// We make sure to call the OCI repository use case to delete the repository
		// including the external secret
		if err := uc.ociUseCase.Delete(ctx, ociRepository.ID); err != nil {
			return err
		}
	}

	// Delete the organization
	return uc.orgRepo.Delete(ctx, orgUUID)
}
