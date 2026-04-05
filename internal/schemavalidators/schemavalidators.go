//
// Copyright 2024-2026 The Chainloop Authors.
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

package schemavalidators

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

// ErrInvalidJSONPayload represents an error for invalid JSON payload.
var ErrInvalidJSONPayload = errors.New("invalid JSON payload")

// CycloneDXVersion represents the version of CycloneDX schema.
type CycloneDXVersion string

// CSAFVersion represents the version of CSAF schema.
type CSAFVersion string

// RunnerContextVersion represents the version of Runner Context schema.
type RunnerContextVersion string

// PRInfoVersion represents the version of PR/MR Info schema.
type PRInfoVersion string

// AIAgentConfigVersion represents the version of AI Agent Config schema.
type AIAgentConfigVersion string

// AICodingSessionVersion represents the version of AI Coding Session schema.
type AICodingSessionVersion string

const (
	// RunnerContextVersion0_1 represents Runner Context version 0.1 schema.
	RunnerContextVersion0_1 RunnerContextVersion = "0.1"
	// PRInfoVersion1_0 represents PR/MR Info version 1.0 schema.
	PRInfoVersion1_0 PRInfoVersion = "1.0"
	// PRInfoVersion1_1 represents PR/MR Info version 1.1 schema (adds reviewers).
	PRInfoVersion1_1 PRInfoVersion = "1.1"
	// PRInfoVersion1_2 represents PR/MR Info version 1.2 schema (adds requested and review_status to reviewers).
	PRInfoVersion1_2 PRInfoVersion = "1.2"
	// PRInfoVersion1_3 represents PR/MR Info version 1.3 schema (author as object with type).
	PRInfoVersion1_3 PRInfoVersion = "1.3"
	// CycloneDXVersion1_5 represents CycloneDX version 1.5 schema.
	CycloneDXVersion1_5 CycloneDXVersion = "1.5"
	// CycloneDXVersion1_6 represents CycloneDX version 1.6 schema.
	CycloneDXVersion1_6 CycloneDXVersion = "1.6"
	// CSAFVersion2_0 represents CSAF version 2.0 schema.
	CSAFVersion2_0 CSAFVersion = "2.0"
	// CSAFVersion2_1 represents CSAF version 2.0 schema.
	CSAFVersion2_1 CSAFVersion = "2.1"
	// AIAgentConfigVersion0_1 represents AI Agent Config version 0.1 schema.
	AIAgentConfigVersion0_1 AIAgentConfigVersion = "0.1"
	// AICodingSessionVersion0_1 represents AI Coding Session version 0.1 schema.
	AICodingSessionVersion0_1 AICodingSessionVersion = "0.1"
)

var (
	// CycloneDX schemas
	//go:embed external_schemas/cyclonedx/jsf-0.82.schema.json
	jsfSpecVersion0_82 string
	//go:embed external_schemas/cyclonedx/spdx.schema.json
	spdxSpec string
	//go:embed external_schemas/cyclonedx/bom-1.5.schema.json
	bomSpecVersion1_5 string
	//go:embed external_schemas/cyclonedx/bom-1.6.schema.json
	bomSpecVersion1_6 string

	// CSAF schemas
	//go:embed external_schemas/csaf/csaf-2.0.schema.json
	casfSpecVersion2_0 string
	//go:embed external_schemas/csaf/csaf-2.1.schema.json
	casfSpecVersion2_1 string
	//go:embed external_schemas/csaf/cvss-v2.0.json
	cvssSpecVersion2_0 string
	//go:embed external_schemas/csaf/cvss-v3.0.json
	cvssSpecVersion3_0 string
	//go:embed external_schemas/csaf/cvss-v3.1.json
	cvssSpecVersion3_1 string
	//go:embed external_schemas/csaf/cvss-v4.0.json
	cvssSpecVersion4_0 string

	// Runner Context schemas
	//go:embed internal_schemas/runnercontext/runner-context-response-0.1.schema.json
	runnerContextSpecVersion0_1 string

	// PR/MR Info schemas
	//go:embed internal_schemas/prinfo/pr-info-1.0.schema.json
	prInfoSpecVersion1_0 string
	//go:embed internal_schemas/prinfo/pr-info-1.1.schema.json
	prInfoSpecVersion1_1 string
	//go:embed internal_schemas/prinfo/pr-info-1.2.schema.json
	prInfoSpecVersion1_2 string
	//go:embed internal_schemas/prinfo/pr-info-1.3.schema.json
	prInfoSpecVersion1_3 string

	// AI Agent Config schemas
	//go:embed internal_schemas/aiagentconfig/ai-agent-config-0.1.schema.json
	aiAgentConfigSpecVersion0_1 string

	// AI Coding Session schemas
	//go:embed internal_schemas/aicodingsession/ai-coding-session-0.1.schema.json
	aiCodingSessionSpecVersion0_1 string
)

