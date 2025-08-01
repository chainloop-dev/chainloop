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
	"time"

	prometheuscollector "github.com/chainloop-dev/chainloop/app/controlplane/pkg/metrics/prometheus/collector"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OrgMetricsUseCase struct {
	logger *log.Helper
	// Repositories
	repo    OrgMetricsRepo
	orgRepo OrganizationRepo
	// Use Cases
	wfUseCase *WorkflowUseCase
}

type OrgMetricsRepo interface {
	// Total number of runs within the provided time window (from now)
	RunsTotal(ctx context.Context, orgID uuid.UUID, timeWindow *TimeWindow, projectIDs []uuid.UUID) (int32, error)
	// Total number by run status
	RunsByStatusTotal(ctx context.Context, orgID uuid.UUID, timeWindow *TimeWindow, projectIDs []uuid.UUID) (map[string]int32, error)
	RunsByRunnerTypeTotal(ctx context.Context, orgID uuid.UUID, timeWindow *TimeWindow, projectIDs []uuid.UUID) (map[string]int32, error)
	TopWorkflowsByRunsCount(ctx context.Context, orgID uuid.UUID, numWorkflows int, timeWindow *TimeWindow, projectIDs []uuid.UUID) ([]*TopWorkflowsByRunsCountItem, error)
	DailyRunsCount(ctx context.Context, orgID, workflowID uuid.UUID, timeWindow *TimeWindow, projectIDs []uuid.UUID) ([]*DayRunsCount, error)
}

type DayRunsCount struct {
	Date   time.Time
	Totals []*ByStatusCount
}

type ByStatusCount struct {
	Status string
	Count  int32
}

// TimeWindow represents in time.Time format not in time.Duration
type TimeWindow struct {
	From time.Time
	To   time.Time
}

// Validate validates the time window checking From and To are set
func (tw *TimeWindow) Validate() error {
	if tw.From.IsZero() || tw.To.IsZero() {
		return NewErrInvalidTimeWindowStr("from and to time must be set in time window")
	}

	return nil
}

func NewOrgMetricsUseCase(r OrgMetricsRepo, orgRepo OrganizationRepo, wfUseCase *WorkflowUseCase, l log.Logger) (*OrgMetricsUseCase, error) {
	return &OrgMetricsUseCase{
		orgRepo:   orgRepo,
		wfUseCase: wfUseCase,
		logger:    log.NewHelper(l),
		repo:      r,
	}, nil
}

func (uc *OrgMetricsUseCase) RunsTotal(ctx context.Context, orgID string, timeWindow *TimeWindow, projectIDs []uuid.UUID) (int32, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return 0, err
	}

	if err := validateTimeWindowIsSet(timeWindow); err != nil {
		return 0, err
	}

	return uc.repo.RunsTotal(ctx, orgUUID, timeWindow, projectIDs)
}

func (uc *OrgMetricsUseCase) RunsTotalByStatus(ctx context.Context, orgID string, timeWindow *TimeWindow, projectIDs []uuid.UUID) (map[string]int32, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	if err := validateTimeWindowIsSet(timeWindow); err != nil {
		return nil, err
	}

	return uc.repo.RunsByStatusTotal(ctx, orgUUID, timeWindow, projectIDs)
}

func (uc *OrgMetricsUseCase) RunsTotalByRunnerType(ctx context.Context, orgID string, timeWindow *TimeWindow, projectIDs []uuid.UUID) (map[string]int32, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	if err := validateTimeWindowIsSet(timeWindow); err != nil {
		return nil, err
	}

	return uc.repo.RunsByRunnerTypeTotal(ctx, orgUUID, timeWindow, projectIDs)
}

// DailyRunsCount returns the number of runs per day within the provided time window (from now)
// Optionally filtered by workflowID
func (uc *OrgMetricsUseCase) DailyRunsCount(ctx context.Context, orgID string, workflowID *string, timeWindow *TimeWindow, projectIDs []uuid.UUID) ([]*DayRunsCount, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	if err := validateTimeWindowIsSet(timeWindow); err != nil {
		return nil, err
	}

	var workflowUUID uuid.UUID
	if workflowID != nil {
		workflowUUID, err = uuid.Parse(*workflowID)
		if err != nil {
			return nil, NewErrInvalidUUID(err)
		}
	}

	return uc.repo.DailyRunsCount(ctx, orgUUID, workflowUUID, timeWindow, projectIDs)
}

type TopWorkflowsByRunsCountItem struct {
	Workflow *Workflow
	ByStatus map[string]int32
	Total    int32
}

func (uc *OrgMetricsUseCase) TopWorkflowsByRunsCount(ctx context.Context, orgID string, numWorkflows int, timeWindow *TimeWindow, projectIDs []uuid.UUID) ([]*TopWorkflowsByRunsCountItem, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	if err := validateTimeWindowIsSet(timeWindow); err != nil {
		return nil, err
	}

	return uc.repo.TopWorkflowsByRunsCount(ctx, orgUUID, numWorkflows, timeWindow, projectIDs)
}

// GetLastWorkflowStatusByRun returns the last status of each workflow by its last run
// It only returns workflows with at least one run and skips workflows with initialized runs
func (uc *OrgMetricsUseCase) GetLastWorkflowStatusByRun(ctx context.Context, orgName string) ([]*prometheuscollector.WorkflowLastStatusByRunReport, error) {
	// Find organization
	org, err := uc.orgRepo.FindByName(ctx, orgName)
	if err != nil {
		return nil, fmt.Errorf("finding organization: %w", err)
	}

	// Check if organization exists, return empty list if not
	if org == nil {
		return nil, NewErrNotFound("organization")
	}

	// Default pagination option
	paginationOpts, err := pagination.NewOffsetPaginationOpts(pagination.DefaultPage, 100)
	if err != nil {
		return nil, fmt.Errorf("creating pagination options: %w", err)
	}

	var wfs []*Workflow
	// Request all workflows using the pagination until the returned workflows are less than the limit
	for {
		// List all workflows
		wfsPage, _, err := uc.wfUseCase.List(ctx, org.ID, nil, paginationOpts)
		if err != nil {
			return nil, fmt.Errorf("listing workflows: %w", err)
		}

		// Append workflows to the list
		wfs = append(wfs, wfsPage...)

		// Check if there are more workflows to fetch
		if len(wfsPage) < paginationOpts.Limit() {
			break
		}

		// Update pagination options with the next offset
		paginationOpts, err = pagination.NewOffsetPaginationOpts(paginationOpts.Offset()+paginationOpts.Limit(), paginationOpts.Limit())
		if err != nil {
			return nil, fmt.Errorf("creating pagination options: %w", err)
		}
	}

	// Create reports
	// nolint:prealloc
	var reports []*prometheuscollector.WorkflowLastStatusByRunReport
	for _, wf := range wfs {
		// Skip workflows with no runs
		if wf.RunsCounter == 0 {
			continue
		}

		// Skip workflows with initialized runs since they are not yet finished
		if wf.LastRun.State == string(WorkflowRunInitialized) {
			continue
		}

		reports = append(reports, &prometheuscollector.WorkflowLastStatusByRunReport{
			OrgName:      orgName,
			WorkflowName: wf.Name,
			Status:       wf.LastRun.State,
			Runner:       wf.LastRun.RunnerType,
		})
	}

	return reports, nil
}

// validateTimeWindowIsSet validates that the time window is set
func validateTimeWindowIsSet(tw *TimeWindow) error {
	// Check if time window is set
	if tw == nil {
		return fmt.Errorf("time window is required")
	}

	// Validate time window
	return tw.Validate()
}
