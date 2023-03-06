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

	pb "github.com/chainloop-dev/bedrock/app/controlplane/api/controlplane/v1"
)

type WorkflowContractDescribe struct {
	cfg *ActionsOpts
}

type WorkflowContractWithVersionItem struct {
	Contract *WorkflowContractItem        `json:"contract"`
	Revision *WorkflowContractVersionItem `json:"revision"`
}

func NewWorkflowContractDescribe(cfg *ActionsOpts) *WorkflowContractDescribe {
	return &WorkflowContractDescribe{cfg}
}

func (action *WorkflowContractDescribe) Run(id string, rev int32) (*WorkflowContractWithVersionItem, error) {
	client := pb.NewWorkflowContractServiceClient(action.cfg.CPConnecction)

	request := &pb.WorkflowContractServiceDescribeRequest{Id: id, Revision: rev}

	resp, err := client.Describe(context.Background(), request)
	if err != nil {
		action.cfg.Logger.Debug().Err(err).Msg("making the API request")
		return nil, err
	}

	return &WorkflowContractWithVersionItem{
		Contract: pbWorkflowContractItemToAction(resp.GetResult().GetContract()),
		Revision: pbWorkflowContractVersionItemToAction(resp.GetResult().GetRevision()),
	}, nil
}
