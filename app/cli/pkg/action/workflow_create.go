//
// Copyright 2024-2026 The Chainloop Authors.
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
	Name, Description, Project, Team, ContractName string
	ContractBytes                                  []byte
	// WorkflowTemplateID optionally binds the workflow to a platform workflow template.
	// The open-source CLI does not set it, it exists so template-aware clients can.
	WorkflowTemplateID string
}

func (action *WorkflowCreate) Run(opts *NewWorkflowCreateOpts) (*WorkflowItem, error) {
	client := pb.NewWorkflowServiceClient(action.cfg.CPConnection)
	resp, err := client.Create(context.Background(), newWorkflowCreateRequest(opts))
	if err != nil {
		return nil, err
	}

	return pbWorkflowItemToAction(resp.Result), nil
}

// newWorkflowCreateRequest maps the create options to the API request
func newWorkflowCreateRequest(opts *NewWorkflowCreateOpts) *pb.WorkflowServiceCreateRequest {
	return &pb.WorkflowServiceCreateRequest{
		Name: opts.Name, ProjectName: opts.Project, Team: opts.Team, ContractName: opts.ContractName,
		Description:        opts.Description,
		ContractBytes:      opts.ContractBytes,
		WorkflowTemplateId: opts.WorkflowTemplateID,
	}
}
