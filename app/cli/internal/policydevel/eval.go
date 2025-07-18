//
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
	"path/filepath"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	engine "github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine/rego"
)

type EvalOptions struct {
	PolicyPath   string
	Material     []byte
	MaterialKind schemaapi.CraftingSchema_Material_MaterialType
	Annotations  map[string]string
}

type EvalResult struct {
	Passed     bool
	Violations []Violation
}

type Violation struct {
	Message string
}

func Evaluate(opts *EvalOptions) (*EvalResult, error) {
	// 1. Load raw policy file
	raw, err := loadPolicy(opts.PolicyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load policy: %w", err)
	}

	// 2. Identify and unmarshal
	format, err := unmarshal.IdentifyFormat(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to identify policy format: %w", err)
	}

	var policy schemaapi.Policy
	if err := unmarshal.FromRaw(raw, format, &policy, true); err != nil {
		return nil, fmt.Errorf("failed to unmarshal policy file: %w", err)
	}

	// 3. Find the matching policy by kind
	var regoSource string
	found := false

	for _, p := range policy.Spec.Policies {
		if p.Kind != opts.MaterialKind {
			continue
		}

		switch {
		case p.GetEmbedded() != "":
			regoSource = p.GetEmbedded()
			found = true
		case p.GetPath() != "":
			fullPath := filepath.Join(filepath.Dir(opts.PolicyPath), p.GetPath())
			data, err := os.ReadFile(fullPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read external rego at %s: %w", p.GetPath(), err)
			}
			regoSource = string(data)
			found = true
		default:
			return nil, fmt.Errorf("policy for kind %s has no embedded or path field", opts.MaterialKind)
		}
		break
	}

	if !found {
		return nil, fmt.Errorf("no matching policy found for kind: %s", opts.MaterialKind)
	}

	// 4. Prepare input
	input, err := prepareInput(opts.Material, opts.MaterialKind, opts.Annotations)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare input: %w", err)
	}

	// 5. Evaluate using OPA
	result, err := evaluatePolicy([]byte(regoSource), input)
	if err != nil {
		return nil, fmt.Errorf("policy evaluation failed: %w", err)
	}

	return result, nil
}

func loadPolicy(policyPath string) ([]byte, error) {
	// If the path is not absolute, look for it in the current directory
	if !filepath.IsAbs(policyPath) {
		fullPath := filepath.Join(".", policyPath)
		if _, err := os.Stat(fullPath); err != nil {
			return nil, fmt.Errorf("policy file not found in current directory: %w", err)
		}
		policyPath = fullPath
	}

	// Read the policy file
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		return nil, fmt.Errorf("reading policy file: %w", err)
	}

	return policy, nil
}

func prepareInput(material []byte, kind schemaapi.CraftingSchema_Material_MaterialType, annotations map[string]string) (interface{}, error) {
	// TODO
	// Parse material based on its kind
	var materialValue interface{}
	switch kind {
	case schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON:
		if err := json.Unmarshal(material, &materialValue); err != nil {
			return nil, fmt.Errorf("unmarshaling JSON material: %w", err)
		}
	}
	return materialValue, nil
}

func evaluatePolicy(policy []byte, input interface{}) (*EvalResult, error) {
	ctx := context.Background()

	// Marshal input to JSON since the engine expects []byte
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input for rego engine: %w", err)
	}

	rego := &rego.Rego{
		OperatingMode: rego.EnvironmentModeRestrictive,
	}

	enginePolicy := &engine.Policy{
		Name:   "policy.rego",
		Source: policy,
	}

	result, err := rego.Verify(ctx, enginePolicy, inputBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("rego evaluation failed: %w", err)
	}

	evalResult := &EvalResult{
		Passed:     len(result.Violations) == 0 && !result.Ignore,
		Violations: make([]Violation, 0, len(result.Violations)),
	}

	for _, v := range result.Violations {
		evalResult.Violations = append(evalResult.Violations, Violation{Message: v.Violation})
	}

	return evalResult, nil
}
