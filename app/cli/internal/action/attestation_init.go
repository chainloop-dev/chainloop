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
	"errors"
	"fmt"
	"strconv"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	clientAPI "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/rs/zerolog"
)

type AttestationInitOpts struct {
	*ActionsOpts
	DryRun bool
	// Force the initialization and override any existing, in-progress ones.
	// Note that this is only useful when local-based attestation state is configured
	// since it's a protection to make sure you don't override the state by mistake
	Force          bool
	UseRemoteState bool
	LocalStatePath string
}

type AttestationInit struct {
	*ActionsOpts
	dryRun, force  bool
	c              *crafter.Crafter
	useRemoteState bool
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
	c, err := newCrafter(&newCrafterStateOpts{enableRemoteState: cfg.UseRemoteState, localStatePath: cfg.LocalStatePath}, cfg.CPConnection, crafter.WithLogger(&cfg.Logger))
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	return &AttestationInit{
		ActionsOpts:    cfg.ActionsOpts,
		c:              c,
		dryRun:         cfg.DryRun,
		force:          cfg.Force,
		useRemoteState: cfg.UseRemoteState,
	}, nil
}

// returns the attestation ID
type AttestationInitRunOpts struct {
	ContractRevision             int
	ProjectName                  string
	ProjectVersion               string
	ProjectVersionMarkAsReleased bool
	WorkflowName                 string
	NewWorkflowContractName      string
}

