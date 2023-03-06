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

package data

import (
	"context"
	"time"

	"github.com/chainloop-dev/bedrock/app/controlplane/internal/biz"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/data/ent/ocirepository"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OCIRepositoryRepo struct {
	data *Data
	log  *log.Helper
}

func NewOCIRepositoryRepo(data *Data, logger log.Logger) biz.OCIRepositoryRepo {
	return &OCIRepositoryRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *OCIRepositoryRepo) FindMainRepo(ctx context.Context, orgID uuid.UUID) (*biz.OCIRepository, error) {
	repo, err := orgScopedQuery(r.data.db, orgID).
		QueryOciRepositories().Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entOCIRepoToBiz(repo), nil
}

func (r *OCIRepositoryRepo) Create(ctx context.Context, opts *biz.OCIRepoCreateOpts) (*biz.OCIRepository, error) {
	repo, err := r.data.db.OCIRepository.Create().
		SetOrganizationID(opts.OrgID).
		SetRepo(opts.Repository).
		SetSecretName(opts.SecretName).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return entOCIRepoToBiz(repo), nil
}

func (r *OCIRepositoryRepo) Update(ctx context.Context, opts *biz.OCIRepoUpdateOpts) (*biz.OCIRepository, error) {
	repo, err := r.data.db.OCIRepository.UpdateOneID(opts.ID).
		SetRepo(opts.Repository).
		SetSecretName(opts.SecretName).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return entOCIRepoToBiz(repo), nil
}

// FindByID finds an OCI repository by ID in the given organization.
// If not found, returns nil and no error
func (r *OCIRepositoryRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.OCIRepository, error) {
	repo, err := r.data.db.OCIRepository.Query().WithOrganization().Where(ocirepository.ID(id)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if repo == nil {
		return nil, nil
	}

	return entOCIRepoToBiz(repo), nil
}

func (r *OCIRepositoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.data.db.OCIRepository.DeleteOneID(id).Exec(ctx)
}

// UpdateValidationStatus updates the validation status of an OCI repository
func (r *OCIRepositoryRepo) UpdateValidationStatus(ctx context.Context, id uuid.UUID, status biz.OCIRepoValidationStatus) error {
	return r.data.db.OCIRepository.UpdateOneID(id).
		SetValidationStatus(status).
		SetValidatedAt(time.Now()).
		Exec(ctx)
}

func entOCIRepoToBiz(repo *ent.OCIRepository) *biz.OCIRepository {
	if repo == nil {
		return nil
	}

	r := &biz.OCIRepository{
		ID:               repo.ID.String(),
		Repo:             repo.Repo,
		SecretName:       repo.SecretName,
		CreatedAt:        toTimePtr(repo.CreatedAt),
		ValidatedAt:      toTimePtr(repo.ValidatedAt),
		ValidationStatus: repo.ValidationStatus,
	}

	if org := repo.Edges.Organization; org != nil {
		r.OrganizationID = org.ID.String()
	}

	return r
}
