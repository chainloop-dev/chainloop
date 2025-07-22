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
	"embed"
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
	"github.com/styrainc/regal/pkg/config"
	"github.com/styrainc/regal/pkg/linter"
	"github.com/styrainc/regal/pkg/rules"
	"gopkg.in/yaml.v2"
)

//go:embed .regal.yaml
var regalConfigFS embed.FS

type PolicyToLint struct {
	Path      string
	YAMLFiles []*File
	RegoFiles []*File
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

// Read policy files from the given directory or file
func Lookup(absPath, config string, format bool) (*PolicyToLint, error) {
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("path does not exist: %s", absPath)
		}
		return nil, fmt.Errorf("failed to stat path %q: %w", absPath, err)
	}

	policy := &PolicyToLint{
		Path:   absPath,
		Format: format,
		Config: config,
	}

	if fileInfo.IsDir() {
		if err := scanDirectory(policy, absPath); err != nil {
			return nil, err
		}
	} else {
		if err := processFile(policy, absPath); err != nil {
			return nil, err
		}
	}

	// Verify we found at least one valid file
	if len(policy.YAMLFiles) == 0 && len(policy.RegoFiles) == 0 {
		return nil, fmt.Errorf("no valid .yaml/.yml or .rego files found")
	}

	return policy, nil
}

// Performs a one-level directory lookup to find .yaml/.yml or .rego files.
func scanDirectory(policy *PolicyToLint, dirPath string) error {
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

func processFile(policy *PolicyToLint, filePath string) error {
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

func (p *PolicyToLint) Validate() {
	// Validate all YAML files (including their embedded policies)
	for _, yamlFile := range p.YAMLFiles {
		p.validateYAMLFile(yamlFile)
	}

	// Validate standalone rego files
	for _, regoFile := range p.RegoFiles {
		p.validateRegoFile(regoFile)
	}
}

func (p *PolicyToLint) validateYAMLFile(file *File) {
	var policy v1.Policy
	if err := unmarshal.FromRaw(file.Content, unmarshal.RawFormatYAML, &policy, true); err != nil {
		p.AddError(file.Path, fmt.Sprintf("failed to parse/validate: %v", err), 0)
		return
	}

	p.processEmbeddedPolicies(&policy, file)

	// Update policy file with formatted content
	if p.Format {
		outYAML, err := protoyaml.Marshal(&policy)
		if err != nil {
			p.AddError(file.Path, fmt.Sprintf("failed to marshal updated YAML: %v", err), 0)
		} else if err := os.WriteFile(file.Path, outYAML, 0600); err != nil {
			p.AddError(file.Path, fmt.Sprintf("failed to save updated file: %v", err), 0)
		} else {
			file.Content = outYAML
		}
	}
}

func (p *PolicyToLint) processEmbeddedPolicies(pa *v1.Policy, file *File) {
	for idx, spec := range pa.Spec.Policies {
		if regoSrc := spec.GetEmbedded(); regoSrc != "" {
			formatted := p.validateAndFormatRego(
				regoSrc,
				fmt.Sprintf("%s:(embedded #%d)", file.Path, idx+1),
			)

			if p.Format && formatted != regoSrc {
				spec.Source = &v1.PolicySpecV2_Embedded{Embedded: formatted}
			}
		}
	}
}

func (p *PolicyToLint) validateRegoFile(file *File) {
	original := string(file.Content)
	formatted := p.validateAndFormatRego(original, file.Path)

	if p.Format && formatted != original {
		if err := os.WriteFile(file.Path, []byte(formatted), 0600); err != nil {
			p.AddError(file.Path, fmt.Sprintf("failed to auto-format: %v", err), 0)
		} else {
			file.Content = []byte(formatted)
		}
	}
}

func (p *PolicyToLint) validateAndFormatRego(content, path string) string {
	// 1. Optionally format
	if p.Format {
		formatted := p.applyOPAFmt(content, path)
		content = formatted
	}

	// 2. Structural validation
	p.checkResultStructure(content, path, []string{"skipped", "violations", "skip_reason"})

	// 3. Run Regal linter
	p.runRegalLinter(path, content)

	return content
}

func (p *PolicyToLint) applyOPAFmt(content, file string) string {
	formatted, err := format.SourceWithOpts(file, []byte(content), format.Opts{})
	if err != nil {
		p.AddError(file, "Auto-formatting failed", 0)
		return content
	}
	return string(formatted)
}

func (p *PolicyToLint) checkResultStructure(content, path string, keys []string) {
	// Regex to capture result := { ... } including multiline
	re := regexp.MustCompile(`(?s)result\s*:=\s*\{(.+?)\}`)
	match := re.FindStringSubmatch(content)
	if match == nil {
		p.AddError(path, "no result literal found", 0)
		return
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
			p.AddError(path, fmt.Sprintf("missing %q key in result", want), 0)
		}
	}
}

// Runs the Regal linter on the given rego content and records any violations
func (p *PolicyToLint) runRegalLinter(filePath, content string) {
	inputModules, err := rules.InputFromText(filePath, content)
	if err != nil {
		p.AddError(filePath, fmt.Sprintf("failed to prepare for linting: %v", err), 0)
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
		p.AddError(filePath, fmt.Sprintf("linting failed: %v", err), 0)
		return
	}

	// Handle Regal violations by formatting
	for _, v := range report.Violations {
		p.processRegalViolation(fmt.Errorf("%s:%d: %s", filePath, v.Location.Row, v.Description), filePath)
	}
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

// Splits grouped errors into individual errors
func (p *PolicyToLint) processRegalViolation(rawErr error, path string) {
	if rawErr == nil {
		return
	}

	errorStr := rawErr.Error()
	// Regex matches file path, line number and error message like: /path/file:line: message
	errorRegex := regexp.MustCompile(`^` + regexp.QuoteMeta(path) + `:(\d+):\s*(.+)$`)

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
				p.AddError(path, matches[2], lineNum)
				continue
			}
		}

		// If we didn't match the standard format, preserve the original error
		p.AddError(path, line, 0)
	}
}
