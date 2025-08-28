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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/styrainc/regal/pkg/report"
)

func TestLookup(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("non-existent file", func(t *testing.T) {
		policy, err := Lookup(filepath.Join(tempDir, "nonexistent.yaml"), "", false)
		assert.Error(t, err)
		assert.Nil(t, policy)
		assert.Contains(t, err.Error(), "unrecognized scheme")
	})

	t.Run("directory instead of file", func(t *testing.T) {
		policy, err := Lookup(tempDir, "", false)
		assert.Error(t, err)
		assert.Nil(t, policy)
		assert.Contains(t, err.Error(), "expected a file but got a directory")
	})

	t.Run("valid yaml file", func(t *testing.T) {
		policy, err := Lookup("testdata/embedded-policy.yaml", "", false)
		require.NoError(t, err)
		assert.NotNil(t, policy)
		assert.Contains(t, policy.Path, "testdata/embedded-policy.yaml")
		assert.Len(t, policy.YAMLFiles, 1)
		assert.Len(t, policy.RegoFiles, 0)
	})

	t.Run("valid rego file", func(t *testing.T) {
		policy, err := Lookup("testdata/valid.rego", "", false)
		require.NoError(t, err)
		assert.NotNil(t, policy)
		assert.Contains(t, policy.Path, "testdata/valid.rego")
		assert.Len(t, policy.YAMLFiles, 0)
		assert.Len(t, policy.RegoFiles, 1)
	})

	t.Run("unsupported file extension", func(t *testing.T) {
		txtFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(txtFile, []byte("some content"), 0600)
		require.NoError(t, err)

		policy, err := Lookup(txtFile, "", false)
		assert.Error(t, err)
		assert.Nil(t, policy)
		assert.Contains(t, err.Error(), "unsupported file extension .txt")
	})

	t.Run("yaml with referenced rego file", func(t *testing.T) {
		policy, err := Lookup("testdata/policy.yaml", "", false)
		require.NoError(t, err)
		assert.NotNil(t, policy)
		assert.Len(t, policy.YAMLFiles, 1)
		assert.Len(t, policy.RegoFiles, 1)
	})
}

func TestPolicyToLint_processFile(t *testing.T) {
	tempDir := t.TempDir()
	policy := &PolicyToLint{}

	t.Run("process yaml file", func(t *testing.T) {
		content := "test: yaml"
		yamlFile := filepath.Join(tempDir, "test.yaml")
		err := os.WriteFile(yamlFile, []byte(content), 0644)
		require.NoError(t, err)

		err = policy.processFile(yamlFile)
		require.NoError(t, err)
		assert.Len(t, policy.YAMLFiles, 1)
		assert.Equal(t, yamlFile, policy.YAMLFiles[0].Path)
		assert.Equal(t, []byte(content), policy.YAMLFiles[0].Content)
	})

	t.Run("process rego file", func(t *testing.T) {
		content := "package main"
		regoFile := filepath.Join(tempDir, "test.rego")
		err := os.WriteFile(regoFile, []byte(content), 0600)
		require.NoError(t, err)

		err = policy.processFile(regoFile)
		require.NoError(t, err)
		assert.Len(t, policy.RegoFiles, 1)
		assert.Equal(t, regoFile, policy.RegoFiles[0].Path)
		assert.Equal(t, []byte(content), policy.RegoFiles[0].Content)
	})

	t.Run("unsupported file extension", func(t *testing.T) {
		txtFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(txtFile, []byte("content"), 0600)
		require.NoError(t, err)

		err = policy.processFile(txtFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported file extension .txt")
	})
}

func TestPolicyToLint_checkResultStructure(t *testing.T) {
	t.Run("valid result structure", func(t *testing.T) {
		policy := &PolicyToLint{}
		content, err := os.ReadFile("testdata/valid.rego")
		require.NoError(t, err)
		policy.checkResultStructure(string(content), "test.rego", []string{"violations", "skip_reason", "skipped"})
		assert.False(t, policy.HasErrors())
	})

	t.Run("missing result literal", func(t *testing.T) {
		policy := &PolicyToLint{}
		content := `package main

output := {
    "violations": []
}`
		policy.checkResultStructure(content, "test.rego", []string{"violations"})
		assert.True(t, policy.HasErrors())
		assert.Contains(t, policy.Errors[0].Message, "no result literal found")
	})

	t.Run("missing required keys", func(t *testing.T) {
		policy := &PolicyToLint{}
		content, err := os.ReadFile("testdata/missing-keys.rego")
		require.NoError(t, err)
		policy.checkResultStructure(string(content), "test.rego", []string{"violations", "skip_reason", "skipped"})
		assert.True(t, policy.HasErrors())
		assert.Len(t, policy.Errors, 2)
		assert.Contains(t, policy.Errors[0].Message, `missing "skip_reason" key`)
		assert.Contains(t, policy.Errors[1].Message, `missing "skipped" key`)
	})
}

func TestPolicyToLint_formatViolationError(t *testing.T) {
	policy := &PolicyToLint{}

	testCases := []struct {
		name         string
		violation    report.Violation
		regoRuleMap  map[int]string
		expectedText string
	}{
		{
			name: "violation with rule name",
			violation: report.Violation{
				Description: "Max rule length exceeded",
				Location: report.Location{
					Row: 5,
				},
				RelatedResources: []report.RelatedResource{
					{Reference: "https://docs.styra.com/regal/rules/style/rule-length"},
				},
			},
			regoRuleMap:  map[int]string{5: "my_rule"},
			expectedText: "[my_rule]: Max rule length exceeded - https://docs.styra.com/regal/rules/style/rule-length",
		},
		{
			name: "violation without rule name",
			violation: report.Violation{
				Description: "General error",
				Location: report.Location{
					Row: 10,
				},
				RelatedResources: []report.RelatedResource{
					{Reference: "https://example.com"},
				},
			},
			regoRuleMap:  map[int]string{},
			expectedText: ": General error - https://example.com",
		},
		{
			name: "violation with multiple resources",
			violation: report.Violation{
				Description: "Multiple issues found",
				Location: report.Location{
					Row: 3,
				},
				RelatedResources: []report.RelatedResource{
					{Reference: "https://link1.com"},
					{Reference: "https://link2.com"},
				},
			},
			regoRuleMap:  map[int]string{3: "test_rule"},
			expectedText: "[test_rule]: Multiple issues found - https://link1.com, https://link2.com",
		},
		{
			name: "violation with opa fmt reference",
			violation: report.Violation{
				Description: "Use `opa fmt` to format",
				Location: report.Location{
					Row: 1,
				},
				RelatedResources: []report.RelatedResource{
					{Reference: "https://example.com"},
				},
			},
			regoRuleMap:  map[int]string{1: "format_rule"},
			expectedText: "[format_rule]: Use `--format` to format - https://example.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := policy.formatViolationError(tc.violation, tc.regoRuleMap)
			assert.Equal(t, tc.expectedText, result)
		})
	}
}

