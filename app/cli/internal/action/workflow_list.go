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
	ContractID             string           `json:"contractID,omitempty"`
	ContractRevisionLatest int32            `json:"contractRevisionLatest,omitempty"`
	LastRun                *WorkflowRunItem `json:"lastRun,omitempty"`
	// A public workflow means that any user can
	// - access to all its workflow runs
	// - their attestation and materials
	Public bool `json:"public"`
}

func NewWorkflowList(cfg *ActionsOpts) *WorkflowList {
	return &WorkflowList{cfg}
}

func (action *WorkflowList) Run() ([]*WorkflowItem, error) {
	client := pb.NewWorkflowServiceClient(action.cfg.CPConnection)
	resp, err := client.List(context.Background(), &pb.WorkflowServiceListRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*WorkflowItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbWorkflowItemToAction(p))
	}

	return result, nil
}

func pbWorkflowItemToAction(wf *pb.WorkflowItem) *WorkflowItem {
	if wf == nil {
		return nil
	}

	res := &WorkflowItem{
		Name: wf.Name, ID: wf.Id, CreatedAt: toTimePtr(wf.CreatedAt.AsTime()),
		Project: wf.Project, Team: wf.Team, RunsCount: wf.RunsCount,
		ContractID:             wf.ContractId,
		ContractRevisionLatest: wf.ContractRevisionLatest,
		LastRun:                pbWorkflowRunItemToAction(wf.LastRun),
		Public:                 wf.Public,
		Description:            wf.Description,
	}

	return res
}

func (wi *WorkflowItem) NamespacedName() string {
	return fmt.Sprintf("%s/%s", wi.Project, wi.Name)
}