var (
	compiledCycloneDxSchemas       map[CycloneDXVersion]*jsonschema.Schema
	cycloneDxOnce                  sync.Once
	compiledCSAFSchemas            map[CSAFVersion]*jsonschema.Schema
	csafOnce                       sync.Once
	compiledRunnerContextSchemas   map[RunnerContextVersion]*jsonschema.Schema
	runnerContextOnce              sync.Once
	compiledPRInfoSchemas          map[PRInfoVersion]*jsonschema.Schema
	prInfoOnce                     sync.Once
	compiledAIAgentConfigSchemas   map[AIAgentConfigVersion]*jsonschema.Schema
	aiAgentConfigOnce              sync.Once
	compiledAICodingSessionSchemas map[AICodingSessionVersion]*jsonschema.Schema
	aiCodingSessionOnce            sync.Once
)

func initCycloneDxSchemas() {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("http://cyclonedx.org/schema/jsf-0.82.schema.json", strings.NewReader(jsfSpecVersion0_82)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "http://cyclonedx.org/schema/jsf-0.82.schema.json", err))
	}
	if err := compiler.AddResource("http://cyclonedx.org/schema/spdx.schema.json", strings.NewReader(spdxSpec)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "http://cyclonedx.org/schema/spdx.schema.json", err))
	}
	if err := compiler.AddResource("http://cyclonedx.org/schema/bom-1.5.schema.json", strings.NewReader(bomSpecVersion1_5)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "http://cyclonedx.org/schema/bom-1.5.schema.json", err))
	}
	if err := compiler.AddResource("http://cyclonedx.org/schema/bom-1.6.schema.json", strings.NewReader(bomSpecVersion1_6)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "http://cyclonedx.org/schema/bom-1.6.schema.json", err))
	}

	// MustCompile panics if the embedded schema is malformed. This is a build-time
	// invariant: the schemas are embedded at compile time and must always be valid.
	compiledCycloneDxSchemas = map[CycloneDXVersion]*jsonschema.Schema{
		CycloneDXVersion1_5: compiler.MustCompile("http://cyclonedx.org/schema/bom-1.5.schema.json"),
		CycloneDXVersion1_6: compiler.MustCompile("http://cyclonedx.org/schema/bom-1.6.schema.json"),
	}
}

func initCSAFSchemas() {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://docs.oasis-open.org/csaf/csaf/v2.0/csaf_json_schema.json", strings.NewReader(casfSpecVersion2_0)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://docs.oasis-open.org/csaf/csaf/v2.0/csaf_json_schema.json", err))
	}
	if err := compiler.AddResource("https://docs.oasis-open.org/csaf/csaf/v2.1/csaf_json_schema.json", strings.NewReader(casfSpecVersion2_1)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://docs.oasis-open.org/csaf/csaf/v2.1/csaf_json_schema.json", err))
	}
	if err := compiler.AddResource("https://www.first.org/cvss/cvss-v2.0.json", strings.NewReader(cvssSpecVersion2_0)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://www.first.org/cvss/cvss-v2.0.json", err))
	}
	if err := compiler.AddResource("https://www.first.org/cvss/cvss-v3.0.json", strings.NewReader(cvssSpecVersion3_0)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://www.first.org/cvss/cvss-v3.0.json", err))
	}
	if err := compiler.AddResource("https://www.first.org/cvss/cvss-v3.1.json", strings.NewReader(cvssSpecVersion3_1)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://www.first.org/cvss/cvss-v3.1.json", err))
	}
	if err := compiler.AddResource("https://www.first.org/cvss/cvss-v4.0.json", strings.NewReader(cvssSpecVersion4_0)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://www.first.org/cvss/cvss-v4.0.json", err))
	}

	compiledCSAFSchemas = map[CSAFVersion]*jsonschema.Schema{
		CSAFVersion2_0: compiler.MustCompile("https://docs.oasis-open.org/csaf/csaf/v2.0/csaf_json_schema.json"),
		CSAFVersion2_1: compiler.MustCompile("https://docs.oasis-open.org/csaf/csaf/v2.1/csaf_json_schema.json"),
	}
}

