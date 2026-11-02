//
// Copyright 2023-2026 The Chainloop Authors.
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
	"bytes"
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/predicate"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflow"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/workflowrun"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

var orgMetricsRepoTracer = otelx.Tracer("chainloop-controlplane", "data/orgmetrics")

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

func (repo *OrgMetricsRepo) RunsTotal(ctx context.Context, orgID uuid.UUID, tw *biz.TimeWindow, projectIDs []uuid.UUID) (int32, error) {
	ctx, span := otelx.Start(ctx, orgMetricsRepoTracer, "OrgMetricsRepo.RunsTotal")
	defer span.End()

	wfQuery := orgScopedQuery(repo.data.DB, orgID).
		QueryWorkflows()

	if projectIDs != nil {
		wfQuery = wfQuery.Where(workflow.ProjectIDIn(projectIDs...))
	}

	total, err := wfQuery.WithProject().
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

func (repo *OrgMetricsRepo) RunsByStatusTotal(ctx context.Context, orgID uuid.UUID, tw *biz.TimeWindow, projectIDs []uuid.UUID) (map[string]int32, error) {
	ctx, span := otelx.Start(ctx, orgMetricsRepoTracer, "OrgMetricsRepo.RunsByStatusTotal")
	defer span.End()

	var runs []struct {
		State string
		Count int32
	}

	wfQuery := orgScopedQuery(repo.data.DB, orgID).
		QueryWorkflows()

	if projectIDs != nil {
		wfQuery = wfQuery.Where(workflow.ProjectIDIn(projectIDs...))
	}

	if err := wfQuery.WithProject().
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

func (repo *OrgMetricsRepo) RunsByRunnerTypeTotal(ctx context.Context, orgID uuid.UUID, tw *biz.TimeWindow, projectIDs []uuid.UUID) (map[string]int32, error) {
	ctx, span := otelx.Start(ctx, orgMetricsRepoTracer, "OrgMetricsRepo.RunsByRunnerTypeTotal")
	defer span.End()

	var runs []struct {
		RunnerType string `json:"runner_type"`
		Count      int32
	}

	wfQuery := orgScopedQuery(repo.data.DB, orgID).
		QueryWorkflows()

	if projectIDs != nil {
		wfQuery = wfQuery.Where(workflow.ProjectIDIn(projectIDs...))
	}

	if err := wfQuery.QueryWorkflowruns().
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

func (repo *OrgMetricsRepo) TopWorkflowsByRunsCount(ctx context.Context, orgID uuid.UUID, numWorkflows int, tw *biz.TimeWindow, projectIDs []uuid.UUID) ([]*biz.TopWorkflowsByRunsCountItem, error) {
	ctx, span := otelx.Start(ctx, orgMetricsRepoTracer, "OrgMetricsRepo.TopWorkflowsByRunsCount")
	defer span.End()

	// Filter on the denormalized workflow_runs.organization_id so the
	// (organization_id, created_at) index drives the scan, instead of reaching
	// the runs through the workflows table. Runs of deleted workflows are excluded.
	wfPredicates := []predicate.Workflow{workflow.DeletedAtIsNil()}
	if projectIDs != nil {
		wfPredicates = append(wfPredicates, workflow.ProjectIDIn(projectIDs...))
	}

	var runs []struct {
		WorkflowID uuid.UUID `json:"workflow_id"`
		State      string    `json:"state"`
		Count      int32     `json:"count"`
	}

	if err := repo.data.DB.WorkflowRun.Query().
		Where(
			workflowrun.OrganizationID(orgID),
			workflowrun.CreatedAtGTE(tw.From),
			workflowrun.CreatedAtLTE(tw.To),
			workflowrun.HasWorkflowWith(wfPredicates...),
		).
		GroupBy(workflowrun.FieldWorkflowID, workflowrun.FieldState).
		Aggregate(ent.Count()).
		Scan(ctx, &runs); err != nil {
		return nil, fmt.Errorf("aggregating workflow runs: %w", err)
	}

	totals := make(map[uuid.UUID]*biz.TopWorkflowsByRunsCountItem)
	for _, r := range runs {
		item, ok := totals[r.WorkflowID]
		if !ok {
			item = &biz.TopWorkflowsByRunsCountItem{ByStatus: make(map[string]int32)}
			totals[r.WorkflowID] = item
		}

		item.ByStatus[r.State] = r.Count
		item.Total += r.Count
	}

	// Pick the top workflows before loading them, sorted by total runs.
	// Ties are broken by ID so the result is deterministic.
	topIDs := slices.Collect(maps.Keys(totals))
	slices.SortFunc(topIDs, func(a, b uuid.UUID) int {
		if c := cmp.Compare(totals[b].Total, totals[a].Total); c != 0 {
			return c
		}
		return bytes.Compare(a[:], b[:])
	})

	if len(topIDs) > numWorkflows {
		topIDs = topIDs[:numWorkflows]
	}

	result := make([]*biz.TopWorkflowsByRunsCountItem, 0, len(topIDs))
	if len(topIDs) == 0 {
		return result, nil
	}

	// Load all the selected workflows and their projects in a single round-trip
	wfs, err := repo.data.DB.Workflow.Query().
		Where(workflow.IDIn(topIDs...), workflow.OrganizationID(orgID), workflow.DeletedAtIsNil()).
		WithProject().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading workflows: %w", err)
	}

	wfByID := make(map[uuid.UUID]*ent.Workflow, len(wfs))
	for _, wf := range wfs {
		wfByID[wf.ID] = wf
	}

	for _, id := range topIDs {
		wf, ok := wfByID[id]
		if !ok {
			// deleted between both queries
			continue
		}

		wfRes, err := entWFToBizWF(ctx, wf)
		if err != nil {
			return nil, fmt.Errorf("converting entity: %w", err)
		}

		item := totals[id]
		item.Workflow = wfRes
		result = append(result, item)
	}

	return result, nil
}

func (repo *OrgMetricsRepo) DailyRunsCount(ctx context.Context, orgID, workflowID uuid.UUID, tw *biz.TimeWindow, projectIDs []uuid.UUID) ([]*biz.DayRunsCount, error) {
	ctx, span := otelx.Start(ctx, orgMetricsRepoTracer, "OrgMetricsRepo.DailyRunsCount")
	defer span.End()

	var runsByStateAndDay []struct {
		State     string
		Count     int32
		CreatedAt time.Time `sql:"creation_day"`
	}

	// Get workflow runs grouped by state and day
	q := orgScopedQuery(repo.data.DB, orgID).QueryWorkflows()

	// filter by visible projects
	if projectIDs != nil {
		q = q.Where(workflow.ProjectIDIn(projectIDs...))
	}

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
