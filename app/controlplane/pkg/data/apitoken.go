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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type APITokenRepo struct {
	data *Data
	log  *log.Helper
}

func NewAPITokenRepo(data *Data, logger log.Logger) biz.APITokenRepo {
	return &APITokenRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// Persist the APIToken to the database.
func (r *APITokenRepo) Create(ctx context.Context, name string, description *string, expiresAt *time.Time, organizationID uuid.UUID) (*biz.APIToken, error) {
	token, err := r.data.DB.APIToken.Create().
		SetName(name).
		SetNillableDescription(description).
		SetNillableExpiresAt(expiresAt).
		SetOrganizationID(organizationID).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.NewErrAlreadyExists(err)
		}

		return nil, fmt.Errorf("saving APIToken: %w", err)
	}

	return entAPITokenToBiz(token), nil
}

func (r *APITokenRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.APIToken, error) {
	token, err := r.data.DB.APIToken.Query().Where(apitoken.ID(id)).WithOrganization().Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("getting APIToken: %w", err)
	} else if token == nil {
		return nil, nil
	}

	return entAPITokenToBiz(token), nil
}

func (r *APITokenRepo) FindByNameInOrg(ctx context.Context, orgID uuid.UUID, name string) (*biz.APIToken, error) {
	token, err := r.data.DB.APIToken.Query().Where(apitoken.NameEQ(name), apitoken.HasOrganizationWith(organization.ID(orgID)), apitoken.RevokedAtIsNil()).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("API token")
		}

		return nil, err
	}

	return entAPITokenToBiz(token), nil
}

func (r *APITokenRepo) List(ctx context.Context, orgID *uuid.UUID, includeRevoked bool) ([]*biz.APIToken, error) {
	query := r.data.DB.APIToken.Query()

	if orgID != nil {
		query = query.Where(apitoken.OrganizationIDEQ(*orgID))
	}

	if !includeRevoked {
		query = query.Where(apitoken.RevokedAtIsNil())
	}

	tokens, err := query.Order(ent.Asc(apitoken.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*biz.APIToken, 0, len(tokens))
	for _, t := range tokens {
		result = append(result, entAPITokenToBiz(t))
	}

	return result, nil
}

func (r *APITokenRepo) Revoke(ctx context.Context, orgID, id uuid.UUID) error {
	// Update a token with id = id that has not been revoked yet and its orgID = orgID
	err := r.data.DB.APIToken.UpdateOneID(id).
		Where(apitoken.OrganizationIDEQ(orgID), apitoken.RevokedAtIsNil()).
		SetRevokedAt(time.Now()).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return biz.NewErrNotFound("API token")
		}

		return fmt.Errorf("revoking APIToken: %w", err)
	}

	return nil
}

func entAPITokenToBiz(t *ent.APIToken) *biz.APIToken {
	result := &biz.APIToken{
		ID:             t.ID,
		Name:           t.Name,
		Description:    t.Description,
		CreatedAt:      toTimePtr(t.CreatedAt),
		ExpiresAt:      toTimePtr(t.ExpiresAt),
		RevokedAt:      toTimePtr(t.RevokedAt),
		OrganizationID: t.OrganizationID,
	}

	// Add organization name if present
	if t.Edges.Organization != nil {
		result.OrganizationName = t.Edges.Organization.Name
	}

	return result
}