func initRunnerContextSchemas() {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://chainloop.dev/schemas/runner-context-response-0.1.schema.json", strings.NewReader(runnerContextSpecVersion0_1)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://chainloop.dev/schemas/runner-context-response-0.1.schema.json", err))
	}

	compiledRunnerContextSchemas = map[RunnerContextVersion]*jsonschema.Schema{
		RunnerContextVersion0_1: compiler.MustCompile("https://chainloop.dev/schemas/runner-context-response-0.1.schema.json"),
	}
}

func initPRInfoSchemas() {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://schemas.chainloop.dev/prinfo/1.0/pr-info.schema.json", strings.NewReader(prInfoSpecVersion1_0)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://schemas.chainloop.dev/prinfo/1.0/pr-info.schema.json", err))
	}
	if err := compiler.AddResource("https://schemas.chainloop.dev/prinfo/1.1/pr-info.schema.json", strings.NewReader(prInfoSpecVersion1_1)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://schemas.chainloop.dev/prinfo/1.1/pr-info.schema.json", err))
	}
	if err := compiler.AddResource("https://schemas.chainloop.dev/prinfo/1.2/pr-info.schema.json", strings.NewReader(prInfoSpecVersion1_2)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://schemas.chainloop.dev/prinfo/1.2/pr-info.schema.json", err))
	}
	if err := compiler.AddResource("https://schemas.chainloop.dev/prinfo/1.3/pr-info.schema.json", strings.NewReader(prInfoSpecVersion1_3)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://schemas.chainloop.dev/prinfo/1.3/pr-info.schema.json", err))
	}

	compiledPRInfoSchemas = map[PRInfoVersion]*jsonschema.Schema{
		PRInfoVersion1_0: compiler.MustCompile("https://schemas.chainloop.dev/prinfo/1.0/pr-info.schema.json"),
		PRInfoVersion1_1: compiler.MustCompile("https://schemas.chainloop.dev/prinfo/1.1/pr-info.schema.json"),
		PRInfoVersion1_2: compiler.MustCompile("https://schemas.chainloop.dev/prinfo/1.2/pr-info.schema.json"),
		PRInfoVersion1_3: compiler.MustCompile("https://schemas.chainloop.dev/prinfo/1.3/pr-info.schema.json"),
	}
}

func initAIAgentConfigSchemas() {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://schemas.chainloop.dev/aiagentconfig/0.1/ai-agent-config.schema.json", strings.NewReader(aiAgentConfigSpecVersion0_1)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://schemas.chainloop.dev/aiagentconfig/0.1/ai-agent-config.schema.json", err))
	}

	compiledAIAgentConfigSchemas = map[AIAgentConfigVersion]*jsonschema.Schema{
		AIAgentConfigVersion0_1: compiler.MustCompile("https://schemas.chainloop.dev/aiagentconfig/0.1/ai-agent-config.schema.json"),
	}
}

func initAICodingSessionSchemas() {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("https://schemas.chainloop.dev/aicodingsession/0.1/ai-coding-session.schema.json", strings.NewReader(aiCodingSessionSpecVersion0_1)); err != nil {
		panic(fmt.Sprintf("schemavalidators: failed to add resource %s: %v", "https://schemas.chainloop.dev/aicodingsession/0.1/ai-coding-session.schema.json", err))
	}

	compiledAICodingSessionSchemas = map[AICodingSessionVersion]*jsonschema.Schema{
		AICodingSessionVersion0_1: compiler.MustCompile("https://schemas.chainloop.dev/aicodingsession/0.1/ai-coding-session.schema.json"),
	}
}

// ValidateCycloneDX validates the given object against the specified CycloneDX schema version.
func ValidateCycloneDX(data interface{}, version CycloneDXVersion) error {
	cycloneDxOnce.Do(initCycloneDxSchemas)

	if version == "" {
		version = CycloneDXVersion1_6
	}

	schema, ok := compiledCycloneDxSchemas[version]
	if !ok {
		return errors.New("invalid CycloneDX schema version")
	}

	if err := schema.Validate(data); err != nil {
		var invalidJSONTypeError jsonschema.InvalidJSONTypeError
		if errors.As(err, &invalidJSONTypeError) {
			return ErrInvalidJSONPayload
		}
		var validationError *jsonschema.ValidationError
		if errors.As(err, &validationError) {
			if slices.ContainsFunc(validationError.Causes, func(cause *jsonschema.ValidationError) bool {
				// Jfrog Xray: Do not fail in case of duplicated components. Policies will take care of validation and deduplication
				if cause.KeywordLocation == "/properties/components/uniqueItems" {
					return true
				}
				// Some validation errors are found deeper in the tree
				return slices.ContainsFunc(cause.Causes, func(c1 *jsonschema.ValidationError) bool {
					// Some scanners like Jfrog Xray might report null `cwes` element ("cwes": null)
					// the validator would fail with "expected array, but got null"
					return c1.KeywordLocation == "/properties/vulnerabilities/items/$ref/properties/cwes/type"
				})
			}) {
				return nil
			}
		}
		return err
	}

	return nil
}

