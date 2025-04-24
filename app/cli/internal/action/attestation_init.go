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
	"net/url"
	"os"
	"regexp"
	"strconv"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	clientAPI "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	NewWorkflowContractRef       string
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
	client := pb.NewAttestationServiceClient(action.CPConnection)

	// 1 - Create the workflow (and contract) if it doesn't exist
	wf, err := action.createWorkflow(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("error creating workflow: %w", err)
	}

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
		WorkflowId:     wf.ID,
		Name:           wf.Name,
		Project:        wf.Project,
		Team:           wf.Team,
		SchemaRevision: strconv.Itoa(int(contractVersion.GetRevision())),
		ContractName:   wf.ContractName,
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

	var (
		// Identifier of this attestation instance
		attestationID          string
		blockOnPolicyViolation bool
		// Timestamp Authority URL for new attestations
		timestampAuthorityURL string
	)

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
		timestampAuthorityURL = runResp.GetResult().GetSigningOptions().GetTimestampAuthorityUrl()
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
		SigningOptions:         &crafter.SigningOpts{TimestampAuthorityURL: timestampAuthorityURL},
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

// createWorkflow creates a new workflow if it doesn't exist, as well as its contract
func (action *AttestationInit) createWorkflow(ctx context.Context, opts *AttestationInitRunOpts) (*WorkflowItem, error) {
	// 1. find workflow if exists
	wf, err := NewWorkflowDescribe(action.ActionsOpts).Run(ctx, opts.WorkflowName, opts.ProjectName)
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, fmt.Errorf("error looking for workflow %q: %w", opts.WorkflowName, err)
	}

	contractRef := opts.NewWorkflowContractRef
	contractName := contractRef
	if contractRef != "" {
		if isContractReference(contractRef) {
			_, err := NewWorkflowContractDescribe(action.ActionsOpts).Run(contractRef, 0)
			if err != nil {
				return nil, fmt.Errorf("contract %q could not be found", contractRef)
			}
			// the contract exists
		} else {
			// use a default name for the contract
			contractName = defaultContractName(opts.ProjectName, opts.WorkflowName)

			// if the workflow exists, we just update its contract with the new contents
			if wf != nil {
				_, err = NewWorkflowContractUpdate(action.ActionsOpts).Run(wf.ContractName, nil, contractRef)
				if err != nil {
					return nil, fmt.Errorf("error updating contract %q: %w", wf.ContractName, err)
				}
				contractName = wf.ContractName
			} else {
				// if the workflow doesn't exist, let's create or update the contract
				cont, err := NewWorkflowContractDescribe(action.ActionsOpts).Run(contractName, 0)
				switch {
				case err != nil && status.Code(err) == codes.NotFound:
					// Contract not found, let's create it
					_, err = NewWorkflowContractCreate(action.ActionsOpts).Run(contractName, nil, contractRef)
					if err != nil {
						return nil, err
					}
				case err != nil:
					return nil, err
				default:
					// contract found, let's update it (chainloop will validate that there is an actual change in the contract file)
					_, err := NewWorkflowContractUpdate(action.ActionsOpts).Run(cont.Contract.Name, &cont.Contract.Description, contractRef)
					if err != nil {
						return nil, err
					}
				}
			}
		}
	}

	// if workflow doesn't exist, let's create it with the contract
	if wf == nil {
		wf, err = NewWorkflowCreate(action.ActionsOpts).Run(&NewWorkflowCreateOpts{
			Name:         opts.WorkflowName,
			Project:      opts.ProjectName,
			ContractName: contractName,
		})
		if err != nil {
			return nil, fmt.Errorf("error creating workflow %q: %w", opts.WorkflowName, err)
		}
	}

	return wf, nil
}

// isContractReference checks if the reference points to an existent contract name (DNS-1123
func isContractReference(ref string) bool {
	// Check if the reference is a valid URL
	if _, err := url.ParseRequestURI(ref); err == nil {
		return false
	}

	// is it a local file?
	if _, err := os.Stat(ref); err == nil {
		return false
	}

	// Check if the reference is a valid DNS-1123 name
	if matched, _ := regexp.Match("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", []byte(ref)); matched {
		return true
	}

	return false
}

func defaultContractName(project, workflow string) string {
	return fmt.Sprintf("%s-%s", project, workflow)
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
