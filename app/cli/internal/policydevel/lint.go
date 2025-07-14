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

package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/open-policy-agent/opa/v1/format"
	"github.com/styrainc/regal/pkg/linter"
	"github.com/styrainc/regal/pkg/rules"
	"gopkg.in/yaml.v3"
)

type Policy struct {
	Path      string
	YAMLFiles []*File
	RegoFiles []*File
	Format    bool
}

type File struct {
	Path    string
	Content []byte
}

type EmbeddedPolicy struct {
	Kind string
	Rego string
}

// Read policy files from the given directory
func Read(absPath string, format bool) (*Policy, error) {
	fileInfo, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", absPath)
	}

	policy := &Policy{
		Path:   absPath,
		Format: format,
	}

	if fileInfo.IsDir() {
		// Read all *.yaml and *.rego files in the directory
		files, err := os.ReadDir(absPath)
		if err != nil {
			return nil, fmt.Errorf("reading directory: %w", err)
		}

		for _, file := range files {
			if file.IsDir() {
				continue
			}

			filePath := filepath.Join(absPath, file.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", file.Name(), err)
			}

			switch strings.ToLower(filepath.Ext(file.Name())) {
			case ".yaml", ".yml":
				policy.YAMLFiles = append(policy.YAMLFiles, &File{
					Path:    filePath,
					Content: content,
				})
			case ".rego":
				policy.RegoFiles = append(policy.RegoFiles, &File{
					Path:    filePath,
					Content: content,
				})
			}

			if len(policy.YAMLFiles) == 0 && len(policy.RegoFiles) == 0 {
				return nil, fmt.Errorf("directory must contain at least one .yaml/.yml or .rego file")
			}
		}
	} else {
		// Read given file
		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("reading file: %w", err)
		}

		switch strings.ToLower(filepath.Ext(absPath)) {
		case ".yaml", ".yml":
			policy.YAMLFiles = append(policy.YAMLFiles, &File{
				Path:    absPath,
				Content: content,
			})
		case ".rego":
			policy.RegoFiles = append(policy.RegoFiles, &File{
				Path:    absPath,
				Content: content,
			})
		default:
			return nil, fmt.Errorf("file must be either .yaml/.yml or .rego")
		}
	}

	return policy, nil
}

func (p *Policy) Validate() []error {
	var allErrors []error

	// Validate all YAML files (including their embedded policies)
	for _, yamlFile := range p.YAMLFiles {
		if yamlErrs := p.validateYAMLFile(yamlFile); len(yamlErrs) > 0 {
			allErrors = append(allErrors, yamlErrs...)
		}
	}

	// Validate standalone rego files
	for _, regoFile := range p.RegoFiles {
		if regoErrs := p.validateRegoFile(regoFile); len(regoErrs) > 0 {
			allErrors = append(allErrors, regoErrs...)
		}
	}

	return allErrors
}

func (p *Policy) validateYAMLFile(file *File) []error {
	var errors []error
	var node yaml.Node

	if err := yaml.Unmarshal(file.Content, &node); err != nil {
		return []error{fmt.Errorf("%s: invalid YAML: %w", file.Path, err)}
	}

	var data map[string]interface{}
	if err := node.Decode(&data); err != nil {
		return []error{fmt.Errorf("%s: invalid YAML structure: %w", file.Path, err)}
	}

	// Validate YAML structure
	if missing := checkRequiredFields(data, []string{"apiVersion", "kind", "spec"}); len(missing) > 0 {
		errors = append(errors, fmt.Errorf("%s: missing required fields: %s", file.Path, strings.Join(missing, ", ")))
	}

	// Process each embedded policy
	if spec, ok := data["spec"].(map[string]interface{}); ok {
		if policies, ok := spec["policies"].([]interface{}); ok {
			for i, policy := range policies {
				if policyMap, ok := policy.(map[string]interface{}); ok {
					if regoContent, ok := policyMap["rego"].(string); ok {
						regoLine := findRegoLineInYAML(&node, i)
						formattedContent, regoErrors := p.validateAndFormatRego(
							regoContent,
							fmt.Sprintf("%s (policy #%d)", file.Path, i+1),
							regoLine,
						)

						errors = append(errors, regoErrors...)

						// Check and apply formatting changes
						if p.Format && formattedContent != regoContent {
							policyMap["rego"] = formattedContent

							updatedContent, err := yaml.Marshal(data)
							if err != nil {
								errors = append(errors, fmt.Errorf("%s: failed to marshal updated YAML: %w", file.Path, err))
							} else if err := os.WriteFile(file.Path, updatedContent, 0600); err != nil {
								errors = append(errors, fmt.Errorf("%s: failed to save formatted file: %w", file.Path, err))
							} else {
								file.Content = updatedContent
							}
						}
					}
				}
			}
		}
	}

	return errors
}

func checkRequiredFields(data map[string]interface{}, fields []string) []string {
	var missing []string
	for _, field := range fields {
		if _, exists := data[field]; !exists {
			missing = append(missing, field)
		}
	}
	return missing
}

