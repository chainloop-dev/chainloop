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

	"github.com/chainloop-dev/chainloop/app/cli/cmd/options"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
)

type WorkflowContractCreate struct {
	cfg *options.ActionsOpts
}

func NewWorkflowContractCreate(cfg *options.ActionsOpts) *WorkflowContractCreate {
	return &WorkflowContractCreate{cfg}
}

func (action *WorkflowContractCreate) Run(name string, description *string, contractPath string, projectName string) (*WorkflowContractItem, error) {
	client := pb.NewWorkflowContractServiceClient(action.cfg.CPConnection)

	request := &pb.WorkflowContractServiceCreateRequest{
		Name: name, Description: description,
	}

	if projectName != "" {
		request.ProjectReference = &pb.IdentityReference{
			Name: &projectName,
		}
	}

	if contractPath != "" {
		rawContract, err := LoadFileOrURL(contractPath)
		if err != nil {
			action.cfg.Logger.Debug().Err(err).Msg("loading the contract")
			return nil, err
		}
		request.RawContract = rawContract
	}

	resp, err := client.Create(context.Background(), request)
	if err != nil {
		action.cfg.Logger.Debug().Err(err).Msg("making the API request")
		return nil, err
	}

	return pbWorkflowContractItemToAction(resp.Result), nil
}
