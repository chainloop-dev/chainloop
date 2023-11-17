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

	clientAPI "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
)

type AttestationInitOpts struct {
	*ActionsOpts
	Override, DryRun bool
}

type AttestationInit struct {
	*ActionsOpts
	override, dryRun bool
	c                *crafter.Crafter
}

// ErrAttestationAlreadyExist means that there is an attestation in progress
var ErrAttestationAlreadyExist = errors.New("attestation already initialized")

type ErrRunnerContextNotFound struct {
	RunnerType string
}

func (e ErrRunnerContextNotFound) Error() string {
	return fmt.Sprintf("The contract expects the attestation to be crafted in a runner of type %q but couldn't be detected", e.RunnerType)
}

func NewAttestationInit(cfg *AttestationInitOpts) *AttestationInit {
	return &AttestationInit{
		ActionsOpts: cfg.ActionsOpts,
		override:    cfg.Override,
		c:           crafter.NewCrafter(crafter.WithLogger(&cfg.Logger)),
		dryRun:      cfg.DryRun,
	}
}

func (action *AttestationInit) Run(contractRevision int) error {
	if initialized := action.c.AlreadyInitialized(); initialized && !action.override {
		return ErrAttestationAlreadyExist
	}

	action.Logger.Debug().Msg("Retrieving attestation definition")
	client := pb.NewAttestationServiceClient(action.ActionsOpts.CPConnection)
	// get information of the workflow
	ctx := context.Background()
	resp, err := client.GetContract(ctx, &pb.AttestationServiceGetContractRequest{ContractRevision: int32(contractRevision)})
	if err != nil {
		return err
	}

	workflow := resp.GetResult().GetWorkflow()
	contractVersion := resp.Result.GetContract()

	workflowMeta := &clientAPI.WorkflowMetadata{
		WorkflowId:     workflow.GetId(),
		Name:           workflow.GetName(),
		Project:        workflow.GetProject(),
		Team:           workflow.GetTeam(),
		SchemaRevision: strconv.Itoa(int(contractVersion.GetRevision())),
	}

	action.Logger.Debug().Msg("workflow contract and metadata retrieved from the control plane")

	// Check that the instantiation is happening in the right runner context
	// and extract the job URL
	runnerType := resp.Result.Contract.GetV1().Runner.GetType()
	runnerContext := crafter.NewRunner(runnerType)
	if !action.dryRun && !runnerContext.CheckEnv() {
		return ErrRunnerContextNotFound{runnerContext.String()}
	}

	// Init in the control plane if needed including the runner context
	if !action.dryRun {
		runResp, err := client.Init(
			ctx,
			&pb.AttestationServiceInitRequest{
				JobUrl:           runnerContext.RunURI(),
				ContractRevision: int32(contractRevision),
			},
		)
		if err != nil {
			return err
		}

		workflowRun := runResp.GetResult().GetWorkflowRun()
		workflowMeta.WorkflowRunId = workflowRun.GetId()
		action.Logger.Debug().Str("workflow-run-id", workflowRun.GetId()).Msg("attestation initialized in the control plane")
	}

	// Initialize the local attestation crafter
	// NOTE: important to run this initialization here since workflowMeta is populated
	// with the workflowRunId that comes from the control plane
	initOpts := &crafter.InitOpts{
		WfInfo: workflowMeta, SchemaV1: contractVersion.GetV1(),
		DryRun: action.dryRun,
	}

	if err := action.c.Init(initOpts); err != nil {
		return err
	}

	// Load the env variables both the system populated and the user predefined ones
	if err := action.c.ResolveEnvVars(); err != nil {
		if action.dryRun {
			return nil
		}

		_ = action.c.Reset()
		return err
	}

	return nil
}
