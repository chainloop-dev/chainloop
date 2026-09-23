//
// Copyright 2023-2026 The Chainloop Authors.
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

	"entgo.io/ent/dialect/sql/sqlgraph"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/apitoken"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

var apiTokenRepoTracer = otelx.Tracer("chainloop-controlplane", "data/apitoken")

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
func (r *APITokenRepo) Create(ctx context.Context, opts *biz.APITokenCreateOpts) (*biz.APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenRepoTracer, "APITokenRepo.Create")
	defer span.End()

	token, err := r.data.DB.APIToken.Create().
		SetName(opts.Name).
		SetNillableDescription(opts.Description).
		SetNillableExpiresAt(opts.ExpiresAt).
		SetNillableOrganizationID(opts.OrganizationID).
		SetNillableProjectID(opts.ProjectID).
		SetNillableWorkflowID(opts.WorkflowID).
		SetNillableScope(opts.Scope).
		SetNillableScopeID(opts.ScopeID).
		SetPolicies(opts.Policies).
		SetIsSystem(opts.IsSystem).
		Save(ctx)
	if err != nil {
		// A CHECK violation is a malformed scope, not a name clash.
		if sqlgraph.IsCheckConstraintError(err) {
			return nil, biz.NewErrValidation(err)
		}

		if ent.IsConstraintError(err) {
			return nil, biz.NewErrAlreadyExists(err)
		}

		return nil, fmt.Errorf("saving APIToken: %w", err)
	}

	return r.FindByID(ctx, token.ID)
}

func (r *APITokenRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenRepoTracer, "APITokenRepo.FindByID")
	defer span.End()

	token, err := r.data.DB.APIToken.Query().Where(apitoken.ID(id)).WithOrganization().WithProject().WithWorkflow().Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("getting APIToken: %w", err)
	} else if token == nil {
		return nil, nil
	}

	return entAPITokenToBiz(token), nil
}

func (r *APITokenRepo) FindByIDInOrg(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*biz.APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenRepoTracer, "APITokenRepo.FindByIDInOrg")
	defer span.End()

	token, err := r.data.DB.APIToken.Query().Where(apitoken.ID(id), apitoken.HasOrganizationWith(organization.ID(orgID)), apitoken.RevokedAtIsNil()).WithProject().WithWorkflow().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("API token")
		}

		return nil, err
	}

	return entAPITokenToBiz(token), nil
}

func (r *APITokenRepo) FindByNameInOrg(ctx context.Context, orgID uuid.UUID, name string) (*biz.APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenRepoTracer, "APITokenRepo.FindByNameInOrg")
	defer span.End()

	token, err := r.data.DB.APIToken.Query().Where(apitoken.Name(name), apitoken.HasOrganizationWith(organization.ID(orgID)), apitoken.RevokedAtIsNil()).WithProject().WithWorkflow().Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("API token")
		}

		// A name is unique per project and per scoped resource, never per organization, so
		// this lookup can legitimately match more than one token. Report that rather than
		// letting ent's NotSingularError be masked into an internal error.
		if ent.IsNotSingular(err) {
			return nil, biz.NewErrValidationStr(fmt.Sprintf("more than one API token is named %q in this organization", name))
		}

		return nil, err
	}

	return entAPITokenToBiz(token), nil
}