func TestPolicyToLint_buildRegoRuleMap(t *testing.T) {
	policy := &PolicyToLint{}

	testCases := []struct {
		name     string
		regoFile string
		expected map[int]string
	}{
		{
			name:     "single rule",
			regoFile: "testdata/valid.rego",
			expected: map[int]string{3: "result"},
		},
		{
			name:     "multiple rules",
			regoFile: "testdata/multiple-rules.rego",
			expected: map[int]string{
				3: "allow",
				5: "deny",
				7: "result",
			},
		},
		{
			name:     "empty rego",
			regoFile: "",
			expected: map[int]string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var content string
			if tc.regoFile != "" {
				contentBytes, err := os.ReadFile(tc.regoFile)
				require.NoError(t, err)
				content = string(contentBytes)
			} else {
				content = "invalid rego syntax {"
			}
			result := policy.buildRegoRuleMap(content)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPolicyToLint_applyOPAFmt(t *testing.T) {
	t.Run("format valid rego", func(t *testing.T) {
		policy := &PolicyToLint{}
		content, err := os.ReadFile("testdata/unformatted.rego")
		require.NoError(t, err)

		result := policy.applyOPAFmt(string(content), "test.rego")
		assert.Contains(t, result, "result := {")
		assert.False(t, policy.HasErrors())
	})

	t.Run("format invalid rego", func(t *testing.T) {
		policy := &PolicyToLint{}
		content := `invalid rego {`
		result := policy.applyOPAFmt(content, "test.rego")
		assert.Equal(t, content, result)
		assert.True(t, policy.HasErrors())
		assert.Contains(t, policy.Errors[0].Message, "auto-formatting failed")
	})
}

func TestPolicyToLint_Validate(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("validate rego files", func(t *testing.T) {
		content, err := os.ReadFile("testdata/valid.rego")
		require.NoError(t, err)

		regoFile := filepath.Join(tempDir, "test.rego")
		err = os.WriteFile(regoFile, content, 0644)
		require.NoError(t, err)

		policy := &PolicyToLint{
			RegoFiles: []*File{
				{
					Path:    regoFile,
					Content: content,
				},
			},
		}

		policy.Validate()
		assert.False(t, policy.HasErrors())
	})

	t.Run("validate and format rego files", func(t *testing.T) {
		content, err := os.ReadFile("testdata/unformatted.rego")
		require.NoError(t, err)

		regoFile := filepath.Join(tempDir, "format_test.rego")
		err = os.WriteFile(regoFile, content, 0644)
		require.NoError(t, err)

		policy := &PolicyToLint{
			Format: true,
			RegoFiles: []*File{
				{
					Path:    regoFile,
					Content: content,
				},
			},
		}

		policy.Validate()

		formatted, err := os.ReadFile(regoFile)
		require.NoError(t, err)
		formattedStr := string(formatted)

		expected, err := os.ReadFile("testdata/valid.rego")
		require.NoError(t, err)
		expectedStr := string(expected)

		assert.Equal(t, expectedStr, formattedStr)
	})
}
