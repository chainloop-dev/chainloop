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

package policydevel

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	"github.com/chainloop-dev/chainloop/pkg/policies/engine"
	"github.com/chainloop-dev/chainloop/pkg/resourceloader"
	extism "github.com/extism/go-sdk"
	opaAst "github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/format"
	"github.com/styrainc/regal/pkg/config"
	"github.com/styrainc/regal/pkg/linter"
	"github.com/styrainc/regal/pkg/report"
	"github.com/styrainc/regal/pkg/rules"
	"gopkg.in/yaml.v3"
)

//go:embed .regal.yaml
var regalConfigFS embed.FS

type PolicyToLint struct {
	Path      string
	YAMLFiles []*File
	RegoFiles []*File
	WASMFiles []*File
	Format    bool
	Config    string
	Errors    []ValidationError
}

type File struct {
	Path    string
	Content []byte
}

type ValidationError struct {
	Path    string
	Line    int
	Message string
}

func (e ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.Path, e.Line, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// Returns true if any validation errors were found
func (p *PolicyToLint) HasErrors() bool {
	return len(p.Errors) > 0
}

// Adds a new validation error
func (p *PolicyToLint) AddError(path, message string, line int) {
	p.Errors = append(p.Errors, ValidationError{
		Path:    path,
		Message: message,
		Line:    line,
	})
}

// Read policy files
func Lookup(absPath, config string, format bool) (*PolicyToLint, error) {
	resolvedPath, err := resourceloader.GetPathForResource(absPath)
	if err != nil {
		return nil, err
	}

	fileInfo, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("policy file does not exist: %s", resolvedPath)
		}
		return nil, err
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("expected a file but got a directory: %s", resolvedPath)
	}

	policy := &PolicyToLint{
		Path:   resolvedPath,
		Format: format,
		Config: config,
	}

	if err := policy.processFile(resolvedPath); err != nil {
		return nil, err
	}

	// Load referenced policy files (rego or wasm) from all YAML files
	if err := policy.loadReferencedPolicyFiles(filepath.Dir(resolvedPath)); err != nil {
		return nil, err
	}

	// Verify we found at least one valid file
	if len(policy.YAMLFiles) == 0 && len(policy.RegoFiles) == 0 && len(policy.WASMFiles) == 0 {
		return nil, fmt.Errorf("no valid .yaml/.yml, .rego, or .wasm files found")
	}

	return policy, nil
}