func (r *APITokenRepo) List(ctx context.Context, orgID *uuid.UUID, filters *biz.APITokenListFilters) ([]*biz.APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenRepoTracer, "APITokenRepo.List")
	defer span.End()

	query := r.data.DB.APIToken.Query().WithProject().WithWorkflow().WithOrganization()

	if filters == nil {
		filters = &biz.APITokenListFilters{}
	}

	if orgID != nil {
		query = query.Where(apitoken.OrganizationIDEQ(*orgID))
	}

	if len(filters.FilterByProjects) > 0 {
		query = query.Where(apitoken.ProjectIDIn(filters.FilterByProjects...))
	}

	if !filters.IncludeSystem {
		query = query.Where(apitoken.IsSystem(false))
	}

	switch filters.FilterByScope {
	case biz.APITokenScopeProject:
		query = query.Where(apitoken.ProjectIDNotNil())
	case biz.APITokenScopeProduct:
		query = query.Where(apitoken.ScopeEQ(authz.ResourceTypeProduct))
	case biz.APITokenScopeGlobal:
		// Organization-wide means confined to neither a project nor a scoped resource.
		query = query.Where(apitoken.ProjectIDIsNil(), apitoken.ScopeIDIsNil())
	case biz.APITokenScopeInstance:
		query = query.Where(apitoken.OrganizationIDIsNil())
	}

	switch filters.StatusFilter {
	case biz.APITokenStatusFilterRevoked:
		query = query.Where(apitoken.RevokedAtNotNil())
	case biz.APITokenStatusFilterAll:
		// no filter — return all tokens
	default: // APITokenStatusFilterActive
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

func (r *APITokenRepo) Revoke(ctx context.Context, orgID *uuid.UUID, id uuid.UUID) error {
	ctx, span := otelx.Start(ctx, apiTokenRepoTracer, "APITokenRepo.Revoke")
	defer span.End()

	// Update a token with id = id that has not been revoked yet and its orgID = orgID
	update := r.data.DB.APIToken.UpdateOneID(id).
		Where(apitoken.RevokedAtIsNil())

	if orgID != nil {
		update = update.Where(apitoken.OrganizationIDEQ(*orgID))
	} else {
		update = update.Where(apitoken.OrganizationIDIsNil())
	}

	err := update.SetRevokedAt(time.Now()).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return biz.NewErrNotFound("API token")
		}

		return fmt.Errorf("revoking APIToken: %w", err)
	}

	return nil
}

// FindInactive returns tokens that have been inactive since the given cutoff.
func (r *APITokenRepo) FindInactive(ctx context.Context, orgID uuid.UUID, inactiveSince time.Time) ([]*biz.APIToken, error) {
	ctx, span := otelx.Start(ctx, apiTokenRepoTracer, "APITokenRepo.FindInactive")
	defer span.End()

	// A token is inactive if last_used_at < inactiveSince (when used before),
	// or created_at < inactiveSince (when never used).
	tokens, err := r.data.DB.APIToken.Query().
		Where(
			apitoken.OrganizationIDEQ(orgID),
			apitoken.RevokedAtIsNil(),
			apitoken.Or(
				apitoken.And(apitoken.LastUsedAtNotNil(), apitoken.LastUsedAtLT(inactiveSince)),
				apitoken.And(apitoken.LastUsedAtIsNil(), apitoken.CreatedAtLT(inactiveSince)),
			),
		).
		WithOrganization().
		WithProject().
		WithWorkflow().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying inactive tokens: %w", err)
	}

	result := make([]*biz.APIToken, 0, len(tokens))
	for _, t := range tokens {
		result = append(result, entAPITokenToBiz(t))
	}

	return result, nil
}

func (r *APITokenRepo) UpdateExpiration(ctx context.Context, id uuid.UUID, expiresAt time.Time) error {
	ctx, span := otelx.Start(ctx, apiTokenRepoTracer, "APITokenRepo.UpdateExpiration")
	defer span.End()

	err := r.data.DB.APIToken.UpdateOneID(id).SetExpiresAt(expiresAt).Exec(ctx)
	if err != nil {
		return fmt.Errorf("updating APIToken: %w", err)
	}

	return nil
}

func (r *APITokenRepo) UpdateLastUsedAt(ctx context.Context, id uuid.UUID, lastUsedAt time.Time) error {
	ctx, span := otelx.Start(ctx, apiTokenRepoTracer, "APITokenRepo.UpdateLastUsedAt")
	defer span.End()

	err := r.data.DB.APIToken.UpdateOneID(id).Where(apitoken.RevokedAtIsNil()).SetLastUsedAt(lastUsedAt).Exec(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return biz.NewErrNotFound("API token")
		}
		return fmt.Errorf("updating API Token: %w", err)
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
		LastUsedAt:     toTimePtr(t.LastUsedAt),
		OrganizationID: t.OrganizationID,
		Policies:       t.Policies,
		IsSystem:       t.IsSystem,
	}

	// Add organization name if present
	if t.Edges.Organization != nil {
		result.OrganizationName = t.Edges.Organization.Name
	}

	if p := t.Edges.Project; p != nil {
		result.ProjectID = biz.ToPtr(p.ID)
		result.ProjectName = biz.ToPtr(p.Name)
	}

	if w := t.Edges.Workflow; w != nil {
		result.WorkflowID = biz.ToPtr(w.ID)
		result.WorkflowName = biz.ToPtr(w.Name)
	}

	// The scoped resource is not an entity in this database, so unlike the project and the
	// workflow it has no edge to load: both values come straight off the row.
	result.Scope = t.Scope
	result.ScopeID = t.ScopeID

	return result
}
