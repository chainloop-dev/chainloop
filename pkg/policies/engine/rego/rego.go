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

package rego

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

// Rego policy checker for chainloop attestations and materials
type Rego struct {
}

const inputArgs = "args"

// Force interface
var _ engine.PolicyEngine = (*Rego)(nil)

func (r *Rego) Verify(ctx context.Context, policy *engine.Policy, input []byte, args map[string]any) ([]*engine.PolicyViolation, error) {
	policyString := string(policy.Source)
	parsedModule, err := ast.ParseModule(policy.Name, policyString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rego policy: %w", err)
	}

	// Decode input as json
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var decodedInput interface{}
	if err := decoder.Decode(&decodedInput); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// put arguments embedded in the input object
	if args != nil {
		inputMap, ok := decodedInput.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected input arguments")
		}
		inputMap[inputArgs] = args
		decodedInput = inputMap
	}

	// add input
	regoInput := rego.Input(decodedInput)

	// add module
	regoFunc := rego.ParsedModule(parsedModule)

	// add query. Note that the predefined rule to look for is `deny`
	query := rego.Query(fmt.Sprintf("%v.deny\n", parsedModule.Package.Path))

	regoEval := rego.New(regoInput, regoFunc, query)

	res, err := regoEval.Eval(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate policy: %w", err)
	}

	// If res is nil, it means that the rule hasn't been found
	if res == nil {
		return nil, errors.New("failed to evaluate policy: no 'deny' rule found")
	}

	violations := make([]*engine.PolicyViolation, 0)
	for _, exp := range res {
		for _, val := range exp.Expressions {
			denyReasons, ok := val.Value.([]interface{})
			if !ok {
				return nil, fmt.Errorf("failed to evaluate policy expression evaluation result: %s", val.Text)
			}

			for _, reason := range denyReasons {
				reasonStr, ok := reason.(string)
				if !ok {
					return nil, fmt.Errorf("failed to evaluate deny reason: %s", val.Text)
				}

				violations = append(violations, &engine.PolicyViolation{
					Subject:   policy.Name,
					Violation: reasonStr,
				})
			}
		}
	}

	return violations, nil
}