// Loads referenced policy files (rego or wasm) from YAML files in the policy
func (p *PolicyToLint) loadReferencedPolicyFiles(baseDir string) error {
	seen := make(map[string]struct{})
	for _, yamlFile := range p.YAMLFiles {
		var parsed v1.Policy
		if err := unmarshal.FromRaw(yamlFile.Content, unmarshal.RawFormatYAML, &parsed, true); err != nil {
			p.AddError(yamlFile.Path, err.Error(), 0)
			continue
		}
		for _, spec := range parsed.Spec.Policies {
			policyPath := spec.GetPath()
			if policyPath != "" {
				// If path is relative, make it relative to the YAML file's directory
				if !filepath.IsAbs(policyPath) {
					policyPath = filepath.Join(baseDir, policyPath)
				}

				resolvedPath, err := resourceloader.GetPathForResource(policyPath)
				if err != nil {
					return err
				}
				if _, ok := seen[resolvedPath]; ok {
					continue // avoid duplicates
				}
				seen[resolvedPath] = struct{}{}
				if err := p.processFile(resolvedPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (p *PolicyToLint) processFile(filePath string) error {
	ext := strings.ToLower(filepath.Ext(filePath))

	// WASM files: validate magic bytes
	if ext == ".wasm" {
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read WASM file %s: %w", filePath, err)
		}

		// Verify magic bytes
		if engine.DetectPolicyType(content) != engine.PolicyTypeWASM {
			return fmt.Errorf("file has .wasm extension but is not a valid WASM file")
		}

		p.WASMFiles = append(p.WASMFiles, &File{Path: filePath, Content: content})
		return nil
	}

	// Other files: read full content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	switch ext {
	case ".yaml", ".yml":
		p.YAMLFiles = append(p.YAMLFiles, &File{
			Path:    filePath,
			Content: content,
		})
	case ".rego":
		p.RegoFiles = append(p.RegoFiles, &File{
			Path:    filePath,
			Content: content,
		})
	default:
		return fmt.Errorf("unsupported file extension %s, must be .yaml/.yml, .rego, or .wasm", ext)
	}

	return nil
}

func (p *PolicyToLint) Validate() {
	// Validate standalone rego files
	for _, regoFile := range p.RegoFiles {
		p.validateRegoFile(regoFile)
	}

	// Validate WASM files
	for _, wasmFile := range p.WASMFiles {
		p.validateWasmFile(wasmFile)
	}
}

func (p *PolicyToLint) validateRegoFile(file *File) {
	original := string(file.Content)
	formatted := p.validateAndFormatRego(original, file.Path)

	if p.Format && formatted != original {
		if err := os.WriteFile(file.Path, []byte(formatted), 0600); err != nil {
			p.AddError(file.Path, err.Error(), 0)
		} else {
			file.Content = []byte(formatted)
		}
	}
}

// validateWasmFile validates a WASM policy file by checking that it exports the required Execute function
func (p *PolicyToLint) validateWasmFile(file *File) {
	ctx := context.Background()

	// Create Extism manifest
	manifest := extism.Manifest{
		Wasm: []extism.Wasm{
			extism.WasmData{Data: file.Content},
		},
	}

	cfg := extism.PluginConfig{
		EnableWasi: true,
	}

	// Create plugin
	plugin, err := extism.NewPlugin(ctx, manifest, cfg, []extism.HostFunction{})
	if err != nil {
		p.AddError(file.Path, fmt.Sprintf("failed to load WASM module: %v", err), 0)
		return
	}
	defer plugin.Close(ctx)

	// Check if Execute function is exported
	if !plugin.FunctionExists("Execute") {
		p.AddError(file.Path, "WASM module missing required 'Execute' function export", 0)
	}
}

func (p *PolicyToLint) validateAndFormatRego(content, path string) string {
	// 1. Optionally format
	if p.Format {
		formatted := p.applyOPAFmt(content, path)
		content = formatted
	}

	// 2. Run Regal linter
	p.runRegalLinter(path, content)

	return content
}

func (p *PolicyToLint) applyOPAFmt(content, file string) string {
	formatted, err := format.SourceWithOpts(file, []byte(content), format.Opts{})
	if err != nil {
		p.AddError(file, "auto-formatting failed", 0)
		return content
	}
	return string(formatted)
}

// Runs the Regal linter on the given rego content and records any violations
func (p *PolicyToLint) runRegalLinter(filePath, content string) {
	inputModules, err := rules.InputFromText(filePath, content)
	if err != nil {
		// Cast to OPA AST errors for better formatting
		var astErrs opaAst.Errors
		if errors.As(err, &astErrs) {
			for _, e := range astErrs {
				line := 0
				if e.Location != nil {
					line = e.Location.Row
				}

				p.AddError(filePath, e.Message, line)
			}
			return
		}
		// Fallback if it's not an ast.Errors type
		p.AddError(filePath, err.Error(), 0)
		return
	}

	// Initialize linter with input modules
	lntr := linter.NewLinter().WithInputModules(&inputModules)

	// Load and apply configuration
	cfg, err := p.loadConfig()
	if err != nil {
		p.AddError(filePath, fmt.Sprintf("%s", err), 0)
	}
	if cfg != nil {
		lntr = lntr.WithUserConfig(*cfg)
	}

	report, err := lntr.Lint(context.Background())
	if err != nil {
		p.AddError(filePath, err.Error(), 0)
		return
	}

	// Parse the Rego AST to map line numbers to rule names
	regoRuleMap := p.buildRegoRuleMap(content)

	// Add violations to the policy errors
	for _, v := range report.Violations {
		errorStr := p.formatViolationError(v, regoRuleMap)
		p.AddError(filePath, errorStr, v.Location.Row)
	}
}

// Creates a formatted error message from a Regal violation
// Follows format <file>:<line>: [<ruleName>] <errorMsg> - <docLinks>
func (p *PolicyToLint) formatViolationError(v report.Violation, regoRuleMap map[int]string) string {
	// Extract resources
	resources := make([]string, 0, len(v.RelatedResources))
	for _, r := range v.RelatedResources {
		resources = append(resources, r.Reference)
	}
	resourceStr := strings.Join(resources, ", ")

	// Try to identify which Rego rule contains this violation
	regoRuleName, exists := regoRuleMap[v.Location.Row]
	if !exists {
		regoRuleName = ""
	} else {
		regoRuleName = fmt.Sprintf("[%s]", regoRuleName)
	}

	// Format the error message
	lintError := fmt.Sprintf("%s: %s - %s", regoRuleName, v.Description, resourceStr)
	return strings.ReplaceAll(lintError, "`opa fmt`", "`--format`")
}

// Attempts to load configuration in this order:
// 1. User-specified config
// 2. Default config
// Returns nil config if no config found at all
func (p *PolicyToLint) loadConfig() (*config.Config, error) {
	// 1. Try user-specified config first
	if p.Config != "" {
		userCfg, err := config.FromPath(p.Config)
		if err == nil {
			return &userCfg, nil
		}
		// If user config fails, we'll fall through to default config
		userErr := fmt.Errorf("failed to load user config from %q: %w (using default config)", p.Config, err)

		// Continue to try default config
		defaultCfg, defaultErr := p.loadDefaultConfig()
		if defaultErr == nil {
			return defaultCfg, userErr
		}
		return nil, fmt.Errorf("%w; also %w", userErr, defaultErr)
	}

	// 2. No user config specified - try default config
	return p.loadDefaultConfig()
}

func (p *PolicyToLint) loadDefaultConfig() (*config.Config, error) {
	cfgData, err := regalConfigFS.ReadFile(".regal.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read default config: %w", err)
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(cfgData, &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse default config: %w", err)
	}

	cfg, err := config.FromMap(configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to convert default config: %w", err)
	}

	return &cfg, nil
}

// Creates a mapping from line numbers to Rego rule names
func (p *PolicyToLint) buildRegoRuleMap(regoSrc string) map[int]string {
	ruleMap := make(map[int]string)

	// Parse the Rego source into AST
	module, err := opaAst.ParseModule("", regoSrc)
	if err != nil {
		// Return empty map if parsing fails
		return ruleMap
	}

	// Walk through the AST to find rule definitions
	for _, rule := range module.Rules {
		if rule.Location != nil {
			ruleName := string(rule.Head.Name)
			startLine := rule.Location.Row
			endLine := startLine

			// Try to find the end line of the rule
			if len(rule.Body) > 0 {
				lastExpr := rule.Body[len(rule.Body)-1]
				if lastExpr.Location != nil {
					endLine = lastExpr.Location.Row
				}
			}

			// Map all lines within this rule to the rule name
			for line := startLine; line <= endLine; line++ {
				ruleMap[line] = ruleName
			}
		}
	}

	return ruleMap
}
