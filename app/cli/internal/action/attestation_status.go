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
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	pbc "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	intoto "github.com/in-toto/attestation/go/v1"
)

type AttestationStatusOpts struct {
	*ActionsOpts
	UseAttestationRemoteState bool
	isPushed                  bool
	LocalStatePath            string
}

type AttestationStatus struct {
	*ActionsOpts
	c *crafter.Crafter
	// Do not show information about the project version release status
	isPushed             bool
	skipPolicyEvaluation bool
}

type AttestationStatusResult struct {
	AttestationID               string                            `json:"attestationID"`
	InitializedAt               *time.Time                        `json:"initializedAt"`
	WorkflowMeta                *AttestationStatusWorkflowMeta    `json:"workflowMeta"`
	Materials                   []AttestationStatusResultMaterial `json:"materials"`
	EnvVars                     map[string]string                 `json:"envVars"`
	RunnerContext               *AttestationResultRunnerContext   `json:"runnerContext"`
	DryRun                      bool                              `json:"dryRun"`
	Annotations                 []*Annotation                     `json:"annotations"`
	IsPushed                    bool                              `json:"isPushed"`
	PolicyEvaluations           map[string][]*PolicyEvaluation    `json:"policy_evaluations,omitempty"`
	HasPolicyViolations         bool                              `json:"has_policy_violations"`
	MustBlockOnPolicyViolations bool                              `json:"must_block_on_policy_violations"`
	// This might only be set if the attestation is pushed
	Digest string `json:"digest"`
}

type AttestationResultRunnerContext struct {
	EnvVars            map[string]string
	JobURL, RunnerType string
}

type AttestationStatusWorkflowMeta struct {
	WorkflowID, Name, Team, Project, ContractRevision, ContractName, Organization string
	ProjectVersion                                                                *ProjectVersion
}

type AttestationStatusResultMaterial struct {
	*Material
	Set, IsOutput, Required bool
}

func NewAttestationStatus(cfg *AttestationStatusOpts) (*AttestationStatus, error) {
	c, err := newCrafter(&newCrafterStateOpts{enableRemoteState: cfg.UseAttestationRemoteState, localStatePath: cfg.LocalStatePath}, cfg.CPConnection, crafter.WithLogger(&cfg.Logger))
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	return &AttestationStatus{
		ActionsOpts: cfg.ActionsOpts,
		c:           c,
		isPushed:    cfg.isPushed,
	}, nil
}

func WithSkipPolicyEvaluation() func(*AttestationStatus) {
	return func(opts *AttestationStatus) {
		opts.skipPolicyEvaluation = true
	}
}

type AttestationStatusOpt func(*AttestationStatus)

