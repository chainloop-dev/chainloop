//
// Copyright 2024 The Chainloop Authors.
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
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/casmapping"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowrun"
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

	mapping, err := r.data.DB.CASMapping.Create().
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
	mappings, err := r.data.DB.CASMapping.Query().
		Where(casmapping.Digest(digest)).
		WithCasBackend().
		WithOrganization().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list cas mappings: %w", err)
	}

	res := make([]*biz.CASMapping, 0, len(mappings))
	for _, m := range mappings {
		public, err := r.IsPublic(ctx, r.data.DB, m.WorkflowRunID)
		if err != nil {
			if biz.IsNotFound(err) {
				return nil, nil
			}

			return nil, fmt.Errorf("failed to check if workflow is public: %w", err)
		}
		r, err := entCASMappingToBiz(m, public)
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
	backend, err := r.data.DB.CASMapping.Query().WithCasBackend().
		Where(casmapping.ID(id)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if backend == nil {
		return nil, nil
	}

	public, err := r.IsPublic(ctx, r.data.DB, backend.WorkflowRunID)
	if err != nil {
		if biz.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to check if workflow is public: %w", err)
	}

	return entCASMappingToBiz(backend, public)
}

func (r *CASMappingRepo) IsPublic(ctx context.Context, client *ent.Client, runID uuid.UUID) (bool, error) {
	// Check if the workflow is public
	wr, err := client.WorkflowRun.Query().Where(workflowrun.ID(runID)).Select(workflowrun.FieldWorkflowID).First(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get workflow run: %w", err)
	} else if wr == nil {
		return false, nil
	}

	workflow, err := client.Workflow.Query().Where(workflow.ID(wr.WorkflowID)).Select(workflow.FieldPublic).First(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get workflow: %w", err)
	} else if workflow == nil {
		return false, nil
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
	}, nil
}
