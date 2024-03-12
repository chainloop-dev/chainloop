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
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type WorkflowContractList struct {
	cfg *ActionsOpts
}

type WorkflowContractItem struct {
	Name           string     `json:"name"`
	Description    string     `json:"description,omitempty"`
	ID             string     `json:"id"`
	LatestRevision int        `json:"latestRevision,omitempty"`
	CreatedAt      *time.Time `json:"createdAt"`
	WorkflowIDs    []string   `json:"workflowIDs,omitempty"`
}

type WorkflowContractVersionItem struct {
	ID        string                   `json:"id"`
	Revision  int                      `json:"revision"`
	CreatedAt *time.Time               `json:"createdAt"`
	BodyV1    *schemav1.CraftingSchema `json:"bodyV1"`
}

func NewWorkflowContractList(cfg *ActionsOpts) *WorkflowContractList {
	return &WorkflowContractList{cfg}
}

func (action *WorkflowContractList) Run() ([]*WorkflowContractItem, error) {
	client := pb.NewWorkflowContractServiceClient(action.cfg.CPConnection)
	resp, err := client.List(context.Background(), &pb.WorkflowContractServiceListRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*WorkflowContractItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		result = append(result, pbWorkflowContractItemToAction(p))
	}

	return result, nil
}

func pbWorkflowContractItemToAction(in *pb.WorkflowContractItem) *WorkflowContractItem {
	return &WorkflowContractItem{
		Name: in.GetName(), ID: in.GetId(), LatestRevision: int(in.GetLatestRevision()),
		CreatedAt: toTimePtr(in.GetCreatedAt().AsTime()), WorkflowIDs: in.WorkflowIds, Description: in.GetDescription(),
	}
}

func pbWorkflowContractVersionItemToAction(in *pb.WorkflowContractVersionItem) *WorkflowContractVersionItem {
	return &WorkflowContractVersionItem{
		Revision: int(in.GetRevision()), ID: in.GetId(), BodyV1: in.GetV1(),
		CreatedAt: toTimePtr(in.GetCreatedAt().AsTime()),
	}
}