func (action *AttestationStatus) Run(ctx context.Context, attestationID string, opts ...AttestationStatusOpt) (*AttestationStatusResult, error) {
	for _, opt := range opts {
		opt(action)
	}

	c := action.c

	if initialized, err := c.AlreadyInitialized(ctx, attestationID); err != nil {
		return nil, fmt.Errorf("checking if attestation is already initialized: %w", err)
	} else if !initialized {
		return nil, ErrAttestationNotInitialized
	}

	if err := c.LoadCraftingState(ctx, attestationID); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return nil, err
	}

	att := c.CraftingState.Attestation
	workflowMeta := att.GetWorkflow()

	res := &AttestationStatusResult{
		AttestationID: workflowMeta.GetWorkflowRunId(),
		WorkflowMeta: &AttestationStatusWorkflowMeta{
			WorkflowID:       workflowMeta.GetWorkflowId(),
			Name:             workflowMeta.GetName(),
			Organization:     workflowMeta.GetOrganization(),
			Project:          workflowMeta.GetProject(),
			Team:             workflowMeta.GetTeam(),
			ContractRevision: workflowMeta.GetSchemaRevision(),
			ContractName:     workflowMeta.GetContractName(),
		},
		InitializedAt:               toTimePtr(att.InitializedAt.AsTime()),
		DryRun:                      c.CraftingState.DryRun,
		Annotations:                 pbAnnotationsToAction(c.CraftingState.InputSchema.GetAnnotations()),
		IsPushed:                    action.isPushed,
		MustBlockOnPolicyViolations: att.GetBlockOnPolicyViolation(),
	}

	if !action.skipPolicyEvaluation {
		// We need to render the statement to get the policy evaluations
		attClient := pb.NewAttestationServiceClient(action.CPConnection)
		renderer, err := renderer.NewAttestationRenderer(c.CraftingState, attClient, "", "", nil, renderer.WithLogger(action.Logger))
		if err != nil {
			return nil, fmt.Errorf("rendering statement: %w", err)
		}

		// We do not want to evaluate policies here during render since we want to do it in a separate step
		statement, err := renderer.RenderStatement(ctx)
		if err != nil {
			return nil, fmt.Errorf("rendering statement: %w", err)
		}

		res.PolicyEvaluations, res.HasPolicyViolations, err = action.getPolicyEvaluations(ctx, c, attestationID, statement)
		if err != nil {
			return nil, fmt.Errorf("getting policy evaluations: %w", err)
		}
	}

	if v := workflowMeta.GetVersion(); v != nil {
		res.WorkflowMeta.ProjectVersion = &ProjectVersion{
			Version:        v.GetVersion(),
			Prerelease:     v.GetPrerelease(),
			MarkAsReleased: v.GetMarkAsReleased(),
		}
	}

	// Let's perform the following steps in order to show all possible materials:
	// 1. Populate the materials that are defined in the contract schema
	// 2. Populate the materials that are not defined in the contract schema, added inline in the attestation
	// In order to avoid duplicates, we keep track of the visited materials
	if err := populateMaterials(c.CraftingState.CraftingState, res); err != nil {
		return nil, fmt.Errorf("populating materials: %w", err)
	}

	// User defined env variables
	envVars := make(map[string]string)
	for _, e := range c.CraftingState.InputSchema.EnvAllowList {
		envVars[e] = ""
		if val, found := c.CraftingState.Attestation.EnvVars[e]; found {
			envVars[e] = val
		}
	}

	res.EnvVars = envVars

	runnerEnvVars, errors := c.Runner.ResolveEnvVars()
	var combinedErrs string
	for _, err := range errors {
		combinedErrs += (*err).Error() + "\n"
	}

	if len(errors) > 0 && !c.CraftingState.DryRun {
		return nil, fmt.Errorf("error resolving env vars: %s", combinedErrs)
	}

	res.RunnerContext = &AttestationResultRunnerContext{
		EnvVars:    runnerEnvVars,
		RunnerType: att.RunnerType.String(),
		JobURL:     att.RunnerUrl,
	}

	return res, nil
}

// getPolicyEvaluations retrieves both material-level and attestation-level policy evaluations and returns if it has violations
func (action *AttestationStatus) getPolicyEvaluations(ctx context.Context, c *crafter.Crafter, attestationID string, statement *intoto.Statement) (map[string][]*PolicyEvaluation, bool, error) {
	// grouped by material name
	evaluations := make(map[string][]*PolicyEvaluation)
	var hasViolations bool

	// Add attestation-level policy evaluations
	if err := c.EvaluateAttestationPolicies(ctx, attestationID, statement); err != nil {
		return nil, false, fmt.Errorf("evaluating attestation policies: %w", err)
	}

	// map evaluations
	for _, v := range c.CraftingState.Attestation.GetPolicyEvaluations() {
		keyName := v.MaterialName
		if keyName == "" {
			keyName = chainloop.AttPolicyEvaluation
		}

		if len(v.GetViolations()) > 0 {
			hasViolations = true
		}

		if existing, ok := evaluations[keyName]; ok {
			evaluations[keyName] = append(existing, policyEvaluationStateToActionForStatus(v))
		} else {
			evaluations[keyName] = []*PolicyEvaluation{policyEvaluationStateToActionForStatus(v)}
		}
	}

	return evaluations, hasViolations, nil
}

// populateMaterials populates the materials in the attestation result regardless of where they are defined
// (contract schema or inline in the attestation)
func populateMaterials(craftingState *v1.CraftingState, res *AttestationStatusResult) error {
	visitedMaterials := make(map[string]struct{})
	attsMaterials := craftingState.GetAttestation().GetMaterials()
	inputSchemaMaterials := craftingState.GetInputSchema().GetMaterials()

	if err := populateContractMaterials(inputSchemaMaterials, attsMaterials, res, visitedMaterials); err != nil {
		return fmt.Errorf("adding materials from the contract: %w", err)
	}

	if err := populateAdditionalMaterials(attsMaterials, res, visitedMaterials); err != nil {
		return fmt.Errorf("adding materials outside the contract: %w", err)
	}

	return nil
}

