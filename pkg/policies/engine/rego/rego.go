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
	"fmt"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
)

// Rego policy checker for chainloop attestations and materials
type Rego struct {
}

const (
	inputArgs      = "args"
	violationsRule = "violations"
	resultRule     = "result"
)

// Force interface
var _ engine.PolicyEngine = (*Rego)(nil)

func (r *Rego) Verify(ctx context.Context, policy *engine.Policy, input []byte, args map[string]any) (*engine.EvaluationResult, error) {
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

	// add query. Note that the predefined rule to look for is `violations`
	res, err := queryRego(ctx, resultRule, parsedModule, regoInput, regoFunc)
	if err != nil {
		return nil, err
	}

	// If `result` has been found, parse it
	if res != nil {
		return parseResultRule(res, policy)
	}

	// query for `violations` rule
	res, err = queryRego(ctx, violationsRule, parsedModule, regoInput, regoFunc)
	if err != nil {
		return nil, err
	}

	// If res is nil, it means that the rule hasn't been found
	if res == nil {
		return nil, fmt.Errorf("failed to evaluate policy: neither '%s' nor '%s' rule found", resultRule, violationsRule)
	}

	return parseViolationsRule(res, policy)
}

func parseViolationsRule(res rego.ResultSet, policy *engine.Policy) (*engine.EvaluationResult, error) {
	violations := make([]*engine.PolicyViolation, 0)
	for _, exp := range res {
		for _, val := range exp.Expressions {
			ruleResults, ok := val.Value.([]interface{})
			if !ok {
				return nil, fmt.Errorf("failed to evaluate policy expression evaluation result: %s", val.Text)
			}

			for _, result := range ruleResults {
				reasonStr, ok := result.(string)
				if !ok {
					return nil, fmt.Errorf("failed to evaluate rule result: %s", val.Text)
				}

				violations = append(violations, &engine.PolicyViolation{
					Subject:   policy.Name,
					Violation: reasonStr,
				})
			}
		}
	}

	return &engine.EvaluationResult{
		Violations: violations,
		Skipped:    false, // best effort
		Message:    "",
	}, nil
}

func parseResultRule(res rego.ResultSet, policy *engine.Policy) (*engine.EvaluationResult, error) {
	result := &engine.EvaluationResult{Violations: make([]*engine.PolicyViolation, 0)}
	for _, exp := range res {
		for _, val := range exp.Expressions {
			ruleResult, ok := val.Value.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("failed to evaluate policy evaluation result: %s", val.Text)
			}

			skipped, ok := ruleResult["skipped"].(bool)
			if !ok {
				return nil, fmt.Errorf("failed to evaluate 'skipped' field in policy evaluation result: %s", val.Text)
			}

			message, ok := ruleResult["message"].(string)
			if !ok {
				return nil, fmt.Errorf("failed to evaluate 'message' field in policy evaluation result: %s", val.Text)
			}

			violations, ok := ruleResult["violations"].([]any)
			if !ok {
				return nil, fmt.Errorf("failed to evaluate 'violations' field in policy evaluation result: %s", val.Text)
			}

			result.Skipped = skipped
			result.Message = message

			for _, violation := range violations {
				vs, ok := violation.(string)
				if !ok {
					return nil, fmt.Errorf("failed to evaluate violation in policy evaluation result: %s", val.Text)
				}
				result.Violations = append(result.Violations, &engine.PolicyViolation{Subject: policy.Name, Violation: vs})
			}
		}
	}

	return result, nil
}

func queryRego(ctx context.Context, ruleName string, parsedModule *ast.Module, options ...func(r *rego.Rego)) (rego.ResultSet, error) {
	query := rego.Query(fmt.Sprintf("%v.%s\n", parsedModule.Package.Path, ruleName))
	regoEval := rego.New(append(options, query)...)
	res, err := regoEval.Eval(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate policy: %w", err)
	}

	return res, nil
}
