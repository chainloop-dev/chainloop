//
// Copyright 2025 The Chainloop Authors.
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
)

type WorkflowContractApply struct {
	cfg *ActionsOpts
}

func NewWorkflowContractApply(cfg *ActionsOpts) *WorkflowContractApply {
	return &WorkflowContractApply{cfg}
}

func (action *WorkflowContractApply) Run(ctx context.Context, rawContract []byte, contractName string, description *string, projectName string) (*WorkflowContractItem, error) {
	client := pb.NewWorkflowContractServiceClient(action.cfg.CPConnection)

	// Try to describe the specific contract first to determine if we should create or update
	describeReq := &pb.WorkflowContractServiceDescribeRequest{
		Name: contractName,
	}

	_, err := client.Describe(ctx, describeReq)
	if err == nil {
		// Contract exists, perform update
		updateReq := &pb.WorkflowContractServiceUpdateRequest{
			Name:        contractName,
			RawContract: rawContract,
		}

		if description != nil {
			updateReq.Description = description
		}

		res, err := client.Update(ctx, updateReq)
		if err != nil {
			return nil, fmt.Errorf("failed to update existing contract '%s': %w", contractName, err)
		}

		return pbWorkflowContractItemToAction(res.Result.Contract), nil
	}

	// Contract doesn't exist, perform create
	createReq := &pb.WorkflowContractServiceCreateRequest{
		Name:        contractName,
		RawContract: rawContract,
	}

	if description != nil {
		createReq.Description = description
	}

	if projectName != "" {
		createReq.ProjectReference = &pb.IdentityReference{
			Name: &projectName,
		}
	}

	res, err := client.Create(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create new contract '%s': %w", contractName, err)
	}

	return pbWorkflowContractItemToAction(res.Result), nil
}
