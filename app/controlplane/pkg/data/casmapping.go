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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
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

	// Access is granted through org membership, independent of the workflow's public visibility.
	return entCASMappingToBiz(m, false)
}

// FindPublicByDigest returns a single CAS mapping for the digest that was produced by a public
// workflow, preferring the default backend. It returns (nil, nil) when no public mapping exists.
//
// A public mapping can live in any organization, so visibility is matched on the mapping's workflow
// rather than on org membership. As there is no ent edge from a mapping to its workflow run, the
// match is expressed as a subquery on workflow_run_id. The selection is bounded with a LIMIT 1.
func (r *CASMappingRepo) FindPublicByDigest(ctx context.Context, digest string) (*biz.CASMapping, error) {
	ctx, span := otelx.Start(ctx, casMappingRepoTracer, "CASMappingRepo.FindPublicByDigest")
	defer span.End()

	publicWorkflowRun := func(s *sql.Selector) {
		wr := sql.Table(workflowrun.Table)
		wf := sql.Table(workflow.Table)
		s.Where(sql.In(
			s.C(casmapping.FieldWorkflowRunID),
			sql.Select(wr.C(workflowrun.FieldID)).
				From(wr).
				Join(wf).On(wr.C(workflowrun.FieldWorkflowID), wf.C(workflow.FieldID)).
				// The workflow must be public and not (soft) deleted.
				Where(sql.And(
					sql.EQ(wf.C(workflow.FieldPublic), true),
					sql.IsNull(wf.C(workflow.FieldDeletedAt)),
				)),
		))
	}

	m, err := r.findOnePreferringDefault(ctx, casmapping.Digest(digest), publicWorkflowRun)
	if err != nil || m == nil {
		return nil, err
	}

	return entCASMappingToBiz(m, true)
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

	public, err := r.IsPublic(ctx, r.data.DB, backend.WorkflowRunID)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("cas mapping")
		}

		return nil, fmt.Errorf("failed to check if workflow is public: %w", err)
	}

	return entCASMappingToBiz(backend, public)
}

func (r *CASMappingRepo) IsPublic(ctx context.Context, client *ent.Client, runID uuid.UUID) (bool, error) {
	ctx, span := otelx.Start(ctx, casMappingRepoTracer, "CASMappingRepo.IsPublic")
	defer span.End()

	// If the workflow run id is not set, the mapping is not public
	if runID == uuid.Nil {
		return false, nil
	}

	// Check if the workflow is public
	wr, err := client.WorkflowRun.Query().Where(workflowrun.ID(runID)).Select(workflowrun.FieldWorkflowID).First(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get workflow run: %w", err)
	}

	workflow, err := client.Workflow.Query().Where(workflow.ID(wr.WorkflowID)).Select(workflow.FieldPublic).First(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get workflow: %w", err)
	}

	return workflow.Public, nil
}

func entCASMappingToBiz(input *ent.CASMapping, public bool) (*biz.CASMapping, error) {
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
		Public:        public,
		ProjectID:     input.ProjectID,
	}, nil
}
