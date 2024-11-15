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
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/predicate"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowrun"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/integrationattachment"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/project"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
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

func (r *WorkflowRepo) Create(ctx context.Context, opts *biz.WorkflowCreateOpts) (wf *biz.Workflow, err error) {
	orgUUID, err := uuid.Parse(opts.OrgID)
	if err != nil {
		return nil, err
	}

	contractUUID, err := uuid.Parse(opts.ContractID)
	if err != nil {
		return nil, err
	}

	// Create project and workflow in a transaction
	tx, err := r.data.DB.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.log.Errorf("rolling back transaction: %v", rbErr)
			}
		}
	}()

	// Find or create project.
	projectID, err := tx.Project.Create().SetName(opts.Project).SetOrganizationID(orgUUID).
		OnConflict(
			sql.ConflictColumns(project.FieldName, project.FieldOrganizationID),
			// Since we are using a partial index, we need to explicitly craft the upsert query
			sql.ConflictWhere(sql.IsNull(project.FieldDeletedAt)),
		).Ignore().ID(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating project: %w", err)
	}

	entwf, err := tx.Workflow.Create().
		SetName(opts.Name).
		SetProjectID(projectID).
		SetTeam(opts.Team).
		SetPublic(opts.Public).
		SetName(opts.Name).
		SetContractID(contractUUID).
		SetOrganizationID(orgUUID).
		SetDescription(opts.Description).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return nil, biz.NewErrAlreadyExists(err)
		}

		return nil, fmt.Errorf("failed to create workflow: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return r.FindByID(ctx, entwf.ID)
}

func (r *WorkflowRepo) Update(ctx context.Context, id uuid.UUID, opts *biz.WorkflowUpdateOpts) (*biz.Workflow, error) {
	if opts == nil {
		opts = &biz.WorkflowUpdateOpts{}
	}

	req := r.data.DB.Workflow.UpdateOneID(id).
		SetNillableTeam(opts.Team).
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
			return nil, biz.NewErrAlreadyExists(err)
		}

		return nil, fmt.Errorf("failed to update workflow: %w", err)
	}

	// Reload the object to include the relations
	return r.FindByID(ctx, wf.ID)
}

func (r *WorkflowRepo) List(ctx context.Context, orgID uuid.UUID, filter *biz.WorkflowListOpts, pagination *pagination.OffsetPaginationOpts) ([]*biz.Workflow, int, error) {
	if pagination == nil {
		return nil, 0, fmt.Errorf("pagination options is required")
	}

	// Initialize the base query for WorkflowRun
	baseQuery := orgScopedQuery(r.data.DB, orgID).QueryWorkflows()

	// Apply filters to the WorkflowRun query based on the provided options
	baseQuery = applyWorkflowRunFilters(baseQuery, filter)

	// Initialize the Workflow query and apply organization and deletion filters
	wfQuery := baseQuery.Where(workflow.DeletedAtIsNil())

	// Apply additional filters to the Workflow query based on the provided options
	wfQuery = applyWorkflowFilters(wfQuery, filter)

	// Get the count of all filtered rows without the limit and offset
	count, err := wfQuery.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Apply pagination options and execute the query
	workflows, err := wfQuery.
		WithLatestWorkflowRun().
		Order(ent.Desc(workflow.FieldCreatedAt)).
		Limit(pagination.Limit()).
		Offset(pagination.Offset()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*biz.Workflow, 0, len(workflows))
	for _, wf := range workflows {
		r, err := entWFToBizWF(ctx, wf)
		if err != nil {
			return nil, 0, fmt.Errorf("converting entity: %w", err)
		}

		result = append(result, r)
	}

	return result, count, nil
}

// applyWorkflowRunFilters applies filters to the WorkflowRun query based on the provided options
func applyWorkflowRunFilters(baseQuery *ent.WorkflowQuery, opts *biz.WorkflowListOpts) *ent.WorkflowQuery {
	if opts == nil || opts.WorkflowRunRunnerType == "" && opts.WorkflowRunLastStatus == "" {
		return baseQuery
	}

	query := baseQuery.QueryLatestWorkflowRun()

	if opts.WorkflowRunRunnerType != "" {
		query = query.Where(
			workflowrun.RunnerType(opts.WorkflowRunRunnerType),
		)
	}

	if opts.WorkflowRunLastStatus != "" {
		query = query.Where(
			workflowrun.StateEQ(opts.WorkflowRunLastStatus),
		)
	}

	return query.QueryWorkflow()
}

