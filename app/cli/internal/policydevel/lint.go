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

package policydevel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bufbuild/protoyaml-go"
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/unmarshal"
	"github.com/open-policy-agent/opa/v1/format"
	"github.com/styrainc/regal/pkg/linter"
	"github.com/styrainc/regal/pkg/rules"
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

// Read policy files from the given directory or file
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
		if err := readDirectory(policy, absPath); err != nil {
			return nil, err
		}
	} else {
		if err := readSingleFile(policy, absPath); err != nil {
			return nil, err
		}
	}

	// Verify we found at least one valid file
	if len(policy.YAMLFiles) == 0 && len(policy.RegoFiles) == 0 {
		return nil, fmt.Errorf("no valid .yaml/.yml or .rego files found")
	}

	return policy, nil
}

func readDirectory(policy *Policy, dirPath string) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	var foundValidFile bool
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(dirPath, file.Name())
		if err := processFile(policy, filePath); err != nil {
			// Skip unsupported files but continue processing others
			continue
		}
		foundValidFile = true
	}

	if !foundValidFile {
		return fmt.Errorf("no valid .yaml/.yml or .rego files found in directory")
	}

	return nil
}

func readSingleFile(policy *Policy, filePath string) error {
	return processFile(policy, filePath)
}

func processFile(policy *Policy, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filepath.Base(filePath), err)
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
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
	default:
		return fmt.Errorf("unsupported file extension %s, must be .yaml/.yml or .rego", ext)
	}

	return nil
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
	var policy v1.Policy
	if err := unmarshal.FromRaw(file.Content, unmarshal.RawFormatYAML, &policy, true); err != nil {
		return []error{fmt.Errorf("%s: failed to parse/validate: %w", file.Path, err)}
	}

	errs := p.processEmbeddedPolicies(&policy, file)

	// Update policy file with formatted content
	if p.Format {
		outYAML, err := protoyaml.Marshal(&policy)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: failed to marshal updated YAML: %w", file.Path, err))
		} else if err := os.WriteFile(file.Path, outYAML, 0600); err != nil {
			errs = append(errs, fmt.Errorf("%s: failed to save updated file: %w", file.Path, err))
		} else {
			file.Content = outYAML
		}
	}

	return errs
}

func (p *Policy) processEmbeddedPolicies(pa *v1.Policy, file *File) []error {
	var errs []error

	for idx, spec := range pa.Spec.Policies {
		if regoSrc := spec.GetEmbedded(); regoSrc != "" {
			formatted, reErrs := p.validateAndFormatRego(
				regoSrc,
				fmt.Sprintf("%s:(embedded #%d)", file.Path, idx+1),
			)
			errs = append(errs, reErrs...)

			if p.Format && formatted != regoSrc {
				spec.Source = &v1.PolicySpecV2_Embedded{Embedded: formatted}
			}
		}
	}

	return errs
}

func (p *Policy) validateRegoFile(file *File) []error {
	var errors []error
	original := string(file.Content)

	formatted, errs := p.validateAndFormatRego(original, file.Path)
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
func (p *Policy) runRegalLinter(filePath, content string) []error {
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
			v.Location.Row,
			v.Description,
		))
	}

	return errors
}

func (p *Policy) validateAndFormatRego(content, path string) (string, []error) {
	var errs []error

	// 1. Optionally format
	if p.Format {
		formatted, err := p.applyOPAFmt(content, path)
		if err != nil {
			errs = append(errs, err)
		} else {
			content = formatted
		}
	}

	// 2. Structural validation
	errs = append(errs, checkResultStructure(content, path, []string{"skipped", "violations", "skip_reason"})...)

	// 3. Run Regal linter
	rawErrs := p.runRegalLinter(path, content)

	errs = append(errs, p.remapOPAfmtErrors(rawErrs, path)...)

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

// Processes raw errors from OPA fmt ran by Regal and:
// 1. Splits grouped errors into individual errors
// 2. Adjusts line numbers for embedded rego scripts
func (p *Policy) remapOPAfmtErrors(rawErrs []error, path string) []error {
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
					errs = append(errs, fmt.Errorf("%s:%d: %s", path, lineNum, matches[2]))
					continue
				}
			}

			// If we didn't match the standard format, preserve the original error
			errs = append(errs, fmt.Errorf("%s: %s", path, line))
		}
	}

	return errs
}
