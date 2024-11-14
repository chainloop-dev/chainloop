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
	"fmt"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// Attach a third party integration to a workflow
type AttachedIntegrationAdd struct{ cfg *ActionsOpts }

func NewAttachedIntegrationAdd(cfg *ActionsOpts) *AttachedIntegrationAdd {
	return &AttachedIntegrationAdd{cfg}
}

func (action *AttachedIntegrationAdd) Run(integrationName, workflowName, projectName string, options map[string]any) (*AttachedIntegrationItem, error) {
	client := pb.NewIntegrationsServiceClient(action.cfg.CPConnection)

	requestConfig, err := structpb.NewStruct(options)
	if err != nil {
		return nil, fmt.Errorf("failed to parse arguments: %w", err)
	}

	resp, err := client.Attach(context.Background(), &pb.IntegrationsServiceAttachRequest{
		WorkflowName:    workflowName,
		ProjectName:     projectName,
		IntegrationName: integrationName,
		Config:          requestConfig,
	})
	if err != nil {
		return nil, err
	}

	return pbIntegrationAttachmentItemToAction(resp.Result)
}
