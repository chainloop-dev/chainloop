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

	prometheuscollector "github.com/chainloop-dev/chainloop/app/controlplane/pkg/metrics/prometheus"

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
	RunsTotal(ctx context.Context, orgID uuid.UUID, timeWindow *TimeWindow) (int32, error)
	// Total number by run status
	RunsByStatusTotal(ctx context.Context, orgID uuid.UUID, timeWindow *TimeWindow) (map[string]int32, error)
	RunsByRunnerTypeTotal(ctx context.Context, orgID uuid.UUID, timeWindow *TimeWindow) (map[string]int32, error)
	TopWorkflowsByRunsCount(ctx context.Context, orgID uuid.UUID, numWorkflows int, timeWindow *TimeWindow) ([]*TopWorkflowsByRunsCountItem, error)
	DailyRunsCount(ctx context.Context, orgID, workflowID uuid.UUID, timeWindow *TimeWindow) ([]*DayRunsCount, error)
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

func (uc *OrgMetricsUseCase) RunsTotal(ctx context.Context, orgID string, timeWindow *TimeWindow) (int32, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return 0, err
	}

	if err := validateTimeWindowIsSet(timeWindow); err != nil {
		return 0, err
	}

	return uc.repo.RunsTotal(ctx, orgUUID, timeWindow)
}

func (uc *OrgMetricsUseCase) RunsTotalByStatus(ctx context.Context, orgID string, timeWindow *TimeWindow) (map[string]int32, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	if err := validateTimeWindowIsSet(timeWindow); err != nil {
		return nil, err
	}

	return uc.repo.RunsByStatusTotal(ctx, orgUUID, timeWindow)
}

func (uc *OrgMetricsUseCase) RunsTotalByRunnerType(ctx context.Context, orgID string, timeWindow *TimeWindow) (map[string]int32, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	if err := validateTimeWindowIsSet(timeWindow); err != nil {
		return nil, err
	}

	return uc.repo.RunsByRunnerTypeTotal(ctx, orgUUID, timeWindow)
}

// DailyRunsCount returns the number of runs per day within the provided time window (from now)
// Optionally filtered by workflowID
func (uc *OrgMetricsUseCase) DailyRunsCount(ctx context.Context, orgID string, workflowID *string, timeWindow *TimeWindow) ([]*DayRunsCount, error) {
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

	return uc.repo.DailyRunsCount(ctx, orgUUID, workflowUUID, timeWindow)
}

type TopWorkflowsByRunsCountItem struct {
	Workflow *Workflow
	ByStatus map[string]int32
	Total    int32
}

func (uc *OrgMetricsUseCase) TopWorkflowsByRunsCount(ctx context.Context, orgID string, numWorkflows int, timeWindow *TimeWindow) ([]*TopWorkflowsByRunsCountItem, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	if err := validateTimeWindowIsSet(timeWindow); err != nil {
		return nil, err
	}

	return uc.repo.TopWorkflowsByRunsCount(ctx, orgUUID, numWorkflows, timeWindow)
}

// GetLastWorkflowStatusByRun returns the last status of each workflow by its last run
func (uc *OrgMetricsUseCase) GetLastWorkflowStatusByRun(orgName string) ([]*prometheuscollector.WorkflowLastStatusByRunReport, error) {
	ctx := context.Background()
	// Find organization
	org, err := uc.orgRepo.FindByName(ctx, orgName)
	if err != nil {
		return nil, fmt.Errorf("finding organization: %w", err)
	}

	// List all workflows
	wfs, err := uc.wfUseCase.List(ctx, org.ID)
	if err != nil {
		return nil, fmt.Errorf("listing workflows: %w", err)
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
