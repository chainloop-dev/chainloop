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

package policydevel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/pkg/casclient"
	"github.com/chainloop-dev/chainloop/pkg/policies"
	"github.com/rs/zerolog"

	v12 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials"
)

type EvalOptions struct {
	PolicyPath       string
	MaterialKind     string
	Annotations      map[string]string
	MaterialPath     string
	Inputs           map[string]string
	AllowedHostnames []string
	Debug            bool
	EnablePrint      bool
}

type EvalResult struct {
	Violations  []string `json:"violations"`
	SkipReasons []string `json:"skip_reasons"`
	Skipped     bool     `json:"skipped"`
}

type EvalSummary struct {
	Result    *EvalResult           `json:"result"`
	DebugInfo *EvalSummaryDebugInfo `json:"debug_info,omitempty"`
}

type EvalSummaryDebugInfo struct {
	Inputs     []json.RawMessage `json:"inputs"`
	RawResults []json.RawMessage `json:"raw_results"`
}

func Evaluate(opts *EvalOptions, logger zerolog.Logger) (*EvalSummary, error) {
	// 1. Create crafting schema
	schema, err := createCraftingSchema(opts.PolicyPath, opts.Inputs)
	if err != nil {
		return nil, err
	}

	// 2. Craft material with annotations
	material, err := craftMaterial(opts.MaterialPath, opts.MaterialKind, &logger)
	if err != nil {
		return nil, err
	}
	material.Annotations = opts.Annotations

	// 3. Verify material against policy
	summary, err := verifyMaterial(schema, material, opts.MaterialPath, opts.Debug, opts.EnablePrint, opts.AllowedHostnames, &logger)
	if err != nil {
		return nil, err
	}

	return summary, nil
}

func createCraftingSchema(policyPath string, inputs map[string]string) (*v1.CraftingSchema, error) {
	return &v1.CraftingSchema{
		Policies: &v1.Policies{
			Materials: []*v1.PolicyAttachment{
				{
					Policy: &v1.PolicyAttachment_Ref{Ref: fmt.Sprintf("file://%s", policyPath)},
					With:   inputs,
				},
			},
			Attestation: nil,
		},
		PolicyGroups: nil,
	}, nil
}

func verifyMaterial(schema *v1.CraftingSchema, material *v12.Attestation_Material, materialPath string, debug, enablePrint bool, allowedHostnames []string, logger *zerolog.Logger) (*EvalSummary, error) {
	var opts []policies.PolicyVerifierOption
	if len(allowedHostnames) > 0 {
		opts = append(opts, policies.WithAllowedHostnames(allowedHostnames...))
	}

	opts = append(opts, policies.WithIncludeRawData(debug))
	opts = append(opts, policies.WithEnablePrint(enablePrint))

	v := policies.NewPolicyVerifier(schema, nil, logger, opts...)
	policyEvs, err := v.VerifyMaterial(context.Background(), material, materialPath)
	if err != nil {
		return nil, err
	}

	if len(policyEvs) == 0 || policyEvs[0] == nil {
		return nil, fmt.Errorf("no execution branch matched for kind %s", material.MaterialType.String())
	}

	// Only one evaluation expected for a single policy attachment
	policyEv := policyEvs[0]

	summary := &EvalSummary{
		Result: &EvalResult{
			Skipped:     policyEv.GetSkipped(),
			SkipReasons: policyEv.SkipReasons,
			Violations:  make([]string, 0, len(policyEv.Violations)),
		},
	}

	// Collect violation messages
	for _, v := range policyEv.Violations {
		summary.Result.Violations = append(summary.Result.Violations, v.Message)
	}

	// Include raw debug info if requested
	if debug {
		summary.DebugInfo = &EvalSummaryDebugInfo{
			Inputs:     []json.RawMessage{},
			RawResults: []json.RawMessage{},
		}

		for _, rr := range policyEv.RawResults {
			if rr == nil {
				continue
			}
			// Take the first input found, as we only allow one material input
			if len(summary.DebugInfo.Inputs) == 0 && rr.Input != nil {
				summary.DebugInfo.Inputs = append(summary.DebugInfo.Inputs, json.RawMessage(rr.Input))
			}
			// Collect all output raw results
			if rr.Output != nil {
				summary.DebugInfo.RawResults = append(summary.DebugInfo.RawResults, json.RawMessage(rr.Output))
			}
		}
	}

	return summary, nil
}

func craftMaterial(materialPath, materialKind string, logger *zerolog.Logger) (*v12.Attestation_Material, error) {
	if fileNotExists(materialPath) {
		return nil, fmt.Errorf("%s: does not exists", materialPath)
	}
	backend := &casclient.CASBackend{
		Name:     "backend",
		MaxSize:  0,
		Uploader: nil, // Skip uploads
	}

	// Explicit kind
	if materialKind != "" {
		kind, ok := v1.CraftingSchema_Material_MaterialType_value[materialKind]
		if !ok {
			return nil, fmt.Errorf("invalid material kind: %s", materialKind)
		}
		return craft(materialPath, v1.CraftingSchema_Material_MaterialType(kind), "material", backend, logger)
	}

	// Auto-detect kind
	for _, kind := range v1.CraftingMaterialInValidationOrder {
		m, err := craft(materialPath, kind, "auto-detected-material", backend, logger)
		if err == nil {
			return m, nil
		}
	}

	return nil, fmt.Errorf("could not auto-detect material kind for: %s", materialPath)
}

func craft(materialPath string, kind v1.CraftingSchema_Material_MaterialType, name string, backend *casclient.CASBackend, logger *zerolog.Logger) (*v12.Attestation_Material, error) {
	materialSchema := &v1.CraftingSchema_Material{
		Type: kind,
		Name: name,
	}

	m, err := materials.Craft(context.Background(), materialSchema, materialPath, backend, nil, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to craft material (kind=%s): %w", kind.String(), err)
	}
	return m, nil
}

func fileNotExists(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}
