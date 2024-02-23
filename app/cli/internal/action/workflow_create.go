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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type WorkflowCreate struct {
	cfg *ActionsOpts
}

func NewWorkflowCreate(cfg *ActionsOpts) *WorkflowCreate {
	return &WorkflowCreate{cfg}
}

type NewWorkflowCreateOpts struct {
	Name, Description, Project, Team, ContractID string
}

func (action *WorkflowCreate) Run(opts *NewWorkflowCreateOpts) (*WorkflowItem, error) {
	client := pb.NewWorkflowServiceClient(action.cfg.CPConnection)
	resp, err := client.Create(context.Background(), &pb.WorkflowServiceCreateRequest{
		Name: opts.Name, Project: opts.Project, Team: opts.Team, SchemaId: opts.ContractID,
		Description: opts.Description,
	})
	if err != nil {
		return nil, err
	}

	return pbWorkflowItemToAction(resp.Result), nil
}
