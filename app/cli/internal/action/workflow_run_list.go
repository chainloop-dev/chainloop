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

package action

import (
	"context"
	"slices"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

// WorkflowRunStatus represents the status of a workflow run
var WorkflowRunStatus = []string{"INITIALIZED", "SUCCEEDED", "FAILED", "EXPIRED", "CANCELLED"}

type WorkflowRunList struct {
	cfg *ActionsOpts
}

type WorkflowRunItem struct {
	ID                     string                       `json:"id"`
	State                  string                       `json:"state"`
	Reason                 string                       `json:"reason,omitempty"`
	CreatedAt              *time.Time                   `json:"createdAt,omitempty"`
	FinishedAt             *time.Time                   `json:"finishedAt,omitempty"`
	Workflow               *WorkflowItem                `json:"workflow,omitempty"`
	RunURL                 string                       `json:"runURL,omitempty"`
	RunnerType             string                       `json:"runnerType,omitempty"`
	ContractVersion        *WorkflowContractVersionItem `json:"contractVersion,omitempty"`
	ContractRevisionUsed   int                          `json:"contractRevisionUsed"`
	ContractRevisionLatest int                          `json:"contractRevisionLatest"`
}

type PaginatedWorkflowRunItem struct {
	Result         []*WorkflowRunItem
	PaginationMeta *PaginationOpts
}

func NewWorkflowRunList(cfg *ActionsOpts) *WorkflowRunList {
	return &WorkflowRunList{cfg}
}

type WorkflowRunListOpts struct {
	WorkflowID string
	Pagination *PaginationOpts
	Status     string
}
type PaginationOpts struct {
	Limit      int
	NextCursor string
}

func (action *WorkflowRunList) Run(opts *WorkflowRunListOpts) (*PaginatedWorkflowRunItem, error) {
	client := pb.NewWorkflowRunServiceClient(action.cfg.CPConnection)
	req := &pb.WorkflowRunServiceListRequest{
		WorkflowId: opts.WorkflowID,
		Pagination: &pb.CursorPaginationRequest{
			Limit:  int32(opts.Pagination.Limit),
			Cursor: opts.Pagination.NextCursor,
		},
	}

	if opts.Status != "" || slices.Contains(WorkflowRunStatus, opts.Status) {
		req.Status = transformWorkflowRunStatus(opts.Status)
	}

	resp, err := client.List(context.Background(), req)
	if err != nil {
		return nil, err
	}

	result := make([]*WorkflowRunItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbWorkflowRunItemToAction(p))
	}

	res := &PaginatedWorkflowRunItem{
		Result: result,
		PaginationMeta: &PaginationOpts{
			NextCursor: resp.GetPagination().GetNextCursor(),
		},
	}

	return res, nil
}

// pbWorkflowRunItemToAction converts a pb.WorkflowRunItem to a pb.RunStatus item
func transformWorkflowRunStatus(status string) pb.RunStatus {
	switch status {
	case "INITIALIZED":
		return pb.RunStatus_RUN_STATUS_INITIALIZED
	case "SUCCEEDED":
		return pb.RunStatus_RUN_STATUS_SUCCEEDED
	case "FAILED":
		return pb.RunStatus_RUN_STATUS_FAILED
	case "EXPIRED":
		return pb.RunStatus_RUN_STATUS_EXPIRED
	case "CANCELLED":
		return pb.RunStatus_RUN_STATUS_CANCELLED
	default:
		return pb.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}

func pbWorkflowRunItemToAction(in *pb.WorkflowRunItem) *WorkflowRunItem {
	if in == nil {
		return nil
	}

	item := &WorkflowRunItem{
		ID: in.Id, State: in.State, Reason: in.Reason, CreatedAt: toTimePtr(in.CreatedAt.AsTime()),
		Workflow:               pbWorkflowItemToAction(in.Workflow),
		RunURL:                 in.GetJobUrl(),
		RunnerType:             humanizedRunnerType(in.GetRunnerType()),
		ContractRevisionUsed:   int(in.GetContractRevisionUsed()),
		ContractRevisionLatest: int(in.GetContractRevisionLatest()),
	}

	if in.GetContractVersion() != nil {
		item.ContractVersion = pbWorkflowContractVersionItemToAction(in.GetContractVersion())
	}

	if in.FinishedAt != nil {
		item.FinishedAt = toTimePtr(in.FinishedAt.AsTime())
	}

	return item
}

func humanizedRunnerType(in v1.CraftingSchema_Runner_RunnerType) string {
	mapping := map[v1.CraftingSchema_Runner_RunnerType]string{
		*v1.CraftingSchema_Runner_RUNNER_TYPE_UNSPECIFIED.Enum(): "Unspecified",
		*v1.CraftingSchema_Runner_GITHUB_ACTION.Enum():           "GitHub",
		*v1.CraftingSchema_Runner_GITLAB_PIPELINE.Enum():         "GitLab",
		*v1.CraftingSchema_Runner_AZURE_PIPELINE.Enum():          "Azure Pipeline",
		*v1.CraftingSchema_Runner_JENKINS_JOB.Enum():             "Jenkins Job",
		*v1.CraftingSchema_Runner_CIRCLECI_BUILD.Enum():          "CircleCI Build",
		*v1.CraftingSchema_Runner_DAGGER_PIPELINE.Enum():         "Dagger Pipeline",
	}

	hrt, ok := mapping[in]
	if !ok {
		return "Unknown"
	}

	return hrt
}
