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
	"fmt"
	"time"

	pbc "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
)

type AttestationStatusOpts struct {
	*ActionsOpts
}

type AttestationStatus struct {
	*ActionsOpts
	c *crafter.Crafter
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
}

type AttestationResultRunnerContext struct {
	EnvVars            map[string]string
	JobURL, RunnerType string
}

type AttestationStatusWorkflowMeta struct {
	WorkflowID, Name, Team, Project, ContractRevision string
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

	return &AttestationStatus{ActionsOpts: cfg.ActionsOpts, c: c}, nil
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
			Project:          workflowMeta.GetProject(),
			Team:             workflowMeta.GetTeam(),
			ContractRevision: workflowMeta.GetSchemaRevision(),
		},
		InitializedAt: toTimePtr(att.InitializedAt.AsTime()),
		DryRun:        c.CraftingState.DryRun,
		Annotations:   pbAnnotationsToAction(c.CraftingState.InputSchema.GetAnnotations()),
	}

	// Materials
	var err error
	if res.Materials, err = craftingStateToMaterials(c.CraftingState); err != nil {
		return nil, fmt.Errorf("failed to retrieve materials: %w", err)
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

func craftingStateToMaterials(craftingState *v1.CraftingState) ([]AttestationStatusResultMaterial, error) {
	var materials []AttestationStatusResultMaterial
	for _, m := range craftingState.InputSchema.Materials {
		materialResult := AttestationStatusResultMaterial{
			Material: &Material{
				Name: m.Name, Type: m.Type.String(),
				Annotations: pbAnnotationsToAction(m.Annotations),
			},
			IsOutput: m.Output, Required: !m.Optional,
		}

		// If it has been added already we load the value
		if cm, found := craftingState.Attestation.Materials[m.Name]; found {
			if err := setMaterialValue(cm, materialResult.Material); err != nil {
				return nil, err
			}
			materialResult.Set = true
		}

		materials = append(materials, materialResult)
	}

	return materials, nil
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

func setMaterialValue(w *v1.Attestation_Material, o *Material) error {
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

	return nil
}
