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
	"github.com/open-policy-agent/opa/v1/topdown/print"
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
	// includeRawData determines whether to collect raw evaluation data
	includeRawData bool
	// enablePrint determines whether to enable print statements in rego policies
	enablePrint bool
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

func WithIncludeRawData(include bool) EngineOption {
	return func(e *newEngineOptions) {
		e.includeRawData = include
	}
}

func WithEnablePrint(enable bool) EngineOption {
	return func(e *newEngineOptions) {
		e.enablePrint = enable
	}
}

type newEngineOptions struct {
	operatingMode         EnvironmentMode
	allowedNetworkDomains []string
	includeRawData        bool
	enablePrint           bool
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
		includeRawData:        options.includeRawData,
		enablePrint:           options.enablePrint,
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
	expectedArgs                              = "expected_args"
	evalResult                                = "evaluation_result"
	inputElements                             = "elements"
	mainRule                                  = "result"
	matchesParametersRule                     = "matches_parameters"
	matchesEvaluationRule                     = "matches_evaluation"
)

// builtinFuncNotAllowed is a list of builtin functions that are not allowed in the compiler
var builtinFuncNotAllowed = []*ast.Builtin{
	ast.OPARuntime,
	ast.RegoParseModule,
	ast.Trace,
}

// Implements the OPA print.Hook interface to capture and output
// print statements from Rego policies during evaluation.
type regoOutputHook struct{}

func (p *regoOutputHook) Print(_ print.Context, msg string) error { //nolint:forbidigo
	fmt.Println(msg)
	return nil
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
		options := []func(r *rego.Rego){regoInput, regoFunc, rego.Capabilities(r.Capabilities())}

		// Add print support if enabled
		if r.enablePrint {
			options = append(options,
				rego.EnablePrintStatements(true),
				rego.PrintHook(&regoOutputHook{}),
			)
		}

		if strict {
			options = append(options, rego.StrictBuiltinErrors(true))
		}

		res, err = queryRego(ctx, rule, options...)
		return err
	}

	var rawData *engine.RawData
	// Get raw results first if requested
	if r.includeRawData {
		if err := executeQuery(getRuleName(parsedModule.Package.Path, ""), r.operatingMode == EnvironmentModeRestrictive); err != nil {
			return nil, err
		}

		inputBytes, err := json.Marshal(decodedInput)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal input for raw data: %w", err)
		}
		outputBytes, err := json.Marshal(regoResultSetToRawResults(res))
		if err != nil {
			return nil, fmt.Errorf("failed to marshal output for raw data: %w", err)
		}
		rawData = &engine.RawData{
			Input:  json.RawMessage(inputBytes),
			Output: json.RawMessage(outputBytes),
		}
	}

	// Try the main rule
	if err := executeQuery(getRuleName(parsedModule.Package.Path, mainRule), r.operatingMode == EnvironmentModeRestrictive); err != nil {
		return nil, err
	}

	if res == nil {
		return nil, fmt.Errorf("failed to evaluate policy: '%s' rule not found", mainRule)
	}

	return parseResultRule(res, policy, rawData)
}

// parse `result` rule
func parseResultRule(res rego.ResultSet, policy *engine.Policy, rawData *engine.RawData) (*engine.EvaluationResult, error) {
	result := &engine.EvaluationResult{Violations: make([]*engine.PolicyViolation, 0)}
	result.RawData = rawData
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

func queryRego(ctx context.Context, fullRuleName string, options ...func(r *rego.Rego)) (rego.ResultSet, error) {
	query := rego.Query(fullRuleName)
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

func regoResultSetToRawResults(res rego.ResultSet) map[string]interface{} {
	raw := make(map[string]interface{})
	for _, r := range res {
		entry := make(map[string]interface{})
		for _, exp := range r.Expressions {
			entry[exp.Text] = exp.Value
		}
		raw = entry
	}
	return raw
}

func getRuleName(packagePath ast.Ref, rule string) string {
	if rule == "" {
		return fmt.Sprintf("%s\n", packagePath)
	}
	return fmt.Sprintf("%v.%s\n", packagePath, rule)
}

// MatchesParameters evaluates the matches_parameters rule in a rego policy.
// The function creates an input object with policy parameters and expected parameters.
// Returns true if the policy's matches_parameters rule evaluates to true, false otherwise.
func (r *Engine) MatchesParameters(ctx context.Context, policy *engine.Policy, evaluationParams, expectedParams map[string]string) (bool, error) {
	policyString := string(policy.Source)
	parsedModule, err := ast.ParseModule(policy.Name, policyString)
	if err != nil {
		return false, fmt.Errorf("failed to parse rego policy: %w", err)
	}

	// Create input with policy and expected parameters
	inputMap := make(map[string]interface{})
	inputMap[inputArgs] = evaluationParams
	inputMap[expectedArgs] = expectedParams

	// Evaluate matches_parameters rule
	matchesParameters, err := r.evaluateMatchingRule(ctx, getRuleName(parsedModule.Package.Path, matchesParametersRule), parsedModule, inputMap)
	if err != nil {
		// Defaults to false
		return false, err
	}

	return matchesParameters, nil
}

// MatchesEvaluation evaluates the matches_evaluation rule in a rego policy.
// The function creates an input object with policy parameters and evaluation result.
// Returns true if the policy's matches_evaluation rule evaluates to true, false otherwise.
// If the rule is not found or evaluation fails, it defaults to false.
func (r *Engine) MatchesEvaluation(ctx context.Context, policy *engine.Policy, ev *engine.EvaluationResult, evaluationParams map[string]string) (bool, error) {
	policyString := string(policy.Source)
	parsedModule, err := ast.ParseModule(policy.Name, policyString)
	if err != nil {
		return false, fmt.Errorf("failed to parse rego policy: %w", err)
	}

	// Create input with the policy evaluation data
	inputMap := make(map[string]interface{})
	inputMap[inputArgs] = evaluationParams
	inputMap[evalResult] = ev

	// Evaluate matches_parameters rule
	matchesEvaluation, err := r.evaluateMatchingRule(ctx, getRuleName(parsedModule.Package.Path, matchesEvaluationRule), parsedModule, inputMap)
	if err != nil {
		// Defaults to false
		return false, err
	}

	return matchesEvaluation, nil
}

// Evaluates a single rule and returns its boolean result
func (r *Engine) evaluateMatchingRule(ctx context.Context, ruleName string, parsedModule *ast.Module, decodedInput interface{}) (bool, error) {
	// Add input
	regoInput := rego.Input(decodedInput)

	// Add module
	regoFunc := rego.ParsedModule(parsedModule)
	options := []func(r *rego.Rego){regoInput, regoFunc, rego.Capabilities(r.Capabilities())}

	if r.operatingMode == EnvironmentModeRestrictive {
		options = append(options, rego.StrictBuiltinErrors(true))
	}

	res, err := queryRego(ctx, ruleName, options...)
	if err != nil {
		return false, err
	}

	// Parse the boolean result
	for _, exp := range res {
		for _, val := range exp.Expressions {
			if boolResult, ok := val.Value.(bool); ok {
				return boolResult, nil
			}
		}
	}

	// No valid boolean result found
	return false, nil
}
