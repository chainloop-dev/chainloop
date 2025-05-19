//
// Copyright 2023-2025 The Chainloop Authors.
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
	Name                    string         `json:"name"`
	Description             string         `json:"description,omitempty"`
	ID                      string         `json:"id"`
	LatestRevision          int            `json:"latestRevision,omitempty"`
	LatestRevisionCreatedAt *time.Time     `json:"latestRevisionCreatedAt,omitempty"`
	CreatedAt               *time.Time     `json:"createdAt"`
	Workflows               []string       `json:"workflows,omitempty"` // TODO: remove this field after all clients are updated
	WorkflowRefs            []*WorkflowRef `json:"workflowRefs,omitempty"`
}

type WorkflowRef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectName string `json:"projectName"`
}

type WorkflowContractVersionItem struct {
	ID        string                   `json:"id"`
	Revision  int                      `json:"revision"`
	CreatedAt *time.Time               `json:"createdAt"`
	BodyV1    *schemav1.CraftingSchema `json:"bodyV1"`
	RawBody   *ContractRawBody         `json:"rawBody"`
}

type ContractRawBody struct {
	Body   string `json:"body"`
	Format string `json:"format"`
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
	// nolint:prealloc
	var workflowRefs []*WorkflowRef
	for _, w := range in.WorkflowRefs {
		workflowRefs = append(workflowRefs, pbWorkflowRefToAction(w))
	}
	return &WorkflowContractItem{
		Name:                    in.GetName(),
		ID:                      in.GetId(),
		LatestRevision:          int(in.GetLatestRevision()),
		CreatedAt:               toTimePtr(in.GetCreatedAt().AsTime()),
		Workflows:               in.WorkflowNames, // nolint:staticcheck
		WorkflowRefs:            workflowRefs,
		Description:             in.GetDescription(),
		LatestRevisionCreatedAt: toTimePtr(in.GetLatestRevisionCreatedAt().AsTime()),
	}
}

func pbWorkflowContractVersionItemToAction(in *pb.WorkflowContractVersionItem) *WorkflowContractVersionItem {
	return &WorkflowContractVersionItem{
		Revision: int(in.GetRevision()), ID: in.GetId(), BodyV1: in.GetV1(),
		CreatedAt: toTimePtr(in.GetCreatedAt().AsTime()),
		RawBody:   &ContractRawBody{Body: string(in.RawContract.GetBody()), Format: in.RawContract.GetFormat().String()},
	}
}

func pbWorkflowRefToAction(in *pb.WorkflowRef) *WorkflowRef {
	return &WorkflowRef{ID: in.GetId(), Name: in.GetName(), ProjectName: in.GetProjectName()}
}
