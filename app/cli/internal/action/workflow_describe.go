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

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type WorkflowDescribe struct {
	cfg *ActionsOpts
}

func NewWorkflowDescribe(cfg *ActionsOpts) *WorkflowDescribe {
	return &WorkflowDescribe{cfg}
}

func (action *WorkflowDescribe) Run(ctx context.Context, workflowID, workflowName string) (*WorkflowItem, error) {
	client := pb.NewWorkflowServiceClient(action.cfg.CPConnection)

	req := &pb.WorkflowServiceViewRequest{Id: workflowID}
	if workflowName != "" {
		req.Name = workflowName
	}
	resp, err := client.View(ctx, req)
	if err != nil {
		return nil, err
	}

	return pbWorkflowItemToAction(resp.Result), nil
}
