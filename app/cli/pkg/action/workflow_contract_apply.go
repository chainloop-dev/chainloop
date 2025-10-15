//
// Copyright 2024-2025 The Chainloop Authors.
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
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
)

type WorkflowContractApply struct {
	cfg *ActionsOpts
}

func NewWorkflowContractApply(cfg *ActionsOpts) *WorkflowContractApply {
	return &WorkflowContractApply{cfg}
}

func (action *WorkflowContractApply) Run(filePath, name string, description *string, projectName string) (*WorkflowContractItem, error) {
	var rawContract []byte
	var err error
	contractName := name

	// Load contract from file if provided
	if filePath != "" {
		rawContract, err = LoadFileOrURL(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read contract file: %w", err)
		}

		// Extract name from the contract file content
		extractedName, err := extractContractNameFromRawSchema(rawContract)
		if err != nil {
			return nil, err
		}

		// For v2 schemas, use the extracted name. For v1 schemas, extractedName will be empty
		if extractedName == "" && name == "" {
			return nil, fmt.Errorf("contracts require --name flag to specify the contract name")
		} else if extractedName != "" {
			contractName = extractedName
		}
	}

	client := pb.NewWorkflowContractServiceClient(action.cfg.CPConnection)

	// Try to describe the specific contract first to determine if we should create or update
	describeReq := &pb.WorkflowContractServiceDescribeRequest{
		Name: contractName,
	}

	_, err = client.Describe(context.Background(), describeReq)
	if err == nil {
		// Contract exists, perform update
		updateReq := &pb.WorkflowContractServiceUpdateRequest{
			Name:        contractName,
			RawContract: rawContract,
		}

		if description != nil {
			updateReq.Description = description
		}

		res, err := client.Update(context.Background(), updateReq)
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

	res, err := client.Create(context.Background(), createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create new contract '%s': %w", contractName, err)
	}

	return pbWorkflowContractItemToAction(res.Result), nil
}

// extractContractNameFromContent tries to extract the contract name from the contract file content
func extractContractNameFromRawSchema(content []byte) (string, error) {
	// Identify format first
	format, err := unmarshal.IdentifyFormat(content)
	if err != nil {
		return "", fmt.Errorf("failed to identify contract format: %w", err)
	}

	// Unmarshal as v2 contract
	// If it fails, we assume it's a v1 contract which doesn't have a name in the content
	v2Contract := &schemaapi.CraftingSchemaV2{}
	if err := unmarshal.FromRaw(content, format, v2Contract, false); err == nil {
		// Successfully parsed as v2, extract name from metadata
		if v2Contract.Metadata != nil && v2Contract.Metadata.Name != "" {
			return v2Contract.Metadata.Name, nil
		}
		return "", fmt.Errorf("missing name in metadata section of the contract")
	}
	return "", nil
}
