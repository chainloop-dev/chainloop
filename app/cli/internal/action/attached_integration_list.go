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
	"encoding/json"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type AttachedIntegrationList struct{ cfg *ActionsOpts }

func NewAttachedIntegrationList(cfg *ActionsOpts) *AttachedIntegrationList {
	return &AttachedIntegrationList{cfg}
}

func (action *AttachedIntegrationList) Run(projectName, workflowName string) ([]*AttachedIntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)

	resp, err := client.ListAttachments(context.Background(), &pb.ListAttachmentsRequest{
		ProjectName:  projectName,
		WorkflowName: workflowName,
	})
	if err != nil {
		return nil, err
	}

	result := make([]*AttachedIntegrationItem, 0, len(resp.Result))
	for _, i := range resp.Result {
		attachment, err := pbIntegrationAttachmentItemToAction(i)
		if err != nil {
			return nil, err
		}

		result = append(result, attachment)
	}

	return result, nil
}

func pbIntegrationAttachmentItemToAction(in *pb.IntegrationAttachmentItem) (*AttachedIntegrationItem, error) {
	integration, err := pbRegisteredIntegrationItemToAction(in.GetIntegration())
	if err != nil {
		return nil, err
	}

	i := &AttachedIntegrationItem{
		ID:          in.GetId(),
		CreatedAt:   toTimePtr(in.GetCreatedAt().AsTime()),
		Integration: integration,
		Workflow:    pbWorkflowItemToAction(in.GetWorkflow()),
	}

	// Old format does not include config so we skip it
	if in.Config == nil {
		return i, nil
	}

	if err = json.Unmarshal(in.Config, &i.Config); err != nil {
		// Can't process configuration
		return i, nil
	}

	return i, nil
}

type AttachedIntegrationItem struct {
	ID          string                     `json:"id"`
	CreatedAt   *time.Time                 `json:"createdAt"`
	Config      map[string]interface{}     `json:"config"`
	Integration *RegisteredIntegrationItem `json:"integration"`
	Workflow    *WorkflowItem              `json:"workflow"`
}
