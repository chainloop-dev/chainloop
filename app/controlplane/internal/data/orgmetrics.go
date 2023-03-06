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
	"sort"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/workflow"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/data/ent/workflowrun"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OrgMetricsRepo struct {
	data *Data
	log  *log.Helper
}

func NewOrgMetricsRepo(data *Data, l log.Logger) biz.OrgMetricsRepo {
	return &OrgMetricsRepo{
		data: data,
		log:  log.NewHelper(l),
	}
}

func (repo *OrgMetricsRepo) RunsTotal(ctx context.Context, orgID uuid.UUID, tw time.Duration) (int32, error) {
	total, err := orgScopedQuery(repo.data.db, orgID).
		QueryWorkflows().
		QueryWorkflowruns().
		Where(workflowrun.CreatedAtGTE(time.Now().Add(-tw))).
		Count(ctx)

	if err != nil {
		return 0, err
	}

	return int32(total), nil
}

func (repo *OrgMetricsRepo) RunsByStatusTotal(ctx context.Context, orgID uuid.UUID, tw time.Duration) (map[string]int32, error) {
	var runs []struct {
		State string
		Count int32
	}

	if err := orgScopedQuery(repo.data.db, orgID).
		QueryWorkflows().
		QueryWorkflowruns().
		Where(workflowrun.CreatedAtGTE(time.Now().Add(-tw))).
		GroupBy(workflowrun.FieldState).
		Aggregate(ent.Count()).
		Scan(ctx, &runs); err != nil {
		return nil, err
	}

	var result = make(map[string]int32)
	for _, r := range runs {
		result[r.State] = r.Count
	}

	return result, nil
}

func (repo *OrgMetricsRepo) RunsByRunnerTypeTotal(ctx context.Context, orgID uuid.UUID, tw time.Duration) (map[string]int32, error) {
	var runs []struct {
		RunnerType string `json:"runner_type"`
		Count      int32
	}

	if err := orgScopedQuery(repo.data.db, orgID).
		QueryWorkflows().
		QueryWorkflowruns().
		Where(workflowrun.CreatedAtGTE(time.Now().Add(-tw))).
		GroupBy(workflowrun.FieldRunnerType).
		Aggregate(ent.Count()).
		Scan(ctx, &runs); err != nil {
		return nil, err
	}

	var result = make(map[string]int32)
	for _, r := range runs {
		result[r.RunnerType] = r.Count
	}

	return result, nil
}

func (repo *OrgMetricsRepo) TopWorkflowsByRunsCount(ctx context.Context, orgID uuid.UUID, numWorkflows int, tw time.Duration) ([]*biz.TopWorkflowsByRunsCountItem, error) {
	var runs []struct {
		WorkflowID string `json:"workflow_workflowruns"`
		State      string
		Count      int32
	}

	// Get workflow runs grouped by state and workflowRunID
	if err := orgScopedQuery(repo.data.db, orgID).
		QueryWorkflows().
		QueryWorkflowruns().
		WithWorkflow().
		Where(workflowrun.CreatedAtGTE(time.Now().Add(-tw))).
		GroupBy(workflowrun.WorkflowColumn, workflowrun.FieldState).
		Aggregate(ent.Count()).
		Scan(ctx, &runs); err != nil {
		return nil, err
	}

	// Map resultMap to include totals
	resultMap := make(map[string]*biz.TopWorkflowsByRunsCountItem)
	for _, r := range runs {
		var item *biz.TopWorkflowsByRunsCountItem
		var found bool
		if item, found = resultMap[r.WorkflowID]; !found {
			workflowID, err := uuid.Parse(r.WorkflowID)
			if err != nil {
				return nil, err
			}

			wf, err := orgScopedQuery(repo.data.db, orgID).QueryWorkflows().Where(workflow.ID(workflowID)).First(ctx)
			if err != nil {
				if ent.IsNotFound(err) {
					continue
				}

				return nil, err
			}

			item = &biz.TopWorkflowsByRunsCountItem{ByStatus: make(map[string]int32), Workflow: entWFToBizWF(wf, nil)}
		}

		item.ByStatus[r.State] = r.Count
		item.Total += r.Count
		resultMap[r.WorkflowID] = item
	}

	result := make([]*biz.TopWorkflowsByRunsCountItem, 0, len(resultMap))
	for _, r := range resultMap {
		result = append(result, r)
	}

	// Sort and limit to numWorkflows
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Total > result[j].Total
	})

	if len(result) < numWorkflows {
		numWorkflows = len(result)
	}

	return result[0:numWorkflows], nil
}
