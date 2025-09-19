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

package action

import (
	"context"
	"fmt"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type WorkflowList struct {
	cfg *ActionsOpts
}

type WorkflowItem struct {
	Name                   string           `json:"name"`
	Description            string           `json:"description,omitempty"`
	ID                     string           `json:"id"`
	Team                   string           `json:"team"`
	Project                string           `json:"project,omitempty"`
	CreatedAt              *time.Time       `json:"createdAt"`
	RunsCount              int32            `json:"runsCount"`
	ContractName           string           `json:"contractName,omitempty"`
	ContractRevisionLatest int32            `json:"contractRevisionLatest,omitempty"`
	LastRun                *WorkflowRunItem `json:"lastRun,omitempty"`
	// A public workflow means that any user can
	// - access to all its workflow runs
	// - their attestation and materials
	Public bool `json:"public"`
}

// WorkflowListResult holds the output of the workflow list action
type WorkflowListResult struct {
	Workflows  []*WorkflowItem   `json:"workflows"`
	Pagination *OffsetPagination `json:"pagination"`
}

// NewWorkflowList creates a new instance of WorkflowList
func NewWorkflowList(cfg *ActionsOpts) *WorkflowList {
	return &WorkflowList{cfg}
}

// Run executes the workflow list action
func (action *WorkflowList) Run(page int, pageSize int) (*WorkflowListResult, error) {
	if page < 1 {
		return nil, fmt.Errorf("page must be greater or equal to 1")
	}
	if pageSize < 1 {
		return nil, fmt.Errorf("page-size must be greater or equal to 1")
	}

	client := pb.NewWorkflowServiceClient(action.cfg.CPConnection)
	res := &WorkflowListResult{}

	resp, err := client.List(context.Background(), &pb.WorkflowServiceListRequest{
		Pagination: &pb.OffsetPaginationRequest{
			Page:     int32(page),
			PageSize: int32(pageSize),
		},
	})
	if err != nil {
		return nil, err
	}

	// Convert the response to the output format
	for _, p := range resp.Result {
		res.Workflows = append(res.Workflows, pbWorkflowItemToAction(p))
	}

	// Add the pagination details
	res.Pagination = &OffsetPagination{
		Page:       int(resp.GetPagination().GetPage()),
		PageSize:   int(resp.GetPagination().GetPageSize()),
		TotalPages: int(resp.GetPagination().GetTotalPages()),
		TotalCount: int(resp.GetPagination().GetTotalCount()),
	}

	return res, nil
}

// pbWorkflowItemToAction converts API response to WorkflowItem
func pbWorkflowItemToAction(wf *pb.WorkflowItem) *WorkflowItem {
	if wf == nil {
		return nil
	}

	return &WorkflowItem{
		Name:                   wf.Name,
		ID:                     wf.Id,
		CreatedAt:              toTimePtr(wf.CreatedAt.AsTime()),
		Project:                wf.Project,
		Team:                   wf.Team,
		RunsCount:              wf.RunsCount,
		ContractName:           wf.ContractName,
		ContractRevisionLatest: wf.ContractRevisionLatest,
		LastRun:                pbWorkflowRunItemToAction(wf.LastRun),
		Public:                 wf.Public,
		Description:            wf.Description,
	}
}

// NamespacedName returns the project and workflow name in a formatted string
func (wi *WorkflowItem) NamespacedName() string {
	return fmt.Sprintf("%s/%s", wi.Project, wi.Name)
}
