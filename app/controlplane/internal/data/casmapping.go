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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/casmapping"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/workflow"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

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

func (r *CASMappingRepo) Create(ctx context.Context, digest string, casBackendID, workflowRunID uuid.UUID) (*biz.CASMapping, error) {
	casBackend, err := r.casBackendrepo.FindByID(ctx, casBackendID)
	if err != nil {
		return nil, fmt.Errorf("failed to find cas backend: %w", err)
	} else if casBackend == nil {
		return nil, fmt.Errorf("cas backend not found")
	}

	mapping, err := r.data.db.CASMapping.Create().
		SetDigest(digest).
		SetCasBackendID(casBackendID).
		SetWorkflowRunID(workflowRunID).
		SetOrganizationID(casBackend.OrganizationID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create casMapping: %w", err)
	}

	// reload to get the edges
	return r.findByID(ctx, mapping.ID)
}

func (r *CASMappingRepo) FindByDigest(ctx context.Context, digest string) ([]*biz.CASMapping, error) {
	mappings, err := r.data.db.CASMapping.Query().
		Where(casmapping.Digest(digest)).
		WithCasBackend().
		WithOrganization().
		WithWorkflowRun().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list cas mappings: %w", err)
	}

	res := make([]*biz.CASMapping, 0, len(mappings))
	for _, m := range mappings {
		r, err := entCASMappingToBiz(m)
		if err != nil {
			return nil, fmt.Errorf("failed to convert cas mapping: %w", err)
		}

		res = append(res, r)
	}

	return res, nil
}

// FindByID finds a CAS Mapping by ID
// If not found, returns nil and no error
func (r *CASMappingRepo) findByID(ctx context.Context, id uuid.UUID) (*biz.CASMapping, error) {
	backend, err := r.data.db.CASMapping.Query().WithCasBackend().WithWorkflowRun().WithOrganization().
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

	workflowRun, err := input.Edges.WorkflowRunOrErr()
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow run: %w", err)
	}

	org, err := input.Edges.OrganizationOrErr()
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	// calculate public flag by querying the workflow
	// this query is not efficient since it's done for each mapping
	// but we know that the number of mappings per workflow is small
	workflow, err := workflowRun.QueryWorkflow().Select(workflow.FieldPublic).Only(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	} else if workflow == nil {
		return nil, fmt.Errorf("workflow not found")
	}

	return &biz.CASMapping{
		ID:            input.ID,
		Digest:        input.Digest,
		CASBackend:    entCASBackendToBiz(casBackend),
		WorkflowRunID: workflowRun.ID,
		OrgID:         org.ID,
		CreatedAt:     toTimePtr(input.CreatedAt),
		Public:        workflow.Public,
	}, nil
}
