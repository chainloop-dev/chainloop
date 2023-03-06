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
	"fmt"
	"time"

	v1 "github.com/chainloop-dev/chainloop/app/cli/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/crafter"
)

type AttestationStatusOpts struct {
	*ActionsOpts
}

type AttestationStatus struct {
	*ActionsOpts
	c *crafter.Crafter
}

type AttestationStatusResult struct {
	InitializedAt *time.Time
	WorkflowMeta  *AttestationStatusWorkflowMeta
	Materials     []AttestationStatusResultMaterial
	EnvVars       map[string]string
	RunnerContext *AttestaionResultRunnerContext
	DryRun        bool
}

type AttestaionResultRunnerContext struct {
	EnvVars            map[string]string
	JobURL, RunnerType string
}

type AttestationStatusWorkflowMeta struct {
	RunID, WorkflowID, Name, Team, Project, ContractRevision string
}

type AttestationStatusResultMaterial struct {
	Name, Type, Value       string
	Set, IsOutput, Required bool
}

func NewAttestationStatus(cfg *AttestationStatusOpts) *AttestationStatus {
	return &AttestationStatus{
		ActionsOpts: cfg.ActionsOpts,
		c:           crafter.NewCrafter(crafter.WithLogger(&cfg.Logger)),
	}
}

func (action *AttestationStatus) Run() (*AttestationStatusResult, error) {
	c := action.c

	if initialized := c.AlreadyInitialized(); !initialized {
		return nil, ErrAttestationNotInitialized
	}

	if err := c.LoadCraftingState(); err != nil {
		action.Logger.Err(err).Msg("loading existing attestation")
		return nil, err
	}

	att := c.CraftingState.Attestation
	workflowMeta := att.GetWorkflow()

	res := &AttestationStatusResult{
		WorkflowMeta: &AttestationStatusWorkflowMeta{
			RunID:            workflowMeta.GetWorkflowRunId(),
			WorkflowID:       workflowMeta.GetWorkflowId(),
			Name:             workflowMeta.GetName(),
			Project:          workflowMeta.GetProject(),
			Team:             workflowMeta.GetTeam(),
			ContractRevision: workflowMeta.GetSchemaRevision(),
		},
		InitializedAt: toTimePtr(att.InitializedAt.AsTime()),
		DryRun:        c.CraftingState.DryRun,
	}

	// Materials
	for _, m := range c.CraftingState.InputSchema.Materials {
		materialResult := &AttestationStatusResultMaterial{
			Name: m.Name, Type: m.Type.String(), IsOutput: m.Output, Required: !m.Optional,
		}

		if cm, found := c.CraftingState.Attestation.Materials[m.Name]; found {
			materialResult.Set = true
			materialResult.Value = getMaterialSetValue(cm)
		}

		res.Materials = append(res.Materials, *materialResult)
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
	res.RunnerContext = &AttestaionResultRunnerContext{
		EnvVars:    c.Runner.ResolveEnvVars(),
		RunnerType: att.RunnerType.String(),
		JobURL:     att.RunnerUrl,
	}

	return res, nil
}

func getMaterialSetValue(w *v1.Attestation_Material) string {
	switch m := w.GetM().(type) {
	case *v1.Attestation_Material_String_:
		return m.String_.GetValue()
	case *v1.Attestation_Material_ContainerImage_:
		return fmt.Sprintf("%s@%s", m.ContainerImage.GetName(), m.ContainerImage.GetDigest())
	case *v1.Attestation_Material_Artifact_:
		return fmt.Sprintf("%s@%s", m.Artifact.GetName(), m.Artifact.GetDigest())
	}

	return ""
}
