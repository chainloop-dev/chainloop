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

type WorkflowUpdate struct {
	cfg *ActionsOpts
}

func NewWorkflowUpdate(cfg *ActionsOpts) *WorkflowUpdate {
	return &WorkflowUpdate{cfg}
}

type NewWorkflowUpdateOpts struct {
	Name, Project, Team *string
	Public              *bool
}

func (action *WorkflowUpdate) Run(ctx context.Context, id string, opts *NewWorkflowUpdateOpts) (*WorkflowItem, error) {
	client := pb.NewWorkflowServiceClient(action.cfg.CPConnection)
	resp, err := client.Update(ctx, &pb.WorkflowServiceUpdateRequest{
		Id:   id,
		Name: opts.Name, Project: opts.Project, Team: opts.Team,
		Public: opts.Public,
	})

	if err != nil {
		return nil, err
	}

	return pbWorkflowItemToAction(resp.Result), nil
}
