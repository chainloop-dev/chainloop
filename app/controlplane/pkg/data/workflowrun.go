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
	"errors"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/pagination"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/organization"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowrun"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"

	"entgo.io/ent/dialect/sql"
)

type WorkflowRunRepo struct {
	data *Data
	log  *log.Helper
}

func NewWorkflowRunRepo(data *Data, logger log.Logger) biz.WorkflowRunRepo {
	return &WorkflowRunRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *WorkflowRunRepo) Create(ctx context.Context, opts *biz.WorkflowRunRepoCreateOpts) (*biz.WorkflowRun, error) {
	// Find the contract to calculate the revisions
	p, err := r.data.DB.WorkflowRun.Create().
		SetWorkflowID(opts.WorkflowID).
		SetContractVersionID(opts.SchemaVersionID).
		SetRunURL(opts.RunURL).
		SetRunnerType(opts.RunnerType).
		AddCasBackendIDs(opts.Backends...).
		SetContractRevisionLatest(opts.LatestRevision).
		SetContractRevisionUsed(opts.UsedRevision).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, p.ID)
}

func eagerLoadWorkflowRun(client *ent.Client) *ent.WorkflowRunQuery {
	return client.WorkflowRun.Query().
		WithWorkflow(func(q *ent.WorkflowQuery) { q.WithOrganization() }).
		WithContractVersion().
		WithCasBackends()
}

func (r *WorkflowRunRepo) FindByID(ctx context.Context, id uuid.UUID) (*biz.WorkflowRun, error) {
	run, err := eagerLoadWorkflowRun(r.data.DB).Where(workflowrun.ID(id)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if run == nil {
		return nil, nil
	}

	return entWrToBizWr(run), nil
}

func (r *WorkflowRunRepo) FindByAttestationDigest(ctx context.Context, digest string) (*biz.WorkflowRun, error) {
	run, err := eagerLoadWorkflowRun(r.data.DB).Where(workflowrun.AttestationDigest(digest)).Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if run == nil {
		return nil, nil
	}

	return entWrToBizWr(run), nil
}

func (r *WorkflowRunRepo) FindByIDInOrg(ctx context.Context, orgID, id uuid.UUID) (*biz.WorkflowRun, error) {
	run, err := orgScopedQuery(r.data.DB, orgID).
		QueryWorkflows().
		QueryWorkflowruns().Where(workflowrun.ID(id)).
		WithWorkflow().WithContractVersion().WithCasBackends().
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, err
	} else if run == nil {
		return nil, biz.NewErrNotFound("workflow run")
	}

	return entWrToBizWr(run), nil
}

// Save the attestation for a workflow run in the database
func (r *WorkflowRunRepo) SaveAttestation(ctx context.Context, id uuid.UUID, att *dsse.Envelope, digest string) error {
	run, err := r.data.DB.WorkflowRun.UpdateOneID(id).
		SetAttestation(att).
		SetAttestationDigest(digest).
		Save(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return err
	} else if run == nil {
		return biz.NewErrNotFound(fmt.Sprintf("workflow run with id %s not found", id))
	}

	return nil
}

func (r *WorkflowRunRepo) MarkAsFinished(ctx context.Context, id uuid.UUID, status biz.WorkflowRunStatus, reason string) error {
	run, err := r.data.DB.WorkflowRun.Query().Where(workflowrun.ID(id)).WithWorkflow().First(ctx)
	if err != nil {
		return fmt.Errorf("failed to find workflow run: %w", err)
	}

	if err = run.Update().SetFinishedAt(time.Now()).SetState(status).SetReason(reason).ClearAttestationState().Exec(ctx); err != nil {
		return fmt.Errorf("failed to mark workflow run as finished: %w", err)
	}

	return nil
}