// populateContractMaterials populates the materials that are defined in the contract schema
func populateContractMaterials(inputSchemaMaterials []*pbc.CraftingSchema_Material, attsMaterial map[string]*v1.Attestation_Material, res *AttestationStatusResult, visitedMaterials map[string]struct{}) error {
	for _, m := range inputSchemaMaterials {
		materialResult := &AttestationStatusResultMaterial{
			Material: &Material{
				Name: m.Name, Type: m.Type.String(),
				Annotations: pbAnnotationsToAction(m.Annotations),
			},
			IsOutput: m.Output, Required: !m.Optional,
		}

		if cm, found := attsMaterial[m.Name]; found {
			if err := setMaterialValue(cm, materialResult); err != nil {
				return fmt.Errorf("setting material value: %w", err)
			}
		}

		res.Materials = append(res.Materials, *materialResult)
		visitedMaterials[m.Name] = struct{}{}
	}
	return nil
}

// populateAdditionalMaterials populates the materials that are not defined in the contract schema
func populateAdditionalMaterials(attsMaterials map[string]*v1.Attestation_Material, res *AttestationStatusResult, visitedMaterials map[string]struct{}) error {
	for name, m := range attsMaterials {
		if _, found := visitedMaterials[name]; found {
			continue
		}

		// No need to check for name collisions, as it is not defined in the contract schema and it's
		// autogenerated by the crafter
		materialResult := &AttestationStatusResultMaterial{
			Material: &Material{
				Name:        name,
				Type:        m.GetMaterialType().String(),
				Annotations: stateAnnotationToAction(m.Annotations),
			},
			// No need to check if the material is optional or not, as it is not defined in the contract schema
			// TODO: Make IsOutput configurable
			IsOutput: false, Required: false,
		}

		if err := setMaterialValue(m, materialResult); err != nil {
			return fmt.Errorf("setting material value: %w", err)
		}

		res.Materials = append(res.Materials, *materialResult)
	}
	return nil
}

func pbAnnotationsToAction(in []*pbc.Annotation) []*Annotation {
	res := make([]*Annotation, 0, len(in))

	for _, a := range in {
		res = append(res, &Annotation{
			Name:  a.GetName(),
			Value: a.GetValue(),
		})
	}

	return res
}

// stateAnnotationToAction converts the map of annotations to a slice of []*Annotation
func stateAnnotationToAction(in map[string]string) []*Annotation {
	res := make([]*Annotation, 0, len(in))

	for k, v := range in {
		res = append(res, &Annotation{
			Name:  k,
			Value: v,
		})
	}

	return res
}

func setMaterialValue(w *v1.Attestation_Material, o *AttestationStatusResultMaterial) error {
	switch m := w.GetM().(type) {
	case *v1.Attestation_Material_String_:
		o.Value = m.String_.GetValue()
		o.Hash = m.String_.GetDigest()
	case *v1.Attestation_Material_ContainerImage_:
		o.Value = m.ContainerImage.GetName()
		o.Hash = m.ContainerImage.GetDigest()
	case *v1.Attestation_Material_Artifact_:
		o.Value = m.Artifact.GetName()
		o.Hash = m.Artifact.GetDigest()
	case *v1.Attestation_Material_SbomArtifact:
		o.Value = m.SbomArtifact.GetArtifact().GetName()
		o.Hash = m.SbomArtifact.GetArtifact().GetDigest()
	default:
		return fmt.Errorf("unknown material type: %T", m)
	}

	// Set common fields
	o.Set = true
	o.Tag = w.GetContainerImage().GetTag()

	return nil
}

func policyEvaluationStateToActionForStatus(in *v1.PolicyEvaluation) *PolicyEvaluation {
	var pr *PolicyReference
	if in.PolicyReference != nil {
		pr = &PolicyReference{
			Name: in.PolicyReference.Name,
		}
	}

	violations := make([]*PolicyViolation, 0, len(in.Violations))
	for _, v := range in.Violations {
		violations = append(violations, &PolicyViolation{
			Subject: v.Subject,
			Message: v.Message,
		})
	}

	return &PolicyEvaluation{
		Name:            in.Name,
		MaterialName:    in.MaterialName,
		Description:     in.Description,
		Annotations:     in.Annotations,
		PolicyReference: pr,
		Violations:      violations,
		Skipped:         in.Skipped,
		SkipReasons:     in.SkipReasons,
	}
}
