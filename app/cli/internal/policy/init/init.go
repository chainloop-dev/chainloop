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

package policy

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

const (
	policyTemplateRegoPath   = "templates/example-policy.rego"
	policyTemplatePath       = "templates/example-policy.yaml"
	defaultPolicyName        = "chainloop-policy"
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

type Content struct {
	YAML string
	Rego string
}

type InitOptions struct {
	Dir         string
	Embedded    bool
	Force       bool
	Name        string
	Description string
}

func Initialize(opts *InitOptions) error {
	content, err := loadAndProcessTemplates(opts)
	if err != nil {
		return fmt.Errorf("failed to process templates: %w", err)
	}

	files := make(map[string]string)
	fileNameBase := sanitizeName(getPolicyName(opts.Name))

	if opts.Embedded {
		files[fileNameBase+".yaml"] = content.YAML
	} else {
		files[fileNameBase+".yaml"] = content.YAML
		files[fileNameBase+".rego"] = content.Rego
	}

	return writeFiles(opts.Dir, files, opts.Force)
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

func loadAndProcessTemplates(opts *InitOptions) (*Content, error) {
	regoContent, err := templateFS.ReadFile(policyTemplateRegoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Rego template: %w", err)
	}

	data := &TemplateData{
		Name:         getPolicyName(opts.Name),
		Description:  getPolicyDescription(opts.Description),
		RegoPath:     sanitizeName(getPolicyName(opts.Name)) + ".rego",
		RegoContent:  string(regoContent),
		Embedded:     opts.Embedded,
		MaterialKind: defaultMaterialKind,
	}

	// Process main template
	content, err := templateFS.ReadFile(policyTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy template: %w", err)
	}

	yamlContent, err := executeTemplate(string(content), data)
	if err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	// For non-embedded case, we still need the Rego content to write to file
	if !opts.Embedded {
		return &Content{
			YAML: yamlContent,
			Rego: data.RegoContent,
		}, nil
	}

	return &Content{YAML: yamlContent}, nil
}

// Add custom template functions
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
