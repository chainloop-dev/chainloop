//
// Copyright 2024-2026 The Chainloop Authors.
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

	"entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/casbackend"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/casmapping"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/predicate"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowrun"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

var casMappingRepoTracer = otelx.Tracer("chainloop-controlplane", "data/casmapping")

type CASMappingRepo struct {
	data           *Data
	log            *log.Helper
	casBackendrepo biz.CASBackendRepo
}

func NewCASMappingRepo(data *Data, cbRepo biz.CASBackendRepo, logger log.Logger) biz.CASMappingRepo {
	return &CASMappingRepo{
		data:           data,
		log:            log.NewHelper(logger),
		casBackendrepo: cbRepo,
	}
}

func (r *CASMappingRepo) Create(ctx context.Context, digest string, casBackendID uuid.UUID, opts *biz.CASMappingCreateOpts) (*biz.CASMapping, error) {
	ctx, span := otelx.Start(ctx, casMappingRepoTracer, "CASMappingRepo.Create")
	defer span.End()

	casBackend, err := r.casBackendrepo.FindByID(ctx, casBackendID)
	if err != nil {
		return nil, fmt.Errorf("failed to find cas backend: %w", err)
	} else if casBackend == nil {
		return nil, fmt.Errorf("cas backend not found")
	}

	// workflow_run_id has no DB-level foreign key, so validate the referenced run exists to avoid
	// creating a mapping that points to a non-existent workflow run.
	if opts != nil && opts.WorkflowRunID != nil {
		exists, err := r.data.DB.WorkflowRun.Query().Where(workflowrun.ID(*opts.WorkflowRunID)).Exist(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to check workflow run: %w", err)
		} else if !exists {
			return nil, biz.NewErrNotFound("workflow run")
		}
	}

	query := r.data.DB.CASMapping.Create().
		SetDigest(digest).
		SetCasBackendID(casBackendID).
		SetOrganizationID(casBackend.OrganizationID)

	if opts != nil {
		query.SetNillableProjectID(opts.ProjectID).SetNillableWorkflowRunID(opts.WorkflowRunID)
	}

	mapping, err := query.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create casMapping: %w", err)
	}

	// reload to get the edges
	return r.findByID(ctx, mapping.ID)
}

// FindByDigestInOrgs returns a single CAS mapping for the digest that is reachable through one of
// the given organizations, honouring project-level RBAC when projectIDs is provided for an org. The
// mapping stored in the default backend is preferred; ties break on the oldest mapping for a stable
// result. It returns (nil, nil) when no accessible mapping exists.
//
// The selection is performed entirely in the database with a LIMIT 1, so the cost is independent of
// how many mappings a digest accumulates (e.g. the same artifact pushed across thousands of runs).
func (r *CASMappingRepo) FindByDigestInOrgs(ctx context.Context, digest string, orgs []uuid.UUID, projectIDs map[uuid.UUID][]uuid.UUID) (*biz.CASMapping, error) {
	ctx, span := otelx.Start(ctx, casMappingRepoTracer, "CASMappingRepo.FindByDigestInOrgs")
	defer span.End()

	if len(orgs) == 0 {
		return nil, nil
	}

	// Build an OR of per-org predicates. When an org has RBAC enabled (its key is present in
	// projectIDs) the mapping's project must be one of the visible projects; otherwise the whole org
	// is accessible.
	orgPreds := make([]predicate.CASMapping, 0, len(orgs))
	for _, o := range orgs {
		if visibleProjects, ok := projectIDs[o]; ok {
			orgPreds = append(orgPreds, casmapping.And(
				casmapping.OrganizationID(o),
				casmapping.ProjectIDIn(visibleProjects...),
			))
		} else {
			orgPreds = append(orgPreds, casmapping.OrganizationID(o))
		}
	}

	m, err := r.findOnePreferringDefault(ctx, casmapping.Digest(digest), casmapping.Or(orgPreds...))
	if err != nil || m == nil {
		return nil, err
	}

	return entCASMappingToBiz(m)
}

// findOnePreferringDefault returns the first CAS mapping matching the given predicates, preferring
// the one stored in the default backend and breaking ties on the oldest mapping. It returns
// (nil, nil) when nothing matches.
func (r *CASMappingRepo) findOnePreferringDefault(ctx context.Context, preds ...predicate.CASMapping) (*ent.CASMapping, error) {
	ctx, span := otelx.Start(ctx, casMappingRepoTracer, "CASMappingRepo.findOnePreferringDefault")
	defer span.End()

	m, err := r.data.DB.CASMapping.Query().
		Where(preds...).
		// Never return a mapping whose backend has been (soft) deleted; it cannot serve downloads.
		Where(casmapping.HasCasBackendWith(casbackend.DeletedAtIsNil())).
		Order(
			casmapping.ByCasBackendField(casbackend.FieldDefault, sql.OrderDesc()),
			casmapping.ByCreatedAt(sql.OrderAsc()),
		).
		WithCasBackend().
		First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find cas mapping: %w", err)
	}

	return m, nil
}

// FindByID finds a CAS Mapping by ID
// If not found, returns nil and no error
func (r *CASMappingRepo) findByID(ctx context.Context, id uuid.UUID) (*biz.CASMapping, error) {
	backend, err := r.data.DB.CASMapping.Query().WithCasBackend().
		Where(casmapping.ID(id)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if backend == nil {
		return nil, nil
	}

	return entCASMappingToBiz(backend)
}

func entCASMappingToBiz(input *ent.CASMapping) (*biz.CASMapping, error) {
	if input == nil {
		return nil, nil
	}

	// Make sure that the casBackend and the WorkflowRun edges are loaded
	casBackend, err := input.Edges.CasBackendOrErr()
	if err != nil {
		return nil, fmt.Errorf("failed to get cas backend: %w", err)
	}

	return &biz.CASMapping{
		ID:            input.ID,
		Digest:        input.Digest,
		CASBackend:    entCASBackendToBiz(casBackend),
		WorkflowRunID: input.WorkflowRunID,
		OrgID:         input.OrganizationID,
		CreatedAt:     toTimePtr(input.CreatedAt),
		ProjectID:     input.ProjectID,
	}, nil
}
