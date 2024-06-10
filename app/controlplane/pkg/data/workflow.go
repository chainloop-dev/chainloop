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
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/integrationattachment"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowrun"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type WorkflowRepo struct {
	data *Data
	log  *log.Helper
}

var _ biz.WorkflowRepo = (*WorkflowRepo)(nil)

func NewWorkflowRepo(data *Data, logger log.Logger) biz.WorkflowRepo {
	return &WorkflowRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *WorkflowRepo) Create(ctx context.Context, opts *biz.WorkflowCreateOpts) (*biz.Workflow, error) {
	orgUUID, err := uuid.Parse(opts.OrgID)
	if err != nil {
		return nil, err
	}

	contractUUID, err := uuid.Parse(opts.ContractID)
	if err != nil {
		return nil, err
	}

	wf, err := r.data.DB.Workflow.Create().
		SetName(opts.Name).
		SetProject(opts.Project).
		SetTeam(opts.Team).
		SetPublic(opts.Public).
		SetName(opts.Name).
		SetContractID(contractUUID).
		SetOrganizationID(orgUUID).
		SetDescription(opts.Description).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.ErrAlreadyExists
		}

		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	return r.FindByID(ctx, wf.ID)
}

func (r *WorkflowRepo) Update(ctx context.Context, id uuid.UUID, opts *biz.WorkflowUpdateOpts) (*biz.Workflow, error) {
	if opts == nil {
		opts = &biz.WorkflowUpdateOpts{}
	}

	req := r.data.DB.Workflow.UpdateOneID(id).
		SetNillableTeam(opts.Team).
		SetNillableProject(opts.Project).
		SetNillablePublic(opts.Public).
		SetNillableDescription(opts.Description)

	// Update the contract if provided
	if opts.ContractID != nil {
		contractUUID, err := uuid.Parse(*opts.ContractID)
		if err != nil {
			return nil, err
		}
		req = req.SetContractID(contractUUID)
	}

	wf, err := req.Save(ctx)

	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.ErrAlreadyExists
		}

		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}

	// Reload the object to include the relations
	return r.FindByID(ctx, wf.ID)
}

func (r *WorkflowRepo) List(ctx context.Context, orgID uuid.UUID) ([]*biz.Workflow, error) {
	workflows, err := orgScopedQuery(r.data.DB, orgID).
		QueryWorkflows().
		Where(workflow.DeletedAtIsNil()).
		WithContract().WithOrganization().
		Order(ent.Desc(workflow.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*biz.Workflow, 0, len(workflows))
	for _, wf := range workflows {
		// Not efficient, we need to do a query limit = 1 grouped by workflowID
		lastRun, err := getLastRun(ctx, wf)
		if err != nil {
			return nil, err
		}

		r, err := entWFToBizWF(wf, lastRun)
		if err != nil {
			return nil, fmt.Errorf("converting entity: %w", err)
		}

		result = append(result, r)
	}

	return result, nil
}

// GetOrgScoped Gets a workflow making sure it belongs to a given org
func (r *WorkflowRepo) GetOrgScoped(ctx context.Context, orgID, workflowID uuid.UUID) (*biz.Workflow, error) {
	workflow, err := orgScopedQuery(r.data.DB, orgID).
		QueryWorkflows().
		Where(workflow.ID(workflowID), workflow.DeletedAtIsNil()).
		WithContract().WithOrganization().
		Order(ent.Desc(workflow.FieldCreatedAt)).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("workflow")
		}
		return nil, err
	}

	// Not efficient, we need to do a query limit = 1 grouped by workflowID
	lastRun, err := getLastRun(ctx, workflow)
	if err != nil {
		return nil, err
	}

	return entWFToBizWF(workflow, lastRun)
}

// GetOrgScopedByName Gets a workflow by name making sure it belongs to a given org
func (r *WorkflowRepo) GetOrgScopedByName(ctx context.Context, orgID uuid.UUID, workflowName string) (*biz.Workflow, error) {
	wf, err := orgScopedQuery(r.data.DB, orgID).QueryWorkflows().
		Where(workflow.Name(workflowName), workflow.DeletedAtIsNil()).
		WithContract().WithOrganization().
		Order(ent.Desc(workflow.FieldCreatedAt)).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("workflow")
		}
		return nil, err
	}

	// Not efficient, we need to do a query limit = 1 grouped by workflowID
	lastRun, err := getLastRun(ctx, wf)
	if err != nil {
		return nil, err
	}

	return entWFToBizWF(wf, lastRun)
}

func (r *WorkflowRepo) IncRunsCounter(ctx context.Context, workflowID uuid.UUID) error {
	return r.data.DB.Workflow.Update().AddRunsCount(1).Where(workflow.ID(workflowID)).Exec(ctx)
}

func (r *WorkflowRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.Workflow, error) {
	workflow, err := r.data.DB.Workflow.Query().
		Where(workflow.DeletedAtIsNil(), workflow.ID(id)).
		WithContract().WithOrganization().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("workflow")
		}
		return nil, err
	}

	// Not efficient, we need to do a query limit = 1 grouped by workflowID
	lastRun, err := getLastRun(ctx, workflow)
	if err != nil {
		return nil, err
	}

	return entWFToBizWF(workflow, lastRun)
}

func (r *WorkflowRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	tx, err := r.data.DB.Tx(ctx)
	if err != nil {
		return err
	}

	// soft-delete attachments associated with this workflow
	if err := tx.IntegrationAttachment.Update().Where(integrationattachment.HasWorkflowWith(workflow.ID(id))).SetDeletedAt(time.Now()).Exec(ctx); err != nil {
		return err
	}

	// Soft delete workflow
	if err := tx.Workflow.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx); err != nil {
		return err
	}

	return tx.Commit()
}

func entWFToBizWF(w *ent.Workflow, r *ent.WorkflowRun) (*biz.Workflow, error) {
	wf := &biz.Workflow{Name: w.Name, ID: w.ID,
		CreatedAt: toTimePtr(w.CreatedAt), Team: w.Team,
		Project: w.Project, RunsCounter: w.RunsCount,
		Public:      w.Public,
		Description: w.Description,
	}

	if contract := w.Edges.Contract; contract != nil {
		wf.ContractID = contract.ID
		lv, err := latestVersion(context.Background(), contract)
		if err != nil {
			return nil, fmt.Errorf("finding contract version: %w", err)
		}
		wf.ContractRevisionLatest = lv.Revision
	}

	if org := w.Edges.Organization; org != nil {
		wf.OrgID = org.ID
	}

	if r != nil {
		wf.LastRun = entWrToBizWr(r)
	}

	return wf, nil
}

func getLastRun(ctx context.Context, wf *ent.Workflow) (*ent.WorkflowRun, error) {
	lastRun, err := wf.QueryWorkflowruns().WithWorkflow().Order(ent.Desc(workflowrun.FieldCreatedAt)).Limit(1).All(ctx)
	if len(lastRun) == 0 {
		return nil, err
	}

	return lastRun[0], nil
}
