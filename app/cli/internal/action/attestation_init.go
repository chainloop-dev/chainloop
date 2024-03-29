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
	"errors"
	"fmt"
	"strconv"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	clientAPI "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
)

type AttestationInitOpts struct {
	*ActionsOpts
	DryRun bool
	// Force the initialization and override any existing, in-progress ones.
	// Note that this is only useful when local-based attestation state is configured
	// since it's a protection to make sure you don't override the state by mistake
	Force bool
}

type AttestationInit struct {
	*ActionsOpts
	dryRun, force bool
	c             *crafter.Crafter
}

// ErrAttestationAlreadyExist means that there is an attestation in progress
var ErrAttestationAlreadyExist = errors.New("attestation already initialized")

type ErrRunnerContextNotFound struct {
	RunnerType string
}

func (e ErrRunnerContextNotFound) Error() string {
	return fmt.Sprintf("The contract expects the attestation to be crafted in a runner of type %q but couldn't be detected", e.RunnerType)
}

func NewAttestationInit(cfg *AttestationInitOpts) (*AttestationInit, error) {
	c, err := newCrafter(cfg.UseAttestationRemoteState, cfg.CPConnection, crafter.WithLogger(&cfg.Logger))
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	return &AttestationInit{
		ActionsOpts: cfg.ActionsOpts,
		c:           c,
		dryRun:      cfg.DryRun,
		force:       cfg.Force,
	}, nil
}

// returns the attestation ID
func (action *AttestationInit) Run(ctx context.Context, contractRevision int) (string, error) {
	if action.dryRun && action.UseAttestationRemoteState {
		return "", errors.New("remote state is not compatible with dry-run mode")
	}

	// During local initializations we need to make sure if there is already an attestation in progress
	// If it is and we are not "forcing" the initialization, we should return an error
	if !action.UseAttestationRemoteState && !action.force {
		if initialized, _ := action.c.AlreadyInitialized(ctx, ""); initialized {
			return "", ErrAttestationAlreadyExist
		}
	}

	action.Logger.Debug().Msg("Retrieving attestation definition")
	client := pb.NewAttestationServiceClient(action.ActionsOpts.CPConnection)
	// get information of the workflow
	contractResp, err := client.GetContract(ctx, &pb.AttestationServiceGetContractRequest{ContractRevision: int32(contractRevision)})
	if err != nil {
		return "", err
	}

	workflow := contractResp.GetResult().GetWorkflow()
	contractVersion := contractResp.Result.GetContract()
	contract := contractResp.GetResult().GetContract().GetV1()

	workflowMeta := &clientAPI.WorkflowMetadata{
		WorkflowId:     workflow.GetId(),
		Name:           workflow.GetName(),
		Project:        workflow.GetProject(),
		Team:           workflow.GetTeam(),
		SchemaRevision: strconv.Itoa(int(contractVersion.GetRevision())),
	}

	action.Logger.Debug().Msg("workflow contract and metadata retrieved from the control plane")

	// Auto discover the runner context and enforce against the one in the contract if needed
	discoveredRunner, err := crafter.DiscoverAndEnforceRunner(contract.GetRunner().GetType(), action.dryRun, action.Logger)
	if err != nil {
		return "", ErrRunnerContextNotFound{err.Error()}
	}

	// Identifier of this attestation instance
	var attestationID string

	// Init in the control plane if needed including the runner context
	if !action.dryRun {
		runResp, err := client.Init(
			ctx,
			&pb.AttestationServiceInitRequest{
				Runner:           discoveredRunner.ID(),
				JobUrl:           discoveredRunner.RunURI(),
				ContractRevision: int32(contractRevision),
			},
		)
		if err != nil {
			return "", err
		}

		workflowRun := runResp.GetResult().GetWorkflowRun()
		workflowMeta.WorkflowRunId = workflowRun.GetId()
		workflowMeta.Organization = runResp.GetResult().GetOrganization()
		action.Logger.Debug().Str("workflow-run-id", workflowRun.GetId()).Msg("attestation initialized in the control plane")
		attestationID = workflowRun.GetId()
	}

	// Initialize the local attestation crafter
	// NOTE: important to run this initialization here since workflowMeta is populated
	// with the workflowRunId that comes from the control plane
	initOpts := &crafter.InitOpts{
		WfInfo: workflowMeta, SchemaV1: contractVersion.GetV1(),
		DryRun:        action.dryRun,
		AttestationID: attestationID,
		Runner:        discoveredRunner,
	}

	if err := action.c.Init(ctx, initOpts); err != nil {
		return "", err
	}

	// Load the env variables both the system populated and the user predefined ones
	if err := action.c.ResolveEnvVars(ctx, attestationID); err != nil {
		if action.dryRun {
			return "", nil
		}

		_ = action.c.Reset(ctx, attestationID)
		return "", err
	}

	return attestationID, nil
}
