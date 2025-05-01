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
	"errors"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/jsonfilter"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type Workflow struct {
	Name, Description, Team, Project string
	CreatedAt                        *time.Time
	RunsCounter                      int
	LastRun                          *WorkflowRun
	ID, ContractID, OrgID            uuid.UUID
	ContractName                     string
	// Latest available contract revision
	ContractRevisionLatest int
	// Public means that the associated workflow runs, attestations and materials
	// are reachable by other users, regardless of their organization
	// This field is also used to calculate if an user can download attestations/materials from the CAS
	Public    bool
	ProjectID uuid.UUID
}

// WorkflowRef is a reference to a workflow
// With combination of the name and the project, it should be unique
type WorkflowRef struct {
	ID          uuid.UUID
	Name        string
	ProjectName string
}

type WorkflowRepo interface {
	Create(ctx context.Context, opts *WorkflowCreateOpts) (*Workflow, error)
	Update(ctx context.Context, id uuid.UUID, opts *WorkflowUpdateOpts) (*Workflow, error)
	List(ctx context.Context, orgID uuid.UUID, filter *WorkflowListOpts, pagination *pagination.OffsetPaginationOpts) ([]*Workflow, int, error)
	GetOrgScoped(ctx context.Context, orgID, workflowID uuid.UUID) (*Workflow, error)
	GetOrgScopedByProjectAndName(ctx context.Context, orgID uuid.UUID, projectName, workflowName string) (*Workflow, error)
	IncRunsCounter(ctx context.Context, workflowID uuid.UUID) error
	FindByID(ctx context.Context, workflowID uuid.UUID) (*Workflow, error)
	SoftDelete(ctx context.Context, workflowID uuid.UUID) error
}

// TODO: move to pointer properties to handle empty values
type WorkflowCreateOpts struct {
	Name, OrgID, Project, Team, ContractName, Description string
	ContractID                                            string
	// Public means that the associated workflow runs, attestations and materials
	// are reachable by other users, regardless of their organization
	Public bool
}

type WorkflowUpdateOpts struct {
	Team, Description, ContractID *string
	Public                        *bool
}

// WorkflowListOpts is the options to filter the list of workflows
type WorkflowListOpts struct {
	// WorkflowName is the name of the workflow
	WorkflowName string
	// WorkflowDescription is the description of the workflow
	WorkflowDescription string
	// WorkflowTeam is the team of the workflow
	WorkflowTeam string
	// WorkflowProjectNames is the project name of the workflow
	WorkflowProjectNames []string
	// WorkflowPublic is the flag to filter public workflows
	WorkflowPublic *bool
	// WorkflowActiveWindow is the active window of the workflow
	WorkflowRunRunnerType string
	// WorkflowActiveWindow is the active window of the workflow
	WorkflowActiveWindow *TimeWindow
	// WorkflowRunStatus is the status of the workflow runs to return
	WorkflowRunLastStatus WorkflowRunStatus
	// JSONFilters is the filters to apply to the JSON fields
	JSONFilters []*jsonfilter.JSONFilter
}

type WorkflowUseCase struct {
	wfRepo      WorkflowRepo
	projectRepo ProjectsRepo
	contractUC  *WorkflowContractUseCase
	auditorUC   *AuditorUseCase
	logger      *log.Helper
}

func NewWorkflowUsecase(wfr WorkflowRepo, projectsRepo ProjectsRepo, schemaUC *WorkflowContractUseCase, auditorUC *AuditorUseCase, logger log.Logger) *WorkflowUseCase {
	return &WorkflowUseCase{wfRepo: wfr, contractUC: schemaUC, projectRepo: projectsRepo, auditorUC: auditorUC, logger: log.NewHelper(logger)}
}

