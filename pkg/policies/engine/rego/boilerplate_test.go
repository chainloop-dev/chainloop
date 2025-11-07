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
	"os"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectBoilerplate(t *testing.T) {
	testCases := []struct {
		name       string
		inputFile  string
		outputName string
	}{
		{
			name:       "simplified policy",
			inputFile:  "testdata/simplified-policy.rego",
			outputName: "simplified-policy-output.rego",
		},
		{
			name:       "full boilerplate exists",
			inputFile:  "testdata/full-boilerplate.rego",
			outputName: "full-boilerplate-output.rego",
		},
		{
			name:       "user defined valid_input",
			inputFile:  "testdata/custom-valid-input.rego",
			outputName: "custom-valid-input-output.rego",
		},
		{
			name:       "partial boilerplate",
			inputFile:  "testdata/partial-boilerplate.rego",
			outputName: "partial-boilerplate-output.rego",
		},
		{
			name:       "preserve multiple imports",
			inputFile:  "testdata/multiple-imports.rego",
			outputName: "multiple-imports-output.rego",
		},
		{
			name:       "with comments",
			inputFile:  "testdata/with-comments.rego",
			outputName: "with-comments-output.rego",
		},
		{
			name:       "only package and import",
			inputFile:  "testdata/only-package-import.rego",
			outputName: "only-package-import-output.rego",
		},
		{
			name:       "real world source commit example",
			inputFile:  "testdata/source-commit-simplified.rego",
			outputName: "source-commit-simplified-output.rego",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input, err := os.ReadFile(tc.inputFile)
			require.NoError(t, err)

			result, err := InjectBoilerplate(input, "test-policy")
			require.NoError(t, err)

			matchesOutput(t, result, tc.outputName)
		})
	}
}

// matchesOutput compares result against expected output file
func matchesOutput(t *testing.T, result []byte, outputName string) {
	t.Helper()

	outputPath := filepath.Join("testdata", "output", outputName)

	expected, err := os.ReadFile(outputPath)
	require.NoError(t, err, "failed to read output file %s", outputPath)

	assert.Equal(t, string(expected), string(result), "output doesn't match expected file %s", outputPath)

	// Also verify it's valid Rego
	_, err = ast.ParseModule("test", string(result))
	require.NoError(t, err, "generated Rego should be valid")
}

func TestDetectExistingRules(t *testing.T) {
	policyBytes, err := os.ReadFile("testdata/detect-rules.rego")
	require.NoError(t, err)

	module, err := ast.ParseModule("test", string(policyBytes))
	require.NoError(t, err)

	existing := detectExistingRules(module)

	// Check rules exist
	assert.True(t, existing.hasRule["result"])
	assert.True(t, existing.hasRule["skipped"])
	assert.True(t, existing.hasRule["valid_input"])
	assert.True(t, existing.hasRule["violations"])
	assert.False(t, existing.hasRule["skip_reason"])
	assert.False(t, existing.hasRule["ignore"])

	// Check defaults for rules
	assert.False(t, existing.hasDefault["result"])
	assert.True(t, existing.hasDefault["skipped"])
	assert.True(t, existing.hasDefault["valid_input"])
	assert.False(t, existing.hasDefault["violations"])
	assert.False(t, existing.hasDefault["skip_reason"])
	assert.False(t, existing.hasDefault["ignore"])
}
