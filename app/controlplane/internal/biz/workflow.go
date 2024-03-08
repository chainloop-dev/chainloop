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

package biz

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type Workflow struct {
	Name, Description, Team, Project string
	CreatedAt                        *time.Time
	RunsCounter                      int
	LastRun                          *WorkflowRun
	ID, ContractID, OrgID            uuid.UUID
	// Latest available contract revision
	ContractRevisionLatest int
	// Public means that the associated workflow runs, attestations and materials
	// are reachable by other users, regardless of their organization
	// This field is also used to calculate if an user can download attestations/materials from the CAS
	Public bool
}

type WorkflowRepo interface {
	Create(ctx context.Context, opts *WorkflowCreateOpts) (*Workflow, error)
	Update(ctx context.Context, id uuid.UUID, opts *WorkflowUpdateOpts) (*Workflow, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*Workflow, error)
	GetOrgScoped(ctx context.Context, orgID, workflowID uuid.UUID) (*Workflow, error)
	IncRunsCounter(ctx context.Context, workflowID uuid.UUID) error
	FindByID(ctx context.Context, workflowID uuid.UUID) (*Workflow, error)
	SoftDelete(ctx context.Context, workflowID uuid.UUID) error
}

// TODO: move to pointer properties to handle empty values
type WorkflowCreateOpts struct {
	Name, OrgID, Project, Team, ContractID, Description string
	// Public means that the associated workflow runs, attestations and materials
	// are reachable by other users, regardless of their organization
	Public bool
}

type WorkflowUpdateOpts struct {
	Name, Project, Team, Description *string
	Public                           *bool
}

type WorkflowUseCase struct {
	wfRepo     WorkflowRepo
	contractUC *WorkflowContractUseCase
	logger     *log.Helper
}

func NewWorkflowUsecase(wfr WorkflowRepo, schemaUC *WorkflowContractUseCase, logger log.Logger) *WorkflowUseCase {
	return &WorkflowUseCase{wfRepo: wfr, contractUC: schemaUC, logger: log.NewHelper(logger)}
}

func (uc *WorkflowUseCase) Create(ctx context.Context, opts *WorkflowCreateOpts) (*Workflow, error) {
	if opts.Name == "" {
		return nil, errors.New("workflow name is required")
	}

	contract, err := uc.findOrCreateContract(ctx, opts.OrgID, opts.ContractID, opts.Project, opts.Name)
	if err != nil {
		return nil, err
	}

	// Set the potential new schemaID
	opts.ContractID = contract.ID.String()
	return uc.wfRepo.Create(ctx, opts)
}

func (uc *WorkflowUseCase) Update(ctx context.Context, orgID, workflowID string, opts *WorkflowUpdateOpts) (*Workflow, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// make sure that the workflow is for the provided org
	if wf, err := uc.wfRepo.GetOrgScoped(ctx, orgUUID, workflowUUID); err != nil {
		return nil, err
	} else if wf == nil {
		return nil, NewErrNotFound("workflow in organization")
	}

	wf, err := uc.wfRepo.Update(ctx, workflowUUID, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to update workflow: %w", err)
	} else if wf == nil {
		return nil, NewErrNotFound("workflow")
	}

	return wf, err
}

func (uc *WorkflowUseCase) findOrCreateContract(ctx context.Context, orgID, contractID, project, name string) (*WorkflowContract, error) {
	// The contractID has been provided so we try to find it in our org
	if contractID != "" {
		return uc.contractUC.FindByIDInOrg(ctx, orgID, contractID)
	}

	// No contractID has been provided so we create a new one
	contractName := fmt.Sprintf("%s/%s", project, name)
	return uc.contractUC.Create(ctx, orgID, contractName, nil)
}

func (uc *WorkflowUseCase) List(ctx context.Context, orgID string) ([]*Workflow, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.wfRepo.List(ctx, orgUUID)
}

func (uc *WorkflowUseCase) IncRunsCounter(ctx context.Context, workflowID string) error {
	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	return uc.wfRepo.IncRunsCounter(ctx, workflowUUID)
}

func (uc *WorkflowUseCase) FindByID(ctx context.Context, workflowID string) (*Workflow, error) {
	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return nil, err
	}

	return uc.wfRepo.FindByID(ctx, workflowUUID)
}

func (uc *WorkflowUseCase) FindByIDInOrg(ctx context.Context, orgID, workflowID string) (*Workflow, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	wf, err := uc.wfRepo.GetOrgScoped(ctx, orgUUID, workflowUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	} else if wf == nil {
		return nil, NewErrNotFound("workflow in organization")
	}

	return wf, nil
}

// Delete soft-deletes the entry
func (uc *WorkflowUseCase) Delete(ctx context.Context, orgID, workflowID string) error {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// Check that the workflow to delete belongs to the provided organization
	if wf, err := uc.wfRepo.GetOrgScoped(ctx, orgUUID, workflowUUID); err != nil {
		return err
	} else if wf == nil {
		return NewErrNotFound("organization")
	}

	return uc.wfRepo.SoftDelete(ctx, workflowUUID)
}
