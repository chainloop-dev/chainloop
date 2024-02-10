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

package biz

import (
	"context"
	"fmt"

	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

type AttestationState struct {
	State *schemav1.CraftingSchema
}

type AttestationStateRepo interface {
	Initialized(ctx context.Context, workflowRunID uuid.UUID) (bool, error)
	Save(ctx context.Context, workflowRunID uuid.UUID, state []byte) error
	Read(ctx context.Context, workflowRunID uuid.UUID) ([]byte, error)
	Reset(ctx context.Context, workflowRunID uuid.UUID) error
}

type AttestationStateUseCase struct {
	repo      AttestationStateRepo
	wfRunRepo WorkflowRunRepo
}

func NewAttestationStateUseCase(repo AttestationStateRepo, wfRunRepo WorkflowRunRepo) (*AttestationStateUseCase, error) {
	return &AttestationStateUseCase{repo, wfRunRepo}, nil
}

func (uc *AttestationStateUseCase) Initialized(ctx context.Context, workflowID, runID string) (bool, error) {
	run, err := uc.checkWorkflowRunInWorkflow(ctx, workflowID, runID)
	if err != nil {
		return false, fmt.Errorf("failed to check workflow run: %w", err)
	}

	initialized, err := uc.repo.Initialized(ctx, run.ID)
	if err != nil {
		return false, fmt.Errorf("failed to check initialized state: %w", err)
	}

	return initialized, nil
}

func (uc *AttestationStateUseCase) Save(ctx context.Context, workflowID, runID string, state *schemav1.CraftingSchema) error {
	run, err := uc.checkWorkflowRunInWorkflow(ctx, workflowID, runID)
	if err != nil {
		return fmt.Errorf("failed to check workflow run: %w", err)
	}

	rawState, err := proto.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal attestation state: %w", err)
	}

	if err := uc.repo.Save(ctx, run.ID, rawState); err != nil {
		return fmt.Errorf("failed to save attestation state: %w", err)
	}

	return nil
}

func (uc *AttestationStateUseCase) Read(ctx context.Context, workflowID, runID string) (*AttestationState, error) {
	run, err := uc.checkWorkflowRunInWorkflow(ctx, workflowID, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to check workflow run: %w", err)
	}

	res, err := uc.repo.Read(ctx, run.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to save attestation state: %w", err)
	}

	state := &schemav1.CraftingSchema{}
	if err := proto.Unmarshal(res, state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attestation state: %w", err)
	}

	return &AttestationState{State: state}, nil
}

func (uc *AttestationStateUseCase) Reset(ctx context.Context, workflowID, runID string) error {
	run, err := uc.checkWorkflowRunInWorkflow(ctx, workflowID, runID)
	if err != nil {
		return fmt.Errorf("failed to check workflow run: %w", err)
	}

	if err := uc.repo.Reset(ctx, run.ID); err != nil {
		return fmt.Errorf("failed to reset attestation state: %w", err)
	}

	return nil
}

// checkWorkflowRunInWorkflow checks if the workflow run belongs to the provided workflow
// This is important because the workflow is something that comes embedded in the auth token
// so it can be used to make sure the user is not spoofing a different run that doesn't have access to
func (uc *AttestationStateUseCase) checkWorkflowRunInWorkflow(ctx context.Context, workflowID, runID string) (*WorkflowRun, error) {
	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	runUUID, err := uuid.Parse(runID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	run, err := uc.wfRunRepo.FindByID(ctx, runUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to workflow run: %w", err)
	} else if run == nil {
		return nil, NewErrNotFound("workflow run")
	}

	if run.Workflow.ID != workflowUUID {
		return nil, NewErrNotFound("workflow run")
	}

	return run, nil
}
