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

package rego

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"github.com/open-policy-agent/opa/v1/ast"
)

const (
	ruleResult     = "result"
	ruleSkipped    = "skipped"
	ruleSkipReason = "skip_reason"
	ruleValidInput = "valid_input"
	ruleViolations = "violations"
	ruleIgnore     = "ignore"
)

//go:embed boilerplate.rego.tmpl
var boilerplateTemplate string

type boilerplateData struct {
	NeedsResult            bool
	NeedsDefaultSkipReason bool
	NeedsSkipReasonRule    bool
	NeedsDefaultSkipped    bool
	NeedsSkippedRule       bool
	NeedsDefaultIgnore     bool
	NeedsDefaultValidInput bool
	NeedsDefaultViolations bool
}

// InjectBoilerplate automatically injects common policy boilerplate if it doesn't exist.
// This allows users to write simplified policies with only the violations rules.
// Requirements: Policy must have package declaration and import rego.v1
// The function:
// - Parses the policy using OPA's AST
// - Detects which boilerplate rules are missing
// - Injects only the missing rules after package and imports
func InjectBoilerplate(policySource []byte, policyName string) ([]byte, error) {
	if len(policySource) == 0 {
		return nil, fmt.Errorf("empty policy source")
	}

	originalPolicy := string(policySource)

	// Parse the policy
	module, err := ast.ParseModule(policyName, originalPolicy)
	if err != nil {
		return nil, fmt.Errorf("failed to parse policy (must have 'package' and 'import rego.v1'): %w", err)
	}

	// Detect which rules already exist using AST
	existing := detectExistingRules(module)

	// If all required boilerplate rules and defaults exist, no injection needed
	if existing.hasRule[ruleResult] &&
		existing.hasDefault[ruleSkipReason] && existing.hasRule[ruleSkipReason] &&
		existing.hasDefault[ruleSkipped] && existing.hasRule[ruleSkipped] &&
		existing.hasDefault[ruleIgnore] &&
		existing.hasDefault[ruleValidInput] &&
		existing.hasDefault[ruleViolations] {
		return policySource, nil
	}

	// Build the boilerplate injection (rules only, no package/import)
	injection, err := buildBoilerplate(existing)
	if err != nil {
		return nil, err
	}

	// If nothing needs to be injected, return original
	if injection == "" {
		return policySource, nil
	}

	// Inject after package and imports
	injected, err := injectAfterImports(module, originalPolicy, injection)
	if err != nil {
		return nil, fmt.Errorf("failed to inject boilerplate: %w", err)
	}

	return []byte(injected), nil
}

type existingRules struct {
	hasRule    map[string]bool
	hasDefault map[string]bool
}

// detectExistingRules scans the AST to find which rules are already defined
func detectExistingRules(module *ast.Module) *existingRules {
	rules := &existingRules{
		hasRule:    make(map[string]bool),
		hasDefault: make(map[string]bool),
	}

	for _, rule := range module.Rules {
		ruleName := string(rule.Head.Name)
		rules.hasRule[ruleName] = true

		// Track if this is a default rule
		if rule.Default {
			rules.hasDefault[ruleName] = true
		}
	}

	return rules
}

// buildBoilerplate constructs the boilerplate template based on what's missing
func buildBoilerplate(rules *existingRules) (string, error) {
	data := boilerplateData{
		NeedsResult:            !rules.hasRule[ruleResult],
		NeedsDefaultSkipReason: !rules.hasDefault[ruleSkipReason] && !rules.hasRule[ruleSkipReason],
		NeedsSkipReasonRule:    !rules.hasRule[ruleSkipReason],
		NeedsDefaultSkipped:    !rules.hasDefault[ruleSkipped] && !rules.hasRule[ruleSkipped],
		NeedsSkippedRule:       !rules.hasRule[ruleSkipped],
		NeedsDefaultIgnore:     !rules.hasDefault[ruleIgnore] && !rules.hasRule[ruleIgnore],
		NeedsDefaultValidInput: !rules.hasDefault[ruleValidInput] && !rules.hasRule[ruleValidInput],
		NeedsDefaultViolations: !rules.hasDefault[ruleViolations] && !rules.hasRule[ruleViolations],
	}

	tmpl, err := template.New("boilerplate").Parse(boilerplateTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse boilerplate template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute boilerplate template: %w", err)
	}

	return buf.String(), nil
}

// injectAfterImports inserts the injection block after the package declaration and existing imports
func injectAfterImports(module *ast.Module, originalPolicy, injection string) (string, error) {
	// Get insertion line from AST - start with package line
	insertionLine := module.Package.Location.Row

	// Find the last import line
	for _, imp := range module.Imports {
		if imp.Location.Row > insertionLine {
			insertionLine = imp.Location.Row
		}
	}

	// Skip any blank lines after package/imports
	lines := strings.Split(originalPolicy, "\n")
	for insertionLine < len(lines) && strings.TrimSpace(lines[insertionLine]) == "" {
		insertionLine++
	}

	// Trim trailing newline from injection to avoid double blank line when joining
	injection = strings.TrimSuffix(injection, "\n")

	// Insert the injection block
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:insertionLine]...)
	result = append(result, injection)
	result = append(result, lines[insertionLine:]...)

	return strings.Join(result, "\n"), nil
}
