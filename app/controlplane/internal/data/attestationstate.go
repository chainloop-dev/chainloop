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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/workflowrun"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type AttestationStateRepo struct {
	data *Data
	log  *log.Helper
}

func NewAttestationStateRepo(data *Data, logger log.Logger) biz.AttestationStateRepo {
	return &AttestationStateRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// find the workflow run by its ID and check that it has attestation state
func (r *AttestationStateRepo) Initialized(ctx context.Context, runID uuid.UUID) (bool, error) {
	exists, err := r.data.db.WorkflowRun.Query().Where(workflowrun.ID(runID)).Where(workflowrun.AttestationStateNotNil()).Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check attestation state: %w", err)
	}

	return exists, nil
}

func (r *AttestationStateRepo) Save(ctx context.Context, runID uuid.UUID, state []byte) error {
	err := r.data.db.WorkflowRun.UpdateOneID(runID).SetAttestationState(state).Exec(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("failed to store attestation state: %w", err)
	} else if err != nil {
		return biz.NewErrNotFound("workflow run")
	}

	return nil
}

func (r *AttestationStateRepo) Read(ctx context.Context, runID uuid.UUID) ([]byte, error) {
	run, err := r.data.db.WorkflowRun.Query().Where(workflowrun.ID(runID)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to read attestation state: %w", err)
	} else if run == nil || run.AttestationState == nil {
		return nil, biz.NewErrNotFound("attestation state")
	}

	return run.AttestationState, nil
}

func (r *AttestationStateRepo) Reset(ctx context.Context, runID uuid.UUID) error {
	err := r.data.db.WorkflowRun.UpdateOneID(runID).ClearAttestationState().Exec(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return fmt.Errorf("failed to clear attestation state: %w", err)
	} else if err != nil {
		return biz.NewErrNotFound("attestation state")
	}

	return nil
}