// applyWorkflowFilters applies filters to the Workflow query based on the provided options
func applyWorkflowFilters(wfQuery *ent.WorkflowQuery, opts *biz.WorkflowListOpts) *ent.WorkflowQuery {
	if opts != nil {
		if opts.WorkflowPublic != nil {
			wfQuery = wfQuery.Where(workflow.Public(*opts.WorkflowPublic))
		}

		// Updated at on Workflows is only updated when a new workflow run is referenced meaning
		// a workflow run is started
		if opts.WorkflowActiveWindow != nil {
			wfQuery = wfQuery.Where(
				workflow.UpdatedAtGTE(opts.WorkflowActiveWindow.From),
				workflow.UpdatedAtLTE(opts.WorkflowActiveWindow.To),
			)
		}

		if opts.WorkflowDescription != "" {
			wfQuery = wfQuery.Where(workflow.DescriptionContains(opts.WorkflowDescription))
		}

		if len(opts.WorkflowProjectNames) != 0 {
			wfQuery = wfQuery.Where(workflow.HasProjectWith(project.NameIn(opts.WorkflowProjectNames...)))
		}

		// Combine WorkflowTeam and WorkflowName filters using OR logic
		var orConditions []predicate.Workflow
		if opts.WorkflowTeam != "" {
			orConditions = append(orConditions, workflow.TeamContains(opts.WorkflowTeam))
		}
		if opts.WorkflowName != "" {
			orConditions = append(orConditions, workflow.NameContains(opts.WorkflowName))
		}

		if len(orConditions) > 0 {
			wfQuery = wfQuery.Where(workflow.Or(orConditions...))
		}
	}

	return wfQuery
}

// GetOrgScoped Gets a workflow making sure it belongs to a given org
func (r *WorkflowRepo) GetOrgScoped(ctx context.Context, orgID, workflowID uuid.UUID) (*biz.Workflow, error) {
	workflow, err := orgScopedQuery(r.data.DB, orgID).
		QueryWorkflows().
		Where(workflow.ID(workflowID), workflow.DeletedAtIsNil()).
		WithContract().WithOrganization().WithLatestWorkflowRun().
		Order(ent.Desc(workflow.FieldCreatedAt)).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("workflow")
		}
		return nil, err
	}

	return entWFToBizWF(ctx, workflow)
}

// GetOrgScopedByProjectAndName Gets a workflow by name making sure it belongs to a given org
func (r *WorkflowRepo) GetOrgScopedByProjectAndName(ctx context.Context, orgID uuid.UUID, projectName, workflowName string) (*biz.Workflow, error) {
	wf, err := orgScopedQuery(r.data.DB, orgID).QueryWorkflows().
		Where(workflow.HasProjectWith(project.Name(projectName)), workflow.Name(workflowName), workflow.DeletedAtIsNil()).
		WithContract().WithOrganization().WithProject().WithLatestWorkflowRun().
		Order(ent.Desc(workflow.FieldCreatedAt)).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("workflow")
		}
		return nil, err
	}

	return entWFToBizWF(ctx, wf)
}

func (r *WorkflowRepo) IncRunsCounter(ctx context.Context, workflowID uuid.UUID) error {
	return r.data.DB.Workflow.Update().AddRunsCount(1).Where(workflow.ID(workflowID)).Exec(ctx)
}

func (r *WorkflowRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.Workflow, error) {
	workflow, err := r.data.DB.Workflow.Query().
		Where(workflow.DeletedAtIsNil(), workflow.ID(id)).
		WithContract().WithOrganization().WithLatestWorkflowRun().
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, biz.NewErrNotFound("workflow")
		}
		return nil, err
	}

	return entWFToBizWF(ctx, workflow)
}

// Soft delete workflow, attachments and related projects (if applicable)
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
	wf, err := tx.Workflow.UpdateOneID(id).SetDeletedAt(time.Now()).SetUpdatedAt(time.Now()).Save(ctx)
	if err != nil {
		return err
	}

	// Soft delete project if it has no more workflows
	// TODO: in the future, we'll handle this separately through explicit user action
	if wfTotal, err := wf.QueryProject().QueryWorkflows().Where(workflow.DeletedAtIsNil()).Count(ctx); err != nil {
		return err
	} else if wfTotal == 0 {
		// soft deleted project if it has no more workflows
		if err := tx.Project.UpdateOneID(wf.ProjectID).SetDeletedAt(time.Now()).Exec(ctx); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func entWFToBizWF(ctx context.Context, w *ent.Workflow) (*biz.Workflow, error) {
	wf := &biz.Workflow{Name: w.Name, ID: w.ID,
		CreatedAt: toTimePtr(w.CreatedAt), Team: w.Team,
		RunsCounter: w.RunsCount,
		Public:      w.Public,
		Description: w.Description,
		OrgID:       w.OrganizationID,
	}

	// Set project either pre-loaded or queried
	if project := w.Edges.Project; project != nil {
		wf.Project = project.Name
	} else {
		project, err := w.QueryProject().Only(ctx)
		if err != nil {
			return nil, err
		}
		wf.Project = project.Name
		wf.ProjectID = project.ID
	}

	if wf.Project == "" {
		return nil, fmt.Errorf("workflow %s has no project", w.ID)
	}

	if contract := w.Edges.Contract; contract != nil {
		wf.ContractID = contract.ID
		wf.ContractName = contract.Name
		lv, err := latestVersion(context.Background(), contract)
		if err != nil {
			return nil, fmt.Errorf("finding contract version: %w", err)
		}
		wf.ContractRevisionLatest = lv.Revision
	}

	if latestRun := w.Edges.LatestWorkflowRun; latestRun != nil {
		lastRun, err := entWrToBizWr(ctx, latestRun)
		if err != nil {
			return nil, fmt.Errorf("converting workflow run: %w", err)
		}

		wf.LastRun = lastRun
	}

	return wf, nil
}
