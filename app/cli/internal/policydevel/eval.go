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
}

type EvalResult struct {
	Skipped     bool
	SkipReasons []string
	Violations  []string
	Ignored     bool
}

func Evaluate(opts *EvalOptions, logger zerolog.Logger) ([]*EvalResult, error) {
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
	result, err := verifyMaterial(schema, material, opts.MaterialPath, opts.AllowedHostnames, &logger)
	if err != nil {
		return nil, err
	}

	return result, nil
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

func verifyMaterial(schema *v1.CraftingSchema, material *v12.Attestation_Material, materialPath string, allowedHostnames []string, logger *zerolog.Logger) ([]*EvalResult, error) {
	var opts []policies.PolicyVerifierOption
	if len(allowedHostnames) > 0 {
		opts = append(opts, policies.WithAllowedHostnames(allowedHostnames...))
	}
	v := policies.NewPolicyVerifier(schema, nil, logger, opts...)
	policyEvs, err := v.VerifyMaterial(context.Background(), material, materialPath)
	if err != nil {
		return nil, err
	}

	// no evaluations were returned
	if len(policyEvs) == 0 {
		return nil, fmt.Errorf("no execution branch matched for kind %s", material.MaterialType.String())
	}

	results := make([]*EvalResult, 0, len(policyEvs))
	for _, policyEv := range policyEvs {
		result := &EvalResult{
			Skipped:     policyEv.GetSkipped(),
			SkipReasons: policyEv.SkipReasons,
			Ignored:     false,
		}

		// Collect all violation messages
		violations := make([]string, 0, len(policyEv.Violations))
		for _, v := range policyEv.Violations {
			violations = append(violations, v.Message)
		}
		result.Violations = violations

		results = append(results, result)
	}

	return results, nil
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
