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
	cxpb "github.com/chainloop-dev/chainloop/app/controlplane/integrations/gen/dependencytrack/cyclonedx/v1"
	"google.golang.org/protobuf/types/known/anypb"
)

// Attach a third party integration to a workflow
type WorkflowIntegrationAttach struct{ cfg *ActionsOpts }

func NewWorkflowIntegrationAttach(cfg *ActionsOpts) *WorkflowIntegrationAttach {
	return &WorkflowIntegrationAttach{cfg}
}

func (action *WorkflowIntegrationAttach) RunDependencyTrack(integrationID, workflowID, projectID, projectName string) (*IntegrationAttachmentItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)

	var projectConfig *cxpb.AttachmentConfig
	if projectID != "" {
		projectConfig = &cxpb.AttachmentConfig{
			Project: &cxpb.AttachmentConfig_ProjectId{ProjectId: projectID},
		}
	} else if projectName != "" {
		projectConfig = &cxpb.AttachmentConfig{
			Project: &cxpb.AttachmentConfig_ProjectName{ProjectName: projectName},
		}
	}

	anyConfig, err := anypb.New(&cxpb.AttachmentRequest{Config: projectConfig})
	if err != nil {
		return nil, err
	}

	resp, err := client.Attach(context.Background(), &pb.IntegrationsServiceAttachRequest{
		WorkflowId: workflowID, IntegrationId: integrationID, AttachmentConfig: anyConfig,
	})

	if err != nil {
		return nil, err
	}

	return pbIntegrationAttachmentItemToAction(resp.Result), nil
}
