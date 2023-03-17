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
)

type WorkflowIntegrationList struct{ cfg *ActionsOpts }

func NewWorkflowIntegrationList(cfg *ActionsOpts) *WorkflowIntegrationList {
	return &WorkflowIntegrationList{cfg}
}

func (action *WorkflowIntegrationList) Run() ([]*IntegrationAttachmentItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)

	resp, err := client.ListAttachments(context.Background(), &pb.ListAttachmentsRequest{})
	if err != nil {
		return nil, err
	}

	result := make([]*IntegrationAttachmentItem, 0, len(resp.Result))
	for _, i := range resp.Result {
		result = append(result, pbIntegrationAttachmentItemToAction(i))
	}

	return result, nil
}

func pbIntegrationAttachmentItemToAction(in *pb.IntegrationAttachmentItem) *IntegrationAttachmentItem {
	i := &IntegrationAttachmentItem{
		ID:          in.GetId(),
		CreatedAt:   toTimePtr(in.GetCreatedAt().AsTime()),
		Integration: pbIntegrationItemToAction(in.GetIntegration()),
		Workflow:    pbWorkflowItemToAction(in.GetWorkflow()),
	}

	if c := in.GetConfig().GetDependencyTrack(); c != nil {
		i.Config = map[string]interface{}{
			"projectID":   c.GetProjectId(),
			"projectName": c.GetProjectName(),
		}
	}

	return i
}

type IntegrationAttachmentItem struct {
	ID          string                 `json:"id"`
	CreatedAt   *time.Time             `json:"createdAt"`
	Config      map[string]interface{} `json:"config"`
	Integration *IntegrationItem       `json:"integration"`
	Workflow    *WorkflowItem          `json:"workflow"`
}
