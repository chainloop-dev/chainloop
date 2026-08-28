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

package schemavalidators_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
	"github.com/stretchr/testify/require"
)

func TestValidateCycloneDX1_5(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "invalid sbom format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing properties: 'bomFormat', 'specVersion'",
		},
		{
			name:     "invalid clycondx format",
			filePath: "./testdata/openvex_v0.2.0.json",
			wantErr:  "missing properties: 'bomFormat', 'specVersion'",
		},
		{
			name:     "1.4 version",
			filePath: "./testdata/sbom.cyclonedx.json",
		},
		{
			name:     "1.5 version",
			filePath: "./testdata/sbom.cyclonedx-1.5.json",
		},
		{
			name:     "1.6 version error when parsing 1.6 specific fields",
			filePath: "./testdata/sbom.cyclonedx-1.6.json",
			wantErr:  "value must be one of \"application\", \"framework\", \"library\",",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v interface{}
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateCycloneDX(v, "1.5")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateCycloneDX1_6(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "invalid sbom format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  " missing properties: 'bomFormat', 'specVersion'",
		},
		{
			name:     "invalid clycondx format",
			filePath: "./testdata/openvex_v0.2.0.json",
			wantErr:  "missing properties: 'bomFormat', 'specVersion'",
		},
		{
			name:     "1.4 version",
			filePath: "./testdata/sbom.cyclonedx.json",
		},
		{
			name:     "1.5 version",
			filePath: "./testdata/sbom.cyclonedx-1.5.json",
		},
		{
			name:     "1.6 version",
			filePath: "./testdata/sbom.cyclonedx-1.6.json",
		},
		{
			name:     "1.6 version with duplicated element",
			filePath: "./testdata/sbom.cyclonedx-duplicated.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v interface{}
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateCycloneDX(v, "1.6")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateCSAF_2_0(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "invalid sbom format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing properties: 'document'",
		},
		{
			name:     "invalid csaf format",
			filePath: "./testdata/openvex_v0.2.0.json",
			wantErr:  "missing properties: 'document'",
		},
		{
			name:     "2.0 vex",
			filePath: "./testdata/csaf_vex_v0.2.0.json",
		},
		{
			name:     "2.0 security advisory",
			filePath: "./testdata/csaf_security_advisory.json",
		},
		{
			name:     "2.0 informational advisory",
			filePath: "./testdata/csaf_informational_advisory.json",
		},
		{
			name:     "2.0 security incident response",
			filePath: "./testdata/csaf_security_incident_response.json",
		},
		{
			name:     "2.1 vex",
			filePath: "./testdata/csaf_vex_v0.2.1.json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v interface{}
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateCSAF(v)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateRunnerContext(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "0.1 GitHub branch protection",
			filePath: "./testdata/runner_context_branches-v0.1.json",
		},
		{
			name:     "0.1 GitHub rulesets",
			filePath: "./testdata/runner_context_rulesets-v0.1.json",
		},
		{
			name:     "0.1 GitLab no protection",
			filePath: "./testdata/runner_context_gitlab_no_protection-v0.1.json",
		},
		{
			name:     "invalid Chainloop runner context - missing meta",
			filePath: "./testdata/runner_context_missing-meta-v0.1.json",
			wantErr:  "missing properties: 'meta'",
		},
		{
			name:     "invalid Chainloop runner context - missing data",
			filePath: "./testdata/runner_context_missing-data-v0.1.json",
			wantErr:  "missing properties: 'data'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v interface{}
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateChainloopRunnerContext(v, "")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateAICodingSession(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "valid coding session",
			filePath: "./testdata/ai_coding_session_valid.json",
		},
		{
			name:     "missing required fields",
			filePath: "./testdata/ai_coding_session_missing_required.json",
			wantErr:  "missing properties",
		},
		{
			name:     "completely wrong format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing properties",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v any
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateAICodingSession(v, "")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateSecurityContext(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "valid security context",
			filePath: "./testdata/ai_security_context_valid.json",
		},
		{
			name:     "missing required fields",
			filePath: "./testdata/ai_security_context_missing_required.json",
			wantErr:  "missing properties",
		},
		{
			name:     "completely wrong format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing properties",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v any
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateSecurityContext(v, "")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateOpenAPI(t *testing.T) {
	testCases := []struct {
		name    string
		data    any
		wantErr string
	}{
		{
			name:    "invalid data type",
			data:    "not a map",
			wantErr: "expected object, but got string",
		},
		{
			name: "missing required fields",
			data: map[string]any{
				"openapi": "3.0.3",
			},
			wantErr: "missing properties",
		},
		{
			name: "valid Swagger 2.0 spec",
			data: loadJSONFile(t, "./testdata/swagger-2.0.json"),
		},
		{
			name: "valid OpenAPI 3.0 spec",
			data: loadJSONFile(t, "./testdata/openapi-3.0.json"),
		},
		{
			name: "valid OpenAPI 3.2 spec",
			data: loadJSONFile(t, "./testdata/openapi-3.2.json"),
		},
		{
			name: "missing required fields for Swagger 2.0",
			data: map[string]any{
				"swagger": "2.0",
			},
			wantErr: "missing properties",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := schemavalidators.ValidateOpenAPI(tc.data)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateAsyncAPI(t *testing.T) {
	testCases := []struct {
		name    string
		data    any
		wantErr string
	}{
		{
			name:    "invalid data type",
			data:    "not a map",
			wantErr: "expected object, but got string",
		},
		{
			name: "missing required fields",
			data: map[string]any{
				"asyncapi": "2.6.0",
			},
			wantErr: "missing properties",
		},
		{
			name: "valid AsyncAPI 2.6 spec",
			data: loadJSONFile(t, "./testdata/asyncapi-2.6.json"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := schemavalidators.ValidateAsyncAPI(tc.data)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func loadJSONFile(t *testing.T, path string) any {
	t.Helper()
	f, err := os.ReadFile(path)
	require.NoError(t, err)
	var v any
	require.NoError(t, json.Unmarshal(f, &v))
	return v
}

func TestValidateAIAgentConfig(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "valid full config",
			filePath: "./testdata/ai_agent_config_valid.json",
		},
		{
			name:     "valid minimal config",
			filePath: "./testdata/ai_agent_config_minimal.json",
		},
		{
			name:     "missing agent",
			filePath: "./testdata/ai_agent_config_missing_agent.json",
			wantErr:  "missing properties: 'agent'",
		},
		{
			name:     "empty config_files array",
			filePath: "./testdata/ai_agent_config_empty_config_files.json",
		},
		{
			name:     "config file missing required fields",
			filePath: "./testdata/ai_agent_config_missing_config_file_fields.json",
			wantErr:  "missing properties",
		},
		{
			name:     "additional properties not allowed",
			filePath: "./testdata/ai_agent_config_extra_fields.json",
			wantErr:  "additionalProperties 'unknown_field' not allowed",
		},
		{
			name:     "completely wrong format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing properties",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v any
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateAIAgentConfig(v, "")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidateOSSFScorecard(t *testing.T) {
	testCases := []struct {
		name     string
		filePath string
		wantErr  string
	}{
		{
			name:     "valid scorecard report",
			filePath: "./testdata/scorecard_valid.json",
		},
		{
			name:     "completely wrong format",
			filePath: "./testdata/sbom-spdx.json",
			wantErr:  "missing properties",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.ReadFile(tc.filePath)
			require.NoError(t, err)

			var v any
			require.NoError(t, json.Unmarshal(f, &v))

			err = schemavalidators.ValidateOSSFScorecard(v, "")
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

// aiSecurityContextSchemaPath is the on-disk copy of the schema embedded by the
// package. The taxonomy guard below reads it directly so it asserts against the
// exact document that ships.
const aiSecurityContextSchemaPath = "./internal_schemas/aisecuritycontext/ai-security-context-0.1.schema.json"

// l0TaxonomyDefinitions is the slice of the AI security context schema that
// carries the L0 taxonomy: the enum that defines it and the pattern derived from
// it. JSON Schema draft-07 cannot interpolate an enum into a pattern, so the
// pattern is generated rather than referenced, and the tests below are what stop
// the two from drifting.
type l0TaxonomyDefinitions struct {
	Definitions struct {
		L0Class struct {
			Enum []string `json:"enum"`
		} `json:"l0_class"`
		L0ClassPath struct {
			Pattern string `json:"pattern"`
		} `json:"l0_class_path"`
	} `json:"definitions"`
}

// l0ClassPathPattern derives the pattern for a slash-joined, sorted path of
// taxonomy members from the taxonomy itself. Whoever edits the enum regenerates
// the pattern with this, rather than hand-editing a second copy of the list.
func l0ClassPathPattern(taxonomy []string) string {
	alternation := strings.Join(taxonomy, "|")
	return fmt.Sprintf("^$|^(%s)(/(%s))*$", alternation, alternation)
}

func readL0Taxonomy(t *testing.T) (l0TaxonomyDefinitions, []byte) {
	t.Helper()

	raw, err := os.ReadFile(aiSecurityContextSchemaPath)
	require.NoError(t, err)

	var defs l0TaxonomyDefinitions
	require.NoError(t, json.Unmarshal(raw, &defs))
	require.NotEmpty(t, defs.Definitions.L0Class.Enum, "l0_class enum must define the taxonomy")

	return defs, raw
}

// TestAISecurityContextL0TaxonomyHasSingleSource pins the taxonomy to one place.
// A second hand-maintained copy of the class list lets the two diverge silently:
// a shared_surface carrying a newly added class would be rejected while a
// fingerprint carrying the same class passes.
func TestAISecurityContextL0TaxonomyHasSingleSource(t *testing.T) {
	defs, raw := readL0Taxonomy(t)
	taxonomy := defs.Definitions.L0Class.Enum

	want := l0ClassPathPattern(taxonomy)
	require.Equal(t, want, defs.Definitions.L0ClassPath.Pattern,
		"l0_class_path.pattern is derived from the l0_class enum; regenerate it after editing the taxonomy")

	// The derived pattern repeats the alternation twice (first segment, then the
	// slash-joined tail) and nothing else in the document may spell out the
	// taxonomy. Any other occurrence is a copy that can drift.
	alternation := strings.Join(taxonomy, "|")
	require.Equal(t, 2, strings.Count(string(raw), alternation),
		"the taxonomy must only be spelled out in l0_class_path.pattern; reference #/definitions/l0_class or #/definitions/l0_class_path instead")
}

// TestValidateSecurityContextL0Class checks that the enum and the derived pattern
// accept the same taxonomy, in both the single-token position (fingerprints) and
// the slash-joined position (shared surfaces).
func TestValidateSecurityContextL0Class(t *testing.T) {
	defs, _ := readL0Taxonomy(t)
	taxonomy := defs.Definitions.L0Class.Enum

	// loadPayload returns a fresh decode per case so mutations do not leak. The
	// validator consumes generically decoded JSON, so a map is the input form here.
	loadPayload := func(t *testing.T) map[string]any {
		t.Helper()
		f, err := os.ReadFile("./testdata/ai_security_context_valid.json")
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(f, &payload))
		return payload
	}

	setClass := func(t *testing.T, payload map[string]any, section, class string) {
		t.Helper()
		entries, ok := payload[section].([]any)
		require.True(t, ok, "%s must be a populated array in the fixture", section)
		require.NotEmpty(t, entries)

		entry, ok := entries[0].(map[string]any)
		require.True(t, ok)
		entry["class"] = class
	}

	for _, class := range taxonomy {
		t.Run("fingerprint accepts "+class, func(t *testing.T) {
			payload := loadPayload(t)
			setClass(t, payload, "fingerprints", class)
			require.NoError(t, schemavalidators.ValidateSecurityContext(payload, ""))
		})

		t.Run("shared surface accepts "+class, func(t *testing.T) {
			payload := loadPayload(t)
			setClass(t, payload, "shared_surfaces", class)
			require.NoError(t, schemavalidators.ValidateSecurityContext(payload, ""))
		})
	}

	t.Run("shared surface accepts a slash-joined cluster", func(t *testing.T) {
		require.GreaterOrEqual(t, len(taxonomy), 2)

		payload := loadPayload(t)
		setClass(t, payload, "shared_surfaces", strings.Join(taxonomy, "/"))
		require.NoError(t, schemavalidators.ValidateSecurityContext(payload, ""))
	})

	t.Run("shared surface accepts an empty class", func(t *testing.T) {
		payload := loadPayload(t)
		setClass(t, payload, "shared_surfaces", "")
		require.NoError(t, schemavalidators.ValidateSecurityContext(payload, ""))
	})

	rejected := []struct {
		name    string
		section string
		class   string
	}{
		{name: "fingerprint rejects a class outside the taxonomy", section: "fingerprints", class: "race_condition"},
		{name: "shared surface rejects a class outside the taxonomy", section: "shared_surfaces", class: "race_condition"},
		{name: "shared surface rejects an unknown segment in a cluster", section: "shared_surfaces", class: taxonomy[0] + "/race_condition"},
		{name: "shared surface rejects a trailing separator", section: "shared_surfaces", class: taxonomy[0] + "/"},
	}

	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			payload := loadPayload(t)
			setClass(t, payload, tc.section, tc.class)
			require.Error(t, schemavalidators.ValidateSecurityContext(payload, ""))
		})
	}
}