// List the runs in an organization, optionally filtered out by workflow
func (r *WorkflowRunRepo) List(ctx context.Context, orgID uuid.UUID, filters *biz.RunListFilters, p *pagination.CursorOptions) (result []*biz.WorkflowRun, cursor string, err error) {
	if p == nil {
		return nil, "", errors.New("pagination options is required")
	}

	orgQuery := r.data.DB.Organization.Query().Where(organization.ID(orgID))
	// Skip the runs that have a workflow marked as deleted
	wfQuery := orgQuery.QueryWorkflows().Where(workflow.DeletedAtIsNil())
	// Append the workflow filter if present
	if filters != nil && filters.WorkflowID != uuid.Nil {
		wfQuery = wfQuery.Where(workflow.ID(filters.WorkflowID))
	}

	wfRunsQuery := wfQuery.QueryWorkflowruns().
		Order(ent.Desc(workflowrun.FieldCreatedAt)).
		Limit(p.Limit + 1).WithWorkflow()

	// Append the state filter if present, i.e only running
	if filters != nil && filters.Status != "" {
		wfRunsQuery = wfRunsQuery.Where(workflowrun.StateEQ(filters.Status))
	}

	if p.Cursor != nil {
		wfRunsQuery = wfRunsQuery.Where(
			func(s *sql.Selector) {
				s.Where(sql.CompositeLT([]string{s.C(workflowrun.FieldCreatedAt), s.C(workflowrun.FieldID)}, p.Cursor.Timestamp, p.Cursor.ID))
			})
	}

	workflowRuns, err := wfRunsQuery.All(ctx)
	if err != nil {
		return nil, "", err
	}

	for i, wr := range workflowRuns {
		// Check if there are additional items for another page
		// if so, set the cursor to the last item in the window
		if i == p.Limit {
			prevwr := workflowRuns[i-1]
			cursor = pagination.EncodeCursor(prevwr.CreatedAt, prevwr.ID)
			continue
		}

		result = append(result, entWrToBizWr(wr))
	}

	return result, cursor, nil
}

func (r *WorkflowRunRepo) ListNotFinishedOlderThan(ctx context.Context, olderThan time.Time) ([]*biz.WorkflowRun, error) {
	// TODO: Look into adding upper bound on the createdAt column to prevent full table scans
	// For now this is fine especially because we have a composite index
	workflowRuns, err := r.data.DB.WorkflowRun.Query().
		WithWorkflow().
		Where(workflowrun.CreatedAtLTE(olderThan)).
		Where(workflowrun.StateEQ(biz.WorkflowRunInitialized)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*biz.WorkflowRun, 0, len(workflowRuns))
	for _, wr := range workflowRuns {
		result = append(result, entWrToBizWr(wr))
	}

	return result, nil
}

func (r *WorkflowRunRepo) Expire(ctx context.Context, id uuid.UUID) error {
	return r.data.DB.WorkflowRun.UpdateOneID(id).SetState(biz.WorkflowRunExpired).ClearAttestationState().Exec(ctx)
}

func entWrToBizWr(wr *ent.WorkflowRun) *biz.WorkflowRun {
	r := &biz.WorkflowRun{
		ID:                     wr.ID,
		CreatedAt:              toTimePtr(wr.CreatedAt),
		FinishedAt:             toTimePtr(wr.FinishedAt),
		State:                  string(wr.State),
		Reason:                 wr.Reason,
		RunURL:                 wr.RunURL,
		RunnerType:             wr.RunnerType,
		CASBackends:            make([]*biz.CASBackend, 0),
		ContractRevisionUsed:   wr.ContractRevisionUsed,
		ContractRevisionLatest: wr.ContractRevisionLatest,
	}

	if wr.Attestation != nil {
		r.Attestation = &biz.Attestation{
			Envelope: wr.Attestation,
			Digest:   wr.AttestationDigest,
		}
	}

	if cv := wr.Edges.ContractVersion; cv != nil {
		r.ContractVersionID = cv.ID
	}

	if wf := wr.Edges.Workflow; wf != nil {
		w, _ := entWFToBizWF(context.TODO(), wf, nil)
		r.Workflow = w
	}

	if backends := wr.Edges.CasBackends; backends != nil {
		for _, b := range backends {
			r.CASBackends = append(r.CASBackends, entCASBackendToBiz(b))
		}
	}

	return r
}
