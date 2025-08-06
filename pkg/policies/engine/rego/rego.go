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

package rego

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"golang.org/x/exp/maps"
)

// Engine policy checker for chainloop attestations and materials
type Engine struct {
	// operatingMode defines the mode of running the policy engine
	// by restricting or not the operations allowed by the compiler
	operatingMode EnvironmentMode
	// allowedNetworkDomains is a list of network domains that are allowed for the compiler to access
	// when using http.send built-in function
	allowedNetworkDomains []string
}

type EngineOption func(*newEngineOptions)

func WithOperatingMode(mode EnvironmentMode) EngineOption {
	return func(e *newEngineOptions) {
		e.operatingMode = mode
	}
}

func WithAllowedNetworkDomains(domains ...string) EngineOption {
	return func(e *newEngineOptions) {
		e.allowedNetworkDomains = domains
	}
}

type newEngineOptions struct {
	operatingMode         EnvironmentMode
	allowedNetworkDomains []string
}

func WithBaseAllowedNetworkDomains(domains ...string) EngineOption {
	return func(e *newEngineOptions) {
		e.allowedNetworkDomains = domains
	}
}

// NewEngine creates a new policy engine with the given options
// default operating mode is EnvironmentModeRestrictive
// default allowed network domains are www.chainloop.dev and www.cisa.gov
// user provided allowed network domains are appended to the base ones
func NewEngine(opts ...EngineOption) *Engine {
	options := &newEngineOptions{
		operatingMode: EnvironmentModeRestrictive,
	}

	for _, opt := range opts {
		opt(options)
	}

	var baseAllowedNetworkDomains = []string{
		"www.chainloop.dev",
		"www.cisa.gov",
	}

	return &Engine{
		operatingMode: options.operatingMode,
		// append base allowed network domains to the user provided ones
		allowedNetworkDomains: append(baseAllowedNetworkDomains, options.allowedNetworkDomains...),
	}
}

// EnvironmentMode defines the mode of running the policy engine
type EnvironmentMode int32

const (
	// EnvironmentModeRestrictive restricts operations that the compiler can do
	EnvironmentModeRestrictive EnvironmentMode = 0
	// EnvironmentModePermissive allows all operations on the compiler
	EnvironmentModePermissive EnvironmentMode = 1
	inputArgs                                 = "args"
	inputElements                             = "elements"
	deprecatedRule                            = "violations"
	mainRule                                  = "result"
)

// builtinFuncNotAllowed is a list of builtin functions that are not allowed in the compiler
var builtinFuncNotAllowed = []*ast.Builtin{
	ast.OPARuntime,
	ast.RegoParseModule,
	ast.Trace,
}

// Force interface
var _ engine.PolicyEngine = (*Engine)(nil)

func (r *Engine) Verify(ctx context.Context, policy *engine.Policy, input []byte, args map[string]any) (*engine.EvaluationResult, error) {
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

	// if input is an array, transform it to an object
	if array, ok := decodedInput.([]interface{}); ok {
		inputMap := make(map[string]interface{})
		inputMap[inputElements] = array
		decodedInput = inputMap
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

	var res rego.ResultSet
	// Function to execute the query with appropriate parameters
	executeQuery := func(rule string, strict bool) error {
		if strict {
			res, err = queryRego(ctx, rule, parsedModule, regoInput, regoFunc, rego.Capabilities(r.Capabilities()), rego.StrictBuiltinErrors(true))
		} else {
			res, err = queryRego(ctx, rule, parsedModule, regoInput, regoFunc, rego.Capabilities(r.Capabilities()))
		}
		return err
	}

	// Try the main rule first
	if err := executeQuery(mainRule, r.operatingMode == EnvironmentModeRestrictive); err != nil {
		return nil, err
	}

	// If res is nil, it means that the rule hasn't been found
	// TODO: Remove when this deprecated rule is not used anymore
	if res == nil {
		// Try with the deprecated main rule
		if err := executeQuery(deprecatedRule, r.operatingMode == EnvironmentModeRestrictive); err != nil {
			return nil, err
		}

		if res == nil {
			return nil, fmt.Errorf("failed to evaluate policy: neither '%s' nor '%s' rule found", mainRule, deprecatedRule)
		}

		return parseViolationsRule(res, policy)
	}

	return parseResultRule(res, policy)
}

// Parse deprecated list of violations.
// TODO: Remove this path once `result` rule is consolidated
func parseViolationsRule(res rego.ResultSet, policy *engine.Policy) (*engine.EvaluationResult, error) {
	violations := make([]*engine.PolicyViolation, 0)
	for _, exp := range res {
		for _, val := range exp.Expressions {
			ruleResults, ok := val.Value.([]interface{})
			if !ok {
				return nil, engine.ResultFormatError{Field: deprecatedRule}
			}

			for _, result := range ruleResults {
				reasonStr, ok := result.(string)
				if !ok {
					return nil, engine.ResultFormatError{Field: deprecatedRule}
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
		SkipReason: "",
		Ignore:     false, // Assume old rules should not be ignored
	}, nil
}

// parse `result` rule
func parseResultRule(res rego.ResultSet, policy *engine.Policy) (*engine.EvaluationResult, error) {
	result := &engine.EvaluationResult{Violations: make([]*engine.PolicyViolation, 0)}
	for _, exp := range res {
		for _, val := range exp.Expressions {
			ruleResult, ok := val.Value.(map[string]any)
			if !ok {
				return nil, engine.ResultFormatError{Field: mainRule}
			}

			var skipped bool
			if val, ok := ruleResult["skipped"].(bool); ok {
				skipped = val
			}

			var reason string
			if val, ok := ruleResult["skip_reason"].(string); ok {
				reason = val
			}

			var ignore bool
			if val, ok := ruleResult["ignore"].(bool); ok {
				ignore = val
			}

			violations, ok := ruleResult["violations"].([]any)
			if !ok {
				return nil, engine.ResultFormatError{Field: "violations"}
			}

			result.Skipped = skipped
			result.SkipReason = reason
			result.Ignore = ignore

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

// Capabilities returns the capabilities of the environment based on the mode of operation
// defaulting to EnvironmentModeRestrictive if not provided.
func (r *Engine) Capabilities() *ast.Capabilities {
	capabilities := ast.CapabilitiesForThisVersion()
	var enabledBuiltin []*ast.Builtin

	switch r.operatingMode {
	case EnvironmentModeRestrictive:
		// Copy all builtins functions
		localBuiltIns := make(map[string]*ast.Builtin, len(ast.BuiltinMap))
		maps.Copy(localBuiltIns, ast.BuiltinMap)

		// Remove not allowed builtins
		for _, notAllowed := range builtinFuncNotAllowed {
			delete(localBuiltIns, notAllowed.Name)
		}

		// Convert map to slice
		enabledBuiltin = make([]*ast.Builtin, 0, len(localBuiltIns))
		for _, builtin := range localBuiltIns {
			enabledBuiltin = append(enabledBuiltin, builtin)
		}

		// Allow specific network domains
		capabilities.AllowNet = r.allowedNetworkDomains

	case EnvironmentModePermissive:
		enabledBuiltin = capabilities.Builtins
	}

	capabilities.Builtins = enabledBuiltin
	return capabilities
}
