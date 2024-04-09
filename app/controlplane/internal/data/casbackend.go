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
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/casbackend"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/organization"
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

func (r *CASBackendRepo) List(ctx context.Context, orgID uuid.UUID) ([]*biz.CASBackend, error) {
	backends, err := orgScopedQuery(r.data.db, orgID).QueryCasBackends().WithOrganization().
		Where(casbackend.DeletedAtIsNil()).
		Order(ent.Desc(casbackend.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list cas backends: %w", err)
	}

	res := make([]*biz.CASBackend, 0, len(backends))
	for _, backend := range backends {
		res = append(res, entCASBackendToBiz(backend))
	}

	return res, nil
}

// FindDefaultBackend finds the CAS backend that's set as default for the given organization
func (r *CASBackendRepo) FindDefaultBackend(ctx context.Context, orgID uuid.UUID) (*biz.CASBackend, error) {
	backend, err := orgScopedQuery(r.data.db, orgID).QueryCasBackends().WithOrganization().
		Where(casbackend.Default(true), casbackend.DeletedAtIsNil()).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entCASBackendToBiz(backend), nil
}

// FindFallbackBackend finds the CAS backend that's set as fallback for the given organization
func (r *CASBackendRepo) FindFallbackBackend(ctx context.Context, orgID uuid.UUID) (*biz.CASBackend, error) {
	backend, err := orgScopedQuery(r.data.db, orgID).QueryCasBackends().WithOrganization().
		Where(casbackend.Fallback(true), casbackend.DeletedAtIsNil()).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	}

	return entCASBackendToBiz(backend), nil
}

// Create creates a new CAS backend in the given organization
// If it's set as default, it will unset the previous default backend
func (r *CASBackendRepo) Create(ctx context.Context, opts *biz.CASBackendCreateOpts) (*biz.CASBackend, error) {
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// 1 - unset default backend for all the other backends in the org
	if opts.Default {
		if err := tx.CASBackend.Update().
			Where(casbackend.HasOrganizationWith(organization.ID(opts.OrgID))).
			Where(casbackend.Default(true)).
			SetDefault(false).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("failed to clear previous default backend: %w", err)
		}
	}

	// 2 - create the new backend and set it as default if needed
	backend, err := tx.CASBackend.Create().
		SetName(opts.Name).
		SetOrganizationID(opts.OrgID).
		SetLocation(opts.Location).
		SetDescription(opts.Description).
		SetFallback(opts.Fallback).
		SetProvider(opts.Provider).
		SetDefault(opts.Default).
		SetSecretName(opts.SecretName).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.ErrAlreadyExists
		}

		return nil, fmt.Errorf("failed to create backend: %w", err)
	}

	// 3 - commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Return the backend from the DB to have consistent marshalled object
	return r.FindByID(ctx, backend.ID)
}

func (r *CASBackendRepo) Update(ctx context.Context, opts *biz.CASBackendUpdateOpts) (*biz.CASBackend, error) {
	tx, err := r.data.db.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// 1 - unset default backend for all the other backends in the org
	if opts.Default {
		if err := tx.CASBackend.Update().
			Where(casbackend.HasOrganizationWith(organization.ID(opts.OrgID))).
			Where(casbackend.Default(true)).
			SetDefault(false).
			Exec(ctx); err != nil {
			return nil, fmt.Errorf("failed to clear previous default backend: %w", err)
		}
	}

	// 2 - Chain the list of updates
	updateChain := tx.CASBackend.UpdateOneID(opts.ID).SetDefault(opts.Default)
	// If description is provided we set it
	if opts.Description != "" {
		updateChain = updateChain.SetDescription(opts.Description)
	}

	// If secretName is provided we set it
	if opts.SecretName != "" {
		updateChain = updateChain.SetSecretName(opts.SecretName)
	}

	backend, err := updateChain.Save(ctx)
	if err != nil {
		return nil, err
	}

	// 3 - commit the transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return r.FindByID(ctx, backend.ID)
}

// FindByID finds a CAS backend by ID
// If not found, returns nil and no error
func (r *CASBackendRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.CASBackend, error) {
	backend, err := r.data.db.CASBackend.Query().WithOrganization().
		Where(casbackend.ID(id), casbackend.DeletedAtIsNil()).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if backend == nil {
		return nil, nil
	}

	return entCASBackendToBiz(backend), nil
}

// FindByIDInOrg finds a CAS backend by ID in the given organization.
// If not found, returns nil and no error
func (r *CASBackendRepo) FindByIDInOrg(ctx context.Context, orgID, id uuid.UUID) (*biz.CASBackend, error) {
	backend, err := orgScopedQuery(r.data.db, orgID).QueryCasBackends().WithOrganization().
		Where(casbackend.ID(id), casbackend.DeletedAtIsNil()).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if backend == nil {
		return nil, nil
	}

	return entCASBackendToBiz(backend), nil
}

// Set deleted at instead of actually deleting the backend
func (r *CASBackendRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return r.data.db.CASBackend.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
}

// Delete deletes a CAS backend from the DB
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

	limits := &biz.CASBackendLimits{
		MaxBytes: biz.CASBackendDefaultMaxBytes,
	}

	inline := backend.Provider == biz.CASBackendInline
	if inline {
		limits.MaxBytes = biz.CASBackendInlineDefaultMaxBytes
	}

	r := &biz.CASBackend{
		ID:               backend.ID,
		Name:             backend.Name,
		Location:         backend.Location,
		Description:      backend.Description,
		SecretName:       backend.SecretName,
		CreatedAt:        toTimePtr(backend.CreatedAt),
		ValidatedAt:      toTimePtr(backend.ValidatedAt),
		ValidationStatus: backend.ValidationStatus,
		Provider:         backend.Provider,
		Default:          backend.Default,
		Inline:           inline,
		Limits:           limits,
		Fallback:         backend.Fallback,
	}

	if org := backend.Edges.Organization; org != nil {
		r.OrganizationID = org.ID
	}

	return r
}
