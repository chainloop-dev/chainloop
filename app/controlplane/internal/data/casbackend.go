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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/casbackend"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type CASBackendRepo struct {
	data *Data
	log  *log.Helper
}

func NewCASBackendRepo(data *Data, logger log.Logger) biz.CASBackendRepo {
	return &CASBackendRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *CASBackendRepo) FindMainBackend(ctx context.Context, orgID uuid.UUID) (*biz.CASBackend, error) {
	backend, err := orgScopedQuery(r.data.db, orgID).
		QueryCasBackends().Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entCASBackendToBiz(backend), nil
}

func (r *CASBackendRepo) Create(ctx context.Context, opts *biz.OCIRepoCreateOpts) (*biz.CASBackend, error) {
	backend, err := r.data.db.CASBackend.Create().
		SetOrganizationID(opts.OrgID).
		SetName(opts.Repository).
		SetSecretName(opts.SecretName).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return entCASBackendToBiz(backend), nil
}

func (r *CASBackendRepo) Update(ctx context.Context, opts *biz.OCIRepoUpdateOpts) (*biz.CASBackend, error) {
	backend, err := r.data.db.CASBackend.UpdateOneID(opts.ID).
		SetName(opts.Repository).
		SetSecretName(opts.SecretName).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return entCASBackendToBiz(backend), nil
}

// FindByID finds a CAS backend by ID in the given organization.
// If not found, returns nil and no error
func (r *CASBackendRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.CASBackend, error) {
	backend, err := r.data.db.CASBackend.Query().WithOrganization().Where(casbackend.ID(id)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if backend == nil {
		return nil, nil
	}

	return entCASBackendToBiz(backend), nil
}

func (r *CASBackendRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.data.db.CASBackend.DeleteOneID(id).Exec(ctx)
}

// UpdateValidationStatus updates the validation status of an OCI repository
func (r *CASBackendRepo) UpdateValidationStatus(ctx context.Context, id uuid.UUID, status biz.CASBackendValidationStatus) error {
	return r.data.db.CASBackend.UpdateOneID(id).
		SetValidationStatus(status).
		SetValidatedAt(time.Now()).
		Exec(ctx)
}

func entCASBackendToBiz(backend *ent.CASBackend) *biz.CASBackend {
	if backend == nil {
		return nil
	}

	r := &biz.CASBackend{
		ID:               backend.ID.String(),
		Name:             backend.Name,
		SecretName:       backend.SecretName,
		CreatedAt:        toTimePtr(backend.CreatedAt),
		ValidatedAt:      toTimePtr(backend.ValidatedAt),
		ValidationStatus: backend.ValidationStatus,
		Provider:         biz.CASBackendProvider(backend.Provider),
	}

	if org := backend.Edges.Organization; org != nil {
		r.OrganizationID = org.ID.String()
	}

	return r
}