func (action *AttestationInit) Run(ctx context.Context, opts *AttestationInitRunOpts) (string, error) {
	if action.dryRun && action.useRemoteState {
		return "", errors.New("remote state is not compatible with dry-run mode")
	}

	// During local initializations we need to make sure if there is already an attestation in progress
	// If it is and we are not "forcing" the initialization, we should return an error
	if !action.useRemoteState && !action.force {
		if initialized, _ := action.c.AlreadyInitialized(ctx, ""); initialized {
			return "", ErrAttestationAlreadyExist
		}
	}

	action.Logger.Debug().Msg("Retrieving attestation definition")
	client := pb.NewAttestationServiceClient(action.ActionsOpts.CPConnection)
	// 1 - Find or create the workflow
	workflowsResp, err := client.FindOrCreateWorkflow(ctx, &pb.FindOrCreateWorkflowRequest{
		ProjectName:  opts.ProjectName,
		WorkflowName: opts.WorkflowName,
		ContractName: opts.NewWorkflowContractName,
	})
	if err != nil {
		return "", err
	}
	workflow := workflowsResp.GetResult()

	// 2 - Get contract
	contractResp, err := client.GetContract(ctx, &pb.AttestationServiceGetContractRequest{
		ContractRevision: int32(opts.ContractRevision),
		WorkflowName:     opts.WorkflowName,
		ProjectName:      opts.ProjectName,
	})
	if err != nil {
		return "", err
	}

	contractVersion := contractResp.Result.GetContract()
	workflowMeta := &clientAPI.WorkflowMetadata{
		WorkflowId:     workflow.GetId(),
		Name:           workflow.GetName(),
		Project:        workflow.GetProject(),
		Team:           workflow.GetTeam(),
		SchemaRevision: strconv.Itoa(int(contractVersion.GetRevision())),
		ContractName:   workflow.ContractName,
	}

	if opts.ProjectVersion != "" {
		workflowMeta.Version = &clientAPI.ProjectVersion{
			Version:        opts.ProjectVersion,
			MarkAsReleased: opts.ProjectVersionMarkAsReleased,
		}
	}

	action.Logger.Debug().Msg("workflow contract and metadata retrieved from the control plane")

	// 3. enrich contract with group materials and policies
	err = enrichContractMaterials(ctx, contractVersion.GetV1(), client, &action.Logger)
	if err != nil {
		return "", fmt.Errorf("failed to apply materials from policy groups: %w", err)
	}

	// Auto discover the runner context and enforce against the one in the contract if needed
	discoveredRunner, err := crafter.DiscoverAndEnforceRunner(contractVersion.GetV1().GetRunner().GetType(), action.dryRun, action.Logger)
	if err != nil {
		return "", ErrRunnerContextNotFound{err.Error()}
	}

	// Identifier of this attestation instance
	var attestationID string
	var blockOnPolicyViolation bool

	// Init in the control plane if needed including the runner context
	if !action.dryRun {
		runResp, err := client.Init(
			ctx,
			&pb.AttestationServiceInitRequest{
				Runner:           discoveredRunner.ID(),
				JobUrl:           discoveredRunner.RunURI(),
				ContractRevision: int32(opts.ContractRevision),
				// send the workflow name explicitly provided by the user to detect that functional case
				WorkflowName:   opts.WorkflowName,
				ProjectName:    opts.ProjectName,
				ProjectVersion: opts.ProjectVersion,
			},
		)
		if err != nil {
			return "", err
		}

		workflowRun := runResp.GetResult().GetWorkflowRun()
		workflowMeta.WorkflowRunId = workflowRun.GetId()
		workflowMeta.Organization = runResp.GetResult().GetOrganization()
		blockOnPolicyViolation = runResp.GetResult().GetBlockOnPolicyViolation()
		if v := workflowMeta.Version; v != nil {
			workflowMeta.Version.Prerelease = runResp.GetResult().GetWorkflowRun().Version.GetPrerelease()
		}

		action.Logger.Debug().Str("workflow-run-id", workflowRun.GetId()).Msg("attestation initialized in the control plane")
		attestationID = workflowRun.GetId()
	}

	// Initialize the local attestation crafter
	// NOTE: important to run this initialization here since workflowMeta is populated
	// with the workflowRunId that comes from the control plane
	initOpts := &crafter.InitOpts{
		WfInfo:                 workflowMeta,
		SchemaV1:               contractVersion.GetV1(),
		DryRun:                 action.dryRun,
		AttestationID:          attestationID,
		Runner:                 discoveredRunner,
		BlockOnPolicyViolation: blockOnPolicyViolation,
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

func enrichContractMaterials(ctx context.Context, schema *v1.CraftingSchema, client pb.AttestationServiceClient, logger *zerolog.Logger) error {
	contractMaterials := schema.GetMaterials()
	for _, pgAtt := range schema.GetPolicyGroups() {
		group, _, err := policies.LoadPolicyGroup(ctx, pgAtt, &policies.LoadPolicyGroupOptions{
			Client: client,
			Logger: logger,
		})
		if err != nil {
			return fmt.Errorf("failed to load policy group: %w", err)
		}
		logger.Debug().Msgf("adding materials from policy group '%s'", group.GetMetadata().GetName())

		toAdd, err := getGroupMaterialsToAdd(group, pgAtt, contractMaterials, logger)
		if err != nil {
			return err
		}
		contractMaterials = append(contractMaterials, toAdd...)
	}

	schema.Materials = contractMaterials

	return nil
}

// merge existing materials with group ones, taking the contract's one in case of conflict
func getGroupMaterialsToAdd(group *v1.PolicyGroup, pgAtt *v1.PolicyGroupAttachment, fromContract []*v1.CraftingSchema_Material, logger *zerolog.Logger) ([]*v1.CraftingSchema_Material, error) {
	toAdd := make([]*v1.CraftingSchema_Material, 0)
	for _, groupMaterial := range group.GetSpec().GetPolicies().GetMaterials() {
		// if material has no name, it's not enforced
		if groupMaterial.GetName() == "" {
			continue
		}

		// apply bindings if needed
		csm, err := groupMaterialToCraftingSchemaMaterial(groupMaterial, group, pgAtt, logger)
		if err != nil {
			return nil, err
		}
		// skip if interpolated material name is still empty
		if csm.GetName() == "" {
			continue
		}

		// check if material already exists in the contract and skip it in that case
		ignore := false
		for _, mat := range fromContract {
			if mat.GetName() == csm.GetName() {
				logger.Warn().Msgf("material '%s' from policy group '%s' is also present in the contract and will be ignored", mat.GetName(), group.GetMetadata().GetName())
				ignore = true
			}
		}
		if !ignore {
			toAdd = append(toAdd, csm)
		}
	}

	return toAdd, nil
}

// translates materials and interpolates material names
func groupMaterialToCraftingSchemaMaterial(gm *v1.PolicyGroup_Material, group *v1.PolicyGroup, pgAtt *v1.PolicyGroupAttachment, logger *zerolog.Logger) (*v1.CraftingSchema_Material, error) {
	// Validates and computes arguments
	args, err := policies.ComputeArguments(group.GetSpec().GetInputs(), pgAtt.GetWith(), nil, logger)
	if err != nil {
		return nil, err
	}

	// Apply arguments as interpolations for materials
	gm, err = policies.InterpolateGroupMaterial(gm, args)
	if err != nil {
		return nil, err
	}

	return &v1.CraftingSchema_Material{
		Type:     gm.Type,
		Name:     gm.Name,
		Optional: gm.Optional,
	}, nil
}
