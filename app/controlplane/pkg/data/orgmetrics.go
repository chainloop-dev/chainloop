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
	"sort"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowrun"
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

func (repo *OrgMetricsRepo) RunsTotal(ctx context.Context, orgID uuid.UUID, tw *biz.TimeWindow) (int32, error) {
	total, err := orgScopedQuery(repo.data.DB, orgID).
		QueryWorkflows().
		QueryWorkflowruns().
		Where(
			workflowrun.CreatedAtGTE(tw.From),
			workflowrun.CreatedAtLTE(tw.To),
		).
		Count(ctx)

	if err != nil {
		return 0, err
	}

	return int32(total), nil
}

func (repo *OrgMetricsRepo) RunsByStatusTotal(ctx context.Context, orgID uuid.UUID, tw *biz.TimeWindow) (map[string]int32, error) {
	var runs []struct {
		State string
		Count int32
	}

	if err := orgScopedQuery(repo.data.DB, orgID).
		QueryWorkflows().
		QueryWorkflowruns().
		Where(
			workflowrun.CreatedAtGTE(tw.From),
			workflowrun.CreatedAtLTE(tw.To),
		).
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

func (repo *OrgMetricsRepo) RunsByRunnerTypeTotal(ctx context.Context, orgID uuid.UUID, tw *biz.TimeWindow) (map[string]int32, error) {
	var runs []struct {
		RunnerType string `json:"runner_type"`
		Count      int32
	}

	if err := orgScopedQuery(repo.data.DB, orgID).
		QueryWorkflows().
		QueryWorkflowruns().
		Where(
			workflowrun.CreatedAtGTE(tw.From),
			workflowrun.CreatedAtLTE(tw.To),
		).
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

func (repo *OrgMetricsRepo) TopWorkflowsByRunsCount(ctx context.Context, orgID uuid.UUID, numWorkflows int, tw *biz.TimeWindow) ([]*biz.TopWorkflowsByRunsCountItem, error) {
	var runs []struct {
		WorkflowID string `json:"workflow_workflowruns"`
		State      string
		Count      int32
	}

	// Get workflow runs grouped by state and workflowRunID
	if err := orgScopedQuery(repo.data.DB, orgID).
		QueryWorkflows().
		QueryWorkflowruns().
		WithWorkflow().
		Where(
			workflowrun.CreatedAtGTE(tw.From),
			workflowrun.CreatedAtLTE(tw.To),
		).
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

			wf, err := orgScopedQuery(repo.data.DB, orgID).QueryWorkflows().Where(workflow.ID(workflowID)).First(ctx)
			if err != nil {
				if ent.IsNotFound(err) {
					continue
				}

				return nil, err
			}

			wfRes, err := entWFToBizWF(ctx, wf, nil)
			if err != nil {
				return nil, fmt.Errorf("converting entity: %w", err)
			}

			item = &biz.TopWorkflowsByRunsCountItem{ByStatus: make(map[string]int32), Workflow: wfRes}
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

func (repo *OrgMetricsRepo) DailyRunsCount(ctx context.Context, orgID, workflowID uuid.UUID, tw *biz.TimeWindow) ([]*biz.DayRunsCount, error) {
	var runsByStateAndDay []struct {
		State     string
		Count     int32
		CreatedAt time.Time `sql:"creation_day"`
	}

	// Get workflow runs grouped by state and day
	q := orgScopedQuery(repo.data.DB, orgID).QueryWorkflows()
	// optionally filter by workflowID
	if workflowID != uuid.Nil {
		q = q.Where(workflow.ID(workflowID))
	}

	err := q.QueryWorkflowruns().
		Where(
			workflowrun.CreatedAtGTE(tw.From),
			workflowrun.CreatedAtLTE(tw.To),
		).
		// group by day and state
		Modify(func(s *sql.Selector) {
			s.GroupBy("creation_day", workflowrun.FieldState)
			s.Select(
				sql.Count("*"),
				sql.As(fmt.Sprintf("Date(%s)", workflowrun.FieldCreatedAt), "creation_day"),
				workflowrun.FieldState)
		}).
		Scan(ctx, &runsByStateAndDay)
	if err != nil {
		return nil, err
	}

	// format tne date in string format
	m := make(map[string]*biz.DayRunsCount)
	for _, r := range runsByStateAndDay {
		date := r.CreatedAt.Format("2006-01-02")
		// It does not exist yet, so we create it
		if _, ok := m[date]; !ok {
			m[date] = &biz.DayRunsCount{Date: r.CreatedAt, Totals: []*biz.ByStatusCount{
				{Status: r.State, Count: r.Count},
			}}
			// If it exists we just append the information about the new state associated to the same day
		} else {
			m[date].Totals = append(m[date].Totals, &biz.ByStatusCount{Status: r.State, Count: r.Count})
		}
	}

	// sort map by date
	var keys = make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	res := make([]*biz.DayRunsCount, 0, len(m))
	for _, k := range keys {
		res = append(res, m[k])
	}

	return res, nil
}
