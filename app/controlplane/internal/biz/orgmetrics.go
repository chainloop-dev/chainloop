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
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type OrgMetricsUseCase struct {
	logger *log.Helper
	repo   OrgMetricsRepo
}

type OrgMetricsRepo interface {
	// Total number of runs within the provided time window (from now)
	RunsTotal(ctx context.Context, orgID uuid.UUID, timeWindow time.Duration) (int32, error)
	// Total number by run status
	RunsByStatusTotal(ctx context.Context, orgID uuid.UUID, timeWindow time.Duration) (map[string]int32, error)
	RunsByRunnerTypeTotal(ctx context.Context, orgID uuid.UUID, timeWindow time.Duration) (map[string]int32, error)
	TopWorkflowsByRunsCount(ctx context.Context, orgID uuid.UUID, numWorkflows int, timeWindow time.Duration) ([]*TopWorkflowsByRunsCountItem, error)
}

func NewOrgMetricsUseCase(r OrgMetricsRepo, l log.Logger) (*OrgMetricsUseCase, error) {
	return &OrgMetricsUseCase{logger: log.NewHelper(l), repo: r}, nil
}

func (uc *OrgMetricsUseCase) RunsTotal(ctx context.Context, orgID string, timeWindow time.Duration) (int32, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return 0, err
	}

	return uc.repo.RunsTotal(ctx, orgUUID, timeWindow)
}

func (uc *OrgMetricsUseCase) RunsTotalByStatus(ctx context.Context, orgID string, timeWindow time.Duration) (map[string]int32, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	return uc.repo.RunsByStatusTotal(ctx, orgUUID, timeWindow)
}

func (uc *OrgMetricsUseCase) RunsTotalByRunnerType(ctx context.Context, orgID string, timeWindow time.Duration) (map[string]int32, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	return uc.repo.RunsByRunnerTypeTotal(ctx, orgUUID, timeWindow)
}

type TopWorkflowsByRunsCountItem struct {
	Workflow *Workflow
	ByStatus map[string]int32
	Total    int32
}

func (uc *OrgMetricsUseCase) TopWorkflowsByRunsCount(ctx context.Context, orgID string, numWorkflows int, timeWindow time.Duration) ([]*TopWorkflowsByRunsCountItem, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, err
	}

	return uc.repo.TopWorkflowsByRunsCount(ctx, orgUUID, numWorkflows, timeWindow)
}
