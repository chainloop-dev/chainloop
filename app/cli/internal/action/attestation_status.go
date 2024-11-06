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
	"time"

	pbc "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
)

type AttestationStatusOpts struct {
	*ActionsOpts
	UseAttestationRemoteState bool
	isPushed                  bool
}

type AttestationStatus struct {
	*ActionsOpts
	c *crafter.Crafter
	// Do not show information about the project version release status
	isPushed bool
}

type AttestationStatusResult struct {
	AttestationID string                            `json:"attestationID"`
	InitializedAt *time.Time                        `json:"initializedAt"`
	WorkflowMeta  *AttestationStatusWorkflowMeta    `json:"workflowMeta"`
	Materials     []AttestationStatusResultMaterial `json:"materials"`
	EnvVars       map[string]string                 `json:"envVars"`
	RunnerContext *AttestationResultRunnerContext   `json:"runnerContext"`
	DryRun        bool                              `json:"dryRun"`
	Annotations   []*Annotation                     `json:"annotations"`
	IsPushed      bool                              `json:"isPushed"`
}

type AttestationResultRunnerContext struct {
	EnvVars            map[string]string
	JobURL, RunnerType string
}

type AttestationStatusWorkflowMeta struct {
	WorkflowID, Name, Team, Project, ContractRevision, Organization string
	ProjectVersion                                                  *ProjectVersion
}

type AttestationStatusResultMaterial struct {
	*Material
	Set, IsOutput, Required bool
}

func NewAttestationStatus(cfg *AttestationStatusOpts) (*AttestationStatus, error) {
	c, err := newCrafter(cfg.UseAttestationRemoteState, cfg.CPConnection, crafter.WithLogger(&cfg.Logger))
	if err != nil {
		return nil, fmt.Errorf("failed to load crafter: %w", err)
	}

	return &AttestationStatus{
		ActionsOpts: cfg.ActionsOpts,
		c:           c,
		isPushed:    cfg.isPushed,
	}, nil
}

func (action *AttestationStatus) Run(ctx context.Context, attestationID string) (*AttestationStatusResult, error) {
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
			ProjectVersion: &ProjectVersion{
				Version:        workflowMeta.GetVersion().GetVersion(),
				Prerelease:     workflowMeta.Version.Prerelease,
				MarkAsReleased: workflowMeta.Version.MarkAsReleased,
			},
		},
		InitializedAt: toTimePtr(att.InitializedAt.AsTime()),
		DryRun:        c.CraftingState.DryRun,
		Annotations:   pbAnnotationsToAction(c.CraftingState.InputSchema.GetAnnotations()),
		IsPushed:      action.isPushed,
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
	case *v1.Attestation_Material_ContainerImage_:
		o.Value = m.ContainerImage.GetName()
		o.Hash = m.ContainerImage.GetDigest()
	case *v1.Attestation_Material_Artifact_:
		o.Value = m.Artifact.GetName()
		o.Hash = m.Artifact.GetDigest()
	default:
		return fmt.Errorf("unknown material type: %T", m)
	}

	// Set common fields
	o.Set = true
	o.Tag = w.GetContainerImage().GetTag()

	return nil
}
