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

package action

import (
	"fmt"
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/internal/policydevel"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
)

type PolicyEvalOpts struct {
	MaterialFile string
	Kind         string
	Annotations  map[string]string
	PolicyPath   string
}

type PolicyEvalResult struct {
	Passed     bool
	Violations []string
}

type PolicyEval struct {
	*ActionsOpts
	opts *PolicyEvalOpts
}

func NewPolicyEval(opts *PolicyEvalOpts, actionOpts *ActionsOpts) (*PolicyEval, error) {
	return &PolicyEval{
		ActionsOpts: actionOpts,
		opts:        opts,
	}, nil
}

func (action *PolicyEval) Run() (*PolicyEvalResult, error) {
	// Validate material
	materialKind, ok := schemaapi.CraftingSchema_Material_MaterialType_value[action.opts.Kind]
	if !ok {
		return nil, fmt.Errorf("invalid material kind: %s", action.opts.Kind)
	}

	// Read material file
	materialContent, err := os.ReadFile(action.opts.MaterialFile)
	if err != nil {
		return nil, fmt.Errorf("reading material file: %w", err)
	}

	// Create evaluation options
	evalOpts := &policydevel.EvalOptions{
		PolicyPath:   action.opts.PolicyPath,
		Material:     materialContent,
		MaterialKind: schemaapi.CraftingSchema_Material_MaterialType(materialKind),
		Annotations:  action.opts.Annotations,
	}

	// Evaluate policy
	result, err := policydevel.Evaluate(evalOpts)
	if err != nil {
		return nil, fmt.Errorf("evaluating policy: %w", err)
	}

	// Convert violations to strings
	violations := make([]string, 0, len(result.Violations))
	for _, v := range result.Violations {
		violations = append(violations, v.Message)
	}

	return &PolicyEvalResult{
		Passed:     result.Passed,
		Violations: violations,
	}, nil
}