func (uc *WorkflowUseCase) Create(ctx context.Context, opts *WorkflowCreateOpts) (*Workflow, error) {
	if opts.Name == "" {
		return nil, errors.New("workflow name is required")
	} else if opts.Project == "" {
		return nil, errors.New("project name is required")
	}

	// validate format of the name and the project
	if err := ValidateIsDNS1123(opts.Name); err != nil {
		return nil, NewErrValidation(err)
	}

	if err := ValidateIsDNS1123(opts.Project); err != nil {
		return nil, NewErrValidation(err)
	}

	// If the name is not provided for the contract we come up with one based on the workflow info
	if opts.ContractName == "" {
		opts.ContractName = fmt.Sprintf("%s-%s", opts.Project, opts.Name)
	}

	contract, err := uc.findOrCreateContract(ctx, opts.OrgID, opts.ContractName)
	if err != nil {
		return nil, err
	} else if contract == nil {
		return nil, NewErrNotFound("contract")
	}

	opts.ContractID = contract.ID.String()
	wf, err := uc.wfRepo.Create(ctx, opts)
	if err != nil {
		if IsErrAlreadyExists(err) {
			return nil, NewErrAlreadyExistsStr(fmt.Sprintf("workflow %q in project %q already exists", opts.Name, opts.Project))
		}

		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	orgUUID, err := uuid.Parse(opts.OrgID)
	if err != nil {
		uc.logger.Warn("failed to parse org id", "err", err)
	} else {
		// Dispatch events to the audit log regarding the workflow
		uc.auditorUC.Dispatch(ctx, &events.WorkflowCreated{
			WorkflowBase: &events.WorkflowBase{
				WorkflowID:   &wf.ID,
				WorkflowName: wf.Name,
				ProjectName:  opts.Project,
			},
			WorkflowContractID:   &contract.ID,
			WorkflowContractName: contract.Name,
			WorkflowDescription:  &opts.Description,
			Team:                 &opts.Team,
			Public:               opts.Public,
		}, &orgUUID)

		// Dispatch events to the audit log regarding the contract
		uc.auditorUC.Dispatch(ctx, &events.WorkflowContractAttached{
			WorkflowContractBase: &events.WorkflowContractBase{
				WorkflowContractID:   &contract.ID,
				WorkflowContractName: contract.Name,
			},
			WorkflowID:   &wf.ID,
			WorkflowName: wf.Name,
		}, &orgUUID)
	}

	return wf, nil
}

func (uc *WorkflowUseCase) Update(ctx context.Context, orgID, workflowID string, opts *WorkflowUpdateOpts) (*Workflow, error) {
	if opts == nil {
		return nil, NewErrValidationStr("no updates provided")
	}

	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	workflowUUID, err := uuid.Parse(workflowID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// make sure that the workflow is for the provided org
	var preUpdateWorkflow *Workflow
	if preUpdateWorkflow, err = uc.wfRepo.GetOrgScoped(ctx, orgUUID, workflowUUID); err != nil {
		return nil, err
	} else if preUpdateWorkflow == nil {
		return nil, NewErrNotFound("workflow in organization")
	}

	// Double check that the contract exists
	var wfContract *WorkflowContract
	if opts.ContractID != nil {
		if wfContract, err = uc.contractUC.FindByIDInOrg(ctx, orgID, *opts.ContractID); err != nil {
			return nil, err
		} else if wfContract == nil {
			return nil, NewErrNotFound("contract")
		}
	}

	wf, err := uc.wfRepo.Update(ctx, workflowUUID, opts)
	if err != nil {
		if IsErrAlreadyExists(err) {
			return nil, NewErrValidationStr("name already taken")
		}

		return nil, fmt.Errorf("failed to update workflow: %w", err)
	} else if wf == nil {
		return nil, NewErrNotFound("workflow")
	}

	// Dispatch events to the audit log regarding the workflow
	uc.handleWorkflowUpdateEvents(ctx, wf, preUpdateWorkflow, opts, wfContract, orgUUID)

	return wf, err
}

// handleWorkflowUpdateEvents dispatches events to the audit log regarding the workflow
func (uc *WorkflowUseCase) handleWorkflowUpdateEvents(ctx context.Context, wf *Workflow, preUpdateWorkflow *Workflow, opts *WorkflowUpdateOpts, newWfContract *WorkflowContract, orgUUID uuid.UUID) {
	uc.dispatchWorkflowUpdatedEvent(ctx, wf, opts, newWfContract, orgUUID)

	if opts.ContractID != nil {
		uc.handleContractChangeEvents(ctx, preUpdateWorkflow, wf, newWfContract, orgUUID)
	}
}

// dispatchWorkflowUpdatedEvent dispatches events to the audit log regarding the workflow
func (uc *WorkflowUseCase) dispatchWorkflowUpdatedEvent(ctx context.Context, wf *Workflow, opts *WorkflowUpdateOpts, newWfContract *WorkflowContract, orgUUID uuid.UUID) {
	baseOptions := &events.WorkflowUpdated{
		WorkflowBase: &events.WorkflowBase{
			WorkflowID:   &wf.ID,
			WorkflowName: wf.Name,
			ProjectName:  wf.Project,
		},
		NewDescription: opts.Description,
		NewTeam:        opts.Team,
		NewPublic:      opts.Public,
	}

	if opts.ContractID != nil && newWfContract != nil {
		baseOptions.NewWorkflowContractName = &newWfContract.Name
		baseOptions.NewWorkflowContractID = &newWfContract.ID
	}

	uc.auditorUC.Dispatch(ctx, baseOptions, &orgUUID)
}

// handleContractChangeEvents dispatches events to the audit log regarding the contract
func (uc *WorkflowUseCase) handleContractChangeEvents(ctx context.Context, preUpdateWorkflow *Workflow, wf *Workflow, newWfContract *WorkflowContract, orgUUID uuid.UUID) {
	// Only process contract events if the contract ID has actually changed
	if newWfContract != nil && preUpdateWorkflow.ContractID != newWfContract.ID {
		if preUpdateWorkflow.ContractID != uuid.Nil {
			uc.dispatchContractEvent(ctx, orgUUID, preUpdateWorkflow.ContractID, preUpdateWorkflow.ContractName, wf.ID, wf.Name, false)
		}
		uc.dispatchContractEvent(ctx, orgUUID, newWfContract.ID, newWfContract.Name, wf.ID, wf.Name, true)
	}
}

// dispatchContractEvent dispatches events to the audit log regarding the contract
func (uc *WorkflowUseCase) dispatchContractEvent(ctx context.Context, orgID uuid.UUID, contractID uuid.UUID, contractName string, wfID uuid.UUID, wfName string, attached bool) {
	contractBase := &events.WorkflowContractBase{
		WorkflowContractID:   &contractID,
		WorkflowContractName: contractName,
	}

	if attached {
		uc.auditorUC.Dispatch(ctx, &events.WorkflowContractAttached{
			WorkflowContractBase: contractBase,
			WorkflowID:           &wfID,
			WorkflowName:         wfName,
		}, &orgID)
	} else {
		uc.auditorUC.Dispatch(ctx, &events.WorkflowContractDetached{
			WorkflowContractBase: contractBase,
			WorkflowID:           &wfID,
			WorkflowName:         wfName,
		}, &orgID)
	}
}

func (uc *WorkflowUseCase) findOrCreateContract(ctx context.Context, orgID, name string) (*WorkflowContract, error) {
	e, _ := uc.contractUC.FindByNameInOrg(ctx, orgID, name)
	if e != nil {
		return e, nil
	}

	return uc.contractUC.Create(ctx, &WorkflowContractCreateOpts{OrgID: orgID, Name: name, AddUniquePrefix: true})
}

// List returns a list of workflows and the total count of workflows
func (uc *WorkflowUseCase) List(ctx context.Context, orgID string, filterOpts *WorkflowListOpts, paginationOpts *pagination.OffsetPaginationOpts) ([]*Workflow, int, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, 0, NewErrInvalidUUID(err)
	}
	// Apply default pagination if not provided
	pgOpts := pagination.NewDefaultOffsetPaginationOpts()
	if paginationOpts != nil {
		pgOpts = paginationOpts
	}

	return uc.wfRepo.List(ctx, orgUUID, filterOpts, pgOpts)
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

func (uc *WorkflowUseCase) FindByNameInOrg(ctx context.Context, orgID, projectName, workflowName string) (*Workflow, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	if workflowName == "" {
		return nil, NewErrValidationStr("empty workflow name")
	} else if projectName == "" {
		return nil, NewErrValidationStr("empty project name")
	}

	wf, err := uc.wfRepo.GetOrgScopedByProjectAndName(ctx, orgUUID, projectName, workflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
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
	wf, err := uc.wfRepo.GetOrgScoped(ctx, orgUUID, workflowUUID)
	if err != nil {
		return fmt.Errorf("failed to get workflow: %w", err)
	} else if wf == nil {
		return NewErrNotFound("organization")
	}

	if err := uc.wfRepo.SoftDelete(ctx, workflowUUID); err != nil {
		return fmt.Errorf("failed to soft delete workflow: %w", err)
	}

	uc.auditorUC.Dispatch(ctx, &events.WorkflowDeleted{
		WorkflowBase: &events.WorkflowBase{
			WorkflowID:   &workflowUUID,
			WorkflowName: wf.Name,
			ProjectName:  wf.Project,
		},
	}, &orgUUID)

	return nil
}