func (p *Policy) validateRegoFile(file *File) []error {
	var errors []error
	original := string(file.Content)

	formatted, errs := p.validateAndFormatRego(original, file.Path, 1)
	errors = append(errors, errs...)

	if p.Format && formatted != original {
		if err := os.WriteFile(file.Path, []byte(formatted), 0600); err != nil {
			errors = append(errors, fmt.Errorf("%s: failed to auto-format: %w", file.Path, err))
		} else {
			file.Content = []byte(formatted)
		}
	}

	return errors
}

// Runs the Regal linter on the given rego content and returns any violations as errors
func (p *Policy) runRegalLinter(filePath, content string, lineOffset int) []error {
	inputModules, err := rules.InputFromText(filePath, content)
	if err != nil {
		return []error{err}
	}

	lntr := linter.NewLinter().WithInputModules(&inputModules)
	report, err := lntr.Lint(context.Background())
	if err != nil {
		return []error{err}
	}

	errors := make([]error, 0, len(report.Violations))
	for _, v := range report.Violations {
		errors = append(errors, fmt.Errorf("%d: %s",
			v.Location.Row+lineOffset,
			v.Description,
		))
	}

	return errors
}

func (p *Policy) validateAndFormatRego(content, file string, offset int) (string, []error) {
	var errs []error

	// 1. Optionally format
	if p.Format {
		formatted, err := p.applyOPAFmt(content, file)
		if err != nil {
			errs = append(errs, err)
		} else {
			content = formatted
		}
	}

	// 2. Structural validation
	errs = append(errs, checkResultStructure(content, file, []string{"skipped", "violations", "skip_reason"})...)

	// 3. Run Regal linter and remap OPA fmt lines
	errs = append(errs, p.runLintWithOffset(content, file, offset)...)

	return content, errs
}

func (p *Policy) applyOPAFmt(content, file string) (string, error) {
	formatted, err := format.SourceWithOpts(file, []byte(content), format.Opts{})
	if err != nil {
		// formatting failed, keep original
		return content, fmt.Errorf("%s: Auto-formatting failed", file)
	}
	return string(formatted), nil
}

func checkResultStructure(content, path string, keys []string) []error {
	var errs []error

	// Regex to capture result := { ... } including multiline
	re := regexp.MustCompile(`(?s)result\s*:=\s*\{(.+?)\}`)
	match := re.FindStringSubmatch(content)
	if match == nil {
		return append(errs, fmt.Errorf("%s: no result literal found", path))
	}

	body := match[1]
	// Find quoted keys inside the object literal
	keyRe := regexp.MustCompile(`"([^"]+)"\s*:`)
	found := make(map[string]bool)
	for _, m := range keyRe.FindAllStringSubmatch(body, -1) {
		found[m[1]] = true
	}

	for _, want := range keys {
		if !found[want] {
			errs = append(errs, fmt.Errorf("%s: missing %q key in result", path, want))
		}
	}
	return errs
}

func (p *Policy) runLintWithOffset(content, path string, offset int) []error {
	rawErrs := p.runRegalLinter(path, content, offset)
	return p.remapOPAfmtErrors(rawErrs, path, offset)
}

// Processes raw errors from OPA fmr ran by Regal and:
// 1. Splits grouped errors into individual errors
// 2. Adjusts line numbers for embedded rego scripts
func (p *Policy) remapOPAfmtErrors(rawErrs []error, path string, offset int) []error {
	var errs []error
	// Regex matches file path, line number and error message like: /path/file:line: message
	errorRegex := regexp.MustCompile(`^` + regexp.QuoteMeta(path) + `:(\d+):\s*(.+)$`)

	for _, err := range rawErrs {
		errorStr := err.Error()

		// Split by newlines to handle both single and multi-line errors
		lines := strings.Split(errorStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			// Skip the "N errors occurred" header
			if strings.Contains(line, "errors occurred:") {
				continue
			}

			// Try to match the standard error format
			if matches := errorRegex.FindStringSubmatch(line); len(matches) == 3 {
				if lineNum, convErr := strconv.Atoi(matches[1]); convErr == nil {
					errs = append(errs, fmt.Errorf("%s:%d: %s", path, lineNum+offset, matches[2]))
					continue
				}
			}

			// If we didn't match the standard format, preserve the original error
			errs = append(errs, fmt.Errorf("%s: %s", path, line))
		}
	}

	return errs
}

// Locates beginning line of embedded rego script
func findRegoLineInYAML(doc *yaml.Node, sectionIdx int) int {
	if doc == nil || len(doc.Content) == 0 {
		return 1
	}

	// Navigate through the YAML structure
	root := doc.Content[0]
	spec := findKey(root, "spec")
	if spec == nil {
		return 1
	}

	policies := findKey(spec, "policies")
	if policies == nil || policies.Kind != yaml.SequenceNode || sectionIdx >= len(policies.Content) {
		return 1
	}

	policy := policies.Content[sectionIdx]
	rego := findKey(policy, "rego")
	if rego == nil {
		return 1
	}

	return rego.Line
}

func findKey(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
