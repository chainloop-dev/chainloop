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

// Attach a third party integration to a workflow
type WorkflowIntegrationAttach struct{ cfg *ActionsOpts }

func NewWorkflowIntegrationAttach(cfg *ActionsOpts) *WorkflowIntegrationAttach {
	return &WorkflowIntegrationAttach{cfg}
}

func (action *WorkflowIntegrationAttach) RunDependencyTrack(integrationID, workflowID, projectID, projectName string) (*IntegrationAttachmentItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)

	var projectConfig *pb.IntegrationAttachmentConfig_DependencyTrack
	if projectID != "" {
		projectConfig = &pb.IntegrationAttachmentConfig_DependencyTrack{
			Project: &pb.IntegrationAttachmentConfig_DependencyTrack_ProjectId{ProjectId: projectID},
		}
	} else if projectName != "" {
		projectConfig = &pb.IntegrationAttachmentConfig_DependencyTrack{
			Project: &pb.IntegrationAttachmentConfig_DependencyTrack_ProjectName{ProjectName: projectName},
		}
	}

	resp, err := client.Attach(context.Background(), &pb.IntegrationsServiceAttachRequest{
		WorkflowId: workflowID, IntegrationId: integrationID,
		Config: &pb.IntegrationAttachmentConfig{
			Config: &pb.IntegrationAttachmentConfig_DependencyTrack_{DependencyTrack: projectConfig},
		},
	})
	if err != nil {
		return nil, err
	}

	return pbIntegrationAttachmentItemToAction(resp.Result), nil
}
