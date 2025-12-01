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
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates/*
var templateFS embed.FS

// PolicyType represents the type of policy to initialize
type PolicyType string

const (
	PolicyTypeRego   PolicyType = "rego"
	PolicyTypeWasmGo PolicyType = "wasm-go"
	PolicyTypeWasmJS PolicyType = "wasm-js"
)

const (
	// Rego templates
	regoTemplateDir  = "templates/rego"
	regoPolicyFile   = "example-policy.rego"
	regoYAMLFile     = "example-policy.yaml"

	// WASM Go templates
	wasmGoTemplateDir  = "templates/wasm-go"
	wasmGoPolicyFile   = "policy.go.tmpl"
	wasmGoModFile      = "go.mod.tmpl"
	wasmGoYAMLFile     = "policy.yaml"
	wasmGoMakefileFile = "Makefile"

	// WASM JS templates
	wasmJSTemplateDir = "templates/wasm-js"
	wasmJSPolicyFile  = "policy.js"
	wasmJSPackageFile = "package.json"
	wasmJSEsbuildFile = "esbuild.js"
	wasmJSDTSFile     = "policy.d.ts"
	wasmJSYAMLFile    = "policy.yaml"

	// Defaults
	defaultPolicyName        = "policy"
	defaultPolicyDescription = "Chainloop validation policy"
	defaultMaterialKind      = "SBOM_CYCLONEDX_JSON"
)

type TemplateData struct {
	Name         string
	Description  string
	RegoPath     string
	RegoContent  string
	Embedded     bool
	MaterialKind string
}

type InitOptions struct {
	Directory   string
	PolicyType  PolicyType
	Embedded    bool
	Force       bool
	Name        string
	Description string
}

func Initialize(opts *InitOptions) error {
	// Default to Rego if no type specified
	if opts.PolicyType == "" {
		opts.PolicyType = PolicyTypeRego
	}

	// Route to appropriate initializer based on policy type
	switch opts.PolicyType {
	case PolicyTypeRego:
		return initializeRegoPolicy(opts)
	case PolicyTypeWasmGo:
		return initializeWasmGoPolicy(opts)
	case PolicyTypeWasmJS:
		return initializeWasmJSPolicy(opts)
	default:
		return fmt.Errorf("unsupported policy type: %s", opts.PolicyType)
	}
}

// initializeRegoPolicy creates a Rego-based policy
func initializeRegoPolicy(opts *InitOptions) error {
	// Load templates
	regoContent, err := templateFS.ReadFile(filepath.Join(regoTemplateDir, regoPolicyFile))
	if err != nil {
		return fmt.Errorf("failed to read Rego template: %w", err)
	}

	yamlContent, err := templateFS.ReadFile(filepath.Join(regoTemplateDir, regoYAMLFile))
	if err != nil {
		return fmt.Errorf("failed to read YAML template: %w", err)
	}

	// Prepare template data
	data := &TemplateData{
		Name:         getPolicyName(opts.Name),
		Description:  getPolicyDescription(opts.Description),
		RegoPath:     sanitizeName(getPolicyName(opts.Name)) + ".rego",
		RegoContent:  string(regoContent),
		Embedded:     opts.Embedded,
		MaterialKind: defaultMaterialKind,
	}

	// Process YAML template
	yamlProcessed, err := executeTemplate(string(yamlContent), data)
	if err != nil {
		return fmt.Errorf("failed to process YAML template: %w", err)
	}

	// Prepare files to write
	files := make(map[string]string)
	fileNameBase := sanitizeName(data.Name)

	files[fileNameBase+".yaml"] = yamlProcessed
	if !opts.Embedded {
		files[fileNameBase+".rego"] = data.RegoContent
	}

	return writeFiles(opts.Directory, files, opts.Force)
}

// initializeWasmGoPolicy creates a WASM Go-based policy
func initializeWasmGoPolicy(opts *InitOptions) error {
	// Load templates
	policyContent, err := templateFS.ReadFile(filepath.Join(wasmGoTemplateDir, wasmGoPolicyFile))
	if err != nil {
		return fmt.Errorf("failed to read Go policy template: %w", err)
	}

	goModContent, err := templateFS.ReadFile(filepath.Join(wasmGoTemplateDir, wasmGoModFile))
	if err != nil {
		return fmt.Errorf("failed to read go.mod template: %w", err)
	}

	yamlContent, err := templateFS.ReadFile(filepath.Join(wasmGoTemplateDir, wasmGoYAMLFile))
	if err != nil {
		return fmt.Errorf("failed to read YAML template: %w", err)
	}

	makefileContent, err := templateFS.ReadFile(filepath.Join(wasmGoTemplateDir, wasmGoMakefileFile))
	if err != nil {
		return fmt.Errorf("failed to read Makefile template: %w", err)
	}

	// Prepare template data
	data := &TemplateData{
		Name:         getPolicyName(opts.Name),
		Description:  getPolicyDescription(opts.Description),
		MaterialKind: defaultMaterialKind,
	}

	// Process templates
	policyProcessed, err := executeTemplate(string(policyContent), data)
	if err != nil {
		return fmt.Errorf("failed to process policy template: %w", err)
	}

	goModProcessed, err := executeTemplate(string(goModContent), data)
	if err != nil {
		return fmt.Errorf("failed to process go.mod template: %w", err)
	}

	yamlProcessed, err := executeTemplate(string(yamlContent), data)
	if err != nil {
		return fmt.Errorf("failed to process YAML template: %w", err)
	}

	makefileProcessed, err := executeTemplate(string(makefileContent), data)
	if err != nil {
		return fmt.Errorf("failed to process Makefile template: %w", err)
	}

	// Prepare files to write
	files := map[string]string{
		"policy.go":   policyProcessed,
		"go.mod":      goModProcessed,
		"policy.yaml": yamlProcessed,
		"Makefile":    makefileProcessed,
	}

	return writeFiles(opts.Directory, files, opts.Force)
}

// initializeWasmJSPolicy creates a WASM JavaScript-based policy
func initializeWasmJSPolicy(opts *InitOptions) error {
	// Load templates
	policyContent, err := templateFS.ReadFile(filepath.Join(wasmJSTemplateDir, wasmJSPolicyFile))
	if err != nil {
		return fmt.Errorf("failed to read JS policy template: %w", err)
	}

	packageContent, err := templateFS.ReadFile(filepath.Join(wasmJSTemplateDir, wasmJSPackageFile))
	if err != nil {
		return fmt.Errorf("failed to read package.json template: %w", err)
	}

	esbuildContent, err := templateFS.ReadFile(filepath.Join(wasmJSTemplateDir, wasmJSEsbuildFile))
	if err != nil {
		return fmt.Errorf("failed to read esbuild.js template: %w", err)
	}

	dtsContent, err := templateFS.ReadFile(filepath.Join(wasmJSTemplateDir, wasmJSDTSFile))
	if err != nil {
		return fmt.Errorf("failed to read policy.d.ts template: %w", err)
	}

	yamlContent, err := templateFS.ReadFile(filepath.Join(wasmJSTemplateDir, wasmJSYAMLFile))
	if err != nil {
		return fmt.Errorf("failed to read YAML template: %w", err)
	}

	// Prepare template data
	data := &TemplateData{
		Name:         getPolicyName(opts.Name),
		Description:  getPolicyDescription(opts.Description),
		MaterialKind: defaultMaterialKind,
	}

	// Process templates
	policyProcessed, err := executeTemplate(string(policyContent), data)
	if err != nil {
		return fmt.Errorf("failed to process policy template: %w", err)
	}

	packageProcessed, err := executeTemplate(string(packageContent), data)
	if err != nil {
		return fmt.Errorf("failed to process package.json template: %w", err)
	}

	dtsProcessed, err := executeTemplate(string(dtsContent), data)
	if err != nil {
		return fmt.Errorf("failed to process policy.d.ts template: %w", err)
	}

	yamlProcessed, err := executeTemplate(string(yamlContent), data)
	if err != nil {
		return fmt.Errorf("failed to process YAML template: %w", err)
	}

	// Prepare files to write (esbuild.js doesn't need template processing)
	files := map[string]string{
		"policy.js":    policyProcessed,
		"package.json": packageProcessed,
		"esbuild.js":   string(esbuildContent),
		"policy.d.ts":  dtsProcessed,
		"policy.yaml":  yamlProcessed,
	}

	return writeFiles(opts.Directory, files, opts.Force)
}

func getPolicyName(name string) string {
	if name == "" {
		return defaultPolicyName
	}
	return name
}

func getPolicyDescription(description string) string {
	if description == "" {
		return defaultPolicyDescription
	}
	return description
}

// executeTemplate processes a template with the given data
func executeTemplate(content string, data *TemplateData) (string, error) {
	tmpl := template.New("policy").Funcs(template.FuncMap{
		"sanitize":  sanitizeName,
		"trimSpace": strings.TrimSpace,
		"indent": func(spaces int, s string) string {
			pad := strings.Repeat(" ", spaces)
			return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
		},
	})

	tmpl, err := tmpl.Parse(content)
	if err != nil {
		return "", fmt.Errorf("template parsing error: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution error: %w", err)
	}

	return buf.String(), nil
}

func sanitizeName(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "-"))
}

func writeFiles(dir string, files map[string]string, force bool) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	for filename, content := range files {
		path := filepath.Join(dir, filename)
		if !force && fileExists(path) {
			return fmt.Errorf("file %s already exists (use --force to overwrite)", path)
		}

		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filename, err)
		}
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
