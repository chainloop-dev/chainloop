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
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type CASMappingRepo struct {
	data *Data
	log  *log.Helper
}

func NewCASMappingRepo(data *Data, logger log.Logger) biz.CASMappingRepo {
	return &CASMappingRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *CASMappingRepo) Create(ctx context.Context, digest string, casBackendID, workflowRunID uuid.UUID) (*biz.CASMapping, error) {
	mapping, err := r.data.db.CASMapping.Create().
		SetDigest(digest).
		SetCasBackendID(casBackendID).
		SetWorkflowRunID(workflowRunID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create casMapping: %w", err)
	}

	// reload to get the edges
	return r.findByID(ctx, mapping.ID)
}

// FindByID finds a CAS Mapping by ID
// If not found, returns nil and no error
func (r *CASMappingRepo) findByID(ctx context.Context, id uuid.UUID) (*biz.CASMapping, error) {
	backend, err := r.data.db.CASMapping.Query().WithCasBackend().WithWorkflowRun().
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

	return &biz.CASMapping{
		ID:            input.ID,
		Digest:        input.Digest,
		CASBackendID:  casBackend.ID,
		WorkflowRunID: workflowRun.ID,
		CreatedAt:     toTimePtr(input.CreatedAt),
	}, nil
}