// ValidateCSAF validates the given object against a CSAF schema version.
// The schema version is determined by the "csaf_version" field in the object.
func ValidateCSAF(data interface{}) error {
	csafOnce.Do(initCSAFSchemas)

	var errs error
	err := compiledCSAFSchemas[CSAFVersion2_1].Validate(data)
	if err != nil {
		if err := errorIsJSONFormat(err); err != nil {
			return err
		}

		errs = multierror.Append(errs, err)
	} else {
		return nil
	}

	err = compiledCSAFSchemas[CSAFVersion2_0].Validate(data)
	if err != nil {
		if err := errorIsJSONFormat(err); err != nil {
			errs = multierror.Append(errs, err)
			return errs
		}
		return multierror.Append(errs, err)
	}

	return nil
}

// ValidateChainloopRunnerContext validates the runner context schema.
// The schema version is determined by the "id" field in the object.
func ValidateChainloopRunnerContext(data interface{}, version RunnerContextVersion) error {
	runnerContextOnce.Do(initRunnerContextSchemas)

	if version == "" {
		version = RunnerContextVersion0_1
	}

	schema, ok := compiledRunnerContextSchemas[version]
	if !ok {
		return errors.New("invalid runner context schema version")
	}

	if err := schema.Validate(data); err != nil {
		var invalidJSONTypeError jsonschema.InvalidJSONTypeError
		if errors.As(err, &invalidJSONTypeError) {
			return ErrInvalidJSONPayload
		}
		return err
	}

	return nil
}

// ValidatePRInfo validates the PR/MR info schema.
func ValidatePRInfo(data interface{}, version PRInfoVersion) error {
	prInfoOnce.Do(initPRInfoSchemas)

	if version == "" {
		version = PRInfoVersion1_3
	}

	schema, ok := compiledPRInfoSchemas[version]
	if !ok {
		return errors.New("invalid PR info schema version")
	}

	if err := schema.Validate(data); err != nil {
		var invalidJSONTypeError jsonschema.InvalidJSONTypeError
		if errors.As(err, &invalidJSONTypeError) {
			return ErrInvalidJSONPayload
		}
		return err
	}

	return nil
}

// ValidateAIAgentConfig validates the AI agent config schema.
func ValidateAIAgentConfig(data any, version AIAgentConfigVersion) error {
	aiAgentConfigOnce.Do(initAIAgentConfigSchemas)

	if version == "" {
		version = AIAgentConfigVersion0_1
	}

	schema, ok := compiledAIAgentConfigSchemas[version]
	if !ok {
		return errors.New("invalid AI agent config schema version")
	}

	if err := schema.Validate(data); err != nil {
		var invalidJSONTypeError jsonschema.InvalidJSONTypeError
		if errors.As(err, &invalidJSONTypeError) {
			return ErrInvalidJSONPayload
		}
		return err
	}

	return nil
}

// ValidateAICodingSession validates the AI coding session schema.
func ValidateAICodingSession(data any, version AICodingSessionVersion) error {
	aiCodingSessionOnce.Do(initAICodingSessionSchemas)

	if version == "" {
		version = AICodingSessionVersion0_1
	}

	schema, ok := compiledAICodingSessionSchemas[version]
	if !ok {
		return errors.New("invalid AI coding session schema version")
	}

	if err := schema.Validate(data); err != nil {
		var invalidJSONTypeError jsonschema.InvalidJSONTypeError
		if errors.As(err, &invalidJSONTypeError) {
			return ErrInvalidJSONPayload
		}
		return err
	}

	return nil
}

// errorIsJSONFormat checks if the error is a JSON format error.
func errorIsJSONFormat(err error) error {
	var invalidJSONTypeError jsonschema.InvalidJSONTypeError
	if errors.As(err, &invalidJSONTypeError) {
		return ErrInvalidJSONPayload
	}
	return nil
}
