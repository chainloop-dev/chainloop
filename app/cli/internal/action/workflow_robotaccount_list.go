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

	pb "github.com/chainloop-dev/bedrock/app/controlplane/api/controlplane/v1"
)

type WorkflowRobotAccountList struct {
	cfg *ActionsOpts
}

type WorkflowRobotAccountItem struct {
	Name       string `json:"name"`
	ID         string `json:"id"`
	WorkflowID string `json:"workflowID"`
	// Key is returned only during the creation
	Key       string     `json:"key,omitempty"`
	CreatedAt *time.Time `json:"createdAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

func NewWorkflowRobotAccountList(cfg *ActionsOpts) *WorkflowRobotAccountList {
	return &WorkflowRobotAccountList{cfg}
}

func (action *WorkflowRobotAccountList) Run(workflowID string, includeRevoked bool) ([]*WorkflowRobotAccountItem, error) {
	client := pb.NewRobotAccountServiceClient(action.cfg.CPConnecction)
	resp, err := client.List(context.Background(), &pb.RobotAccountServiceListRequest{WorkflowId: workflowID, IncludeRevoked: includeRevoked})
	if err != nil {
		return nil, err
	}

	result := make([]*WorkflowRobotAccountItem, 0, len(resp.Result))
	for _, p := range resp.Result {
		item := &WorkflowRobotAccountItem{
			Name: p.Name, ID: p.Id, WorkflowID: p.WorkflowId,
			CreatedAt: toTimePtr(p.CreatedAt.AsTime()),
		}
		if p.RevokedAt != nil {
			item.RevokedAt = toTimePtr(p.RevokedAt.AsTime())
		}

		result = append(result, item)
	}

	return result, nil
}
