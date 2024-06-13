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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OrganizationRepo struct {
	data *Data
	log  *log.Helper
}

func NewOrganizationRepo(data *Data, logger log.Logger) biz.OrganizationRepo {
	return &OrganizationRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *OrganizationRepo) Create(ctx context.Context, name string) (*biz.Organization, error) {
	org, err := r.data.DB.Organization.Create().
		SetName(name).
		Save(ctx)
	if err != nil && ent.IsConstraintError(err) {
		return nil, biz.ErrAlreadyExists
	} else if err != nil {
		return nil, err
	}

	// Reloading the organization to get the proper created_at field from the DB
	// Otherwise there is a mismatch between the created_at field in the DB (millisecods) and the one in the struct (nanoseconds)
	return r.FindByID(ctx, org.ID)
}

func (r *OrganizationRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.Organization, error) {
	org, err := r.data.DB.Organization.Get(ctx, id)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if org == nil {
		return nil, nil
	}

	return entOrgToBizOrg(org), nil
}

// FindByName finds an organization by name.
func (r *OrganizationRepo) FindByName(ctx context.Context, name string) (*biz.Organization, error) {
	org, err := r.data.DB.Organization.Query().Where(organization.NameEQ(name)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if org == nil {
		return nil, nil
	}

	return entOrgToBizOrg(org), nil
}

func (r *OrganizationRepo) Update(ctx context.Context, id uuid.UUID, name *string) (*biz.Organization, error) {
	req := r.data.DB.Organization.UpdateOneID(id)
	if name != nil && *name != "" {
		req = req.SetName(*name)
	}

	org, err := req.Save(ctx)
	if err != nil && ent.IsConstraintError(err) {
		return nil, biz.ErrAlreadyExists
	} else if err != nil {
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	// Reload the object to include the relations
	return r.FindByID(ctx, org.ID)
}

// Delete deletes an organization by ID.
func (r *OrganizationRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.data.DB.Organization.DeleteOneID(id).Exec(ctx)
}

func entOrgToBizOrg(eu *ent.Organization) *biz.Organization {
	return &biz.Organization{Name: eu.Name, ID: eu.ID.String(), CreatedAt: toTimePtr(eu.CreatedAt)}
}
