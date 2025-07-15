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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitialize(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("embedded rego", func(t *testing.T) {
		opts := &InitOptions{
			Directory:   tempDir,
			Embedded:    true,
			Name:        "test-policy",
			Description: "test description",
		}

		err := Initialize(opts)
		require.NoError(t, err)

		policyPath := filepath.Join(tempDir, "test-policy.yaml")
		assert.FileExists(t, policyPath)
	})

	t.Run("standalone rego file", func(t *testing.T) {
		opts := &InitOptions{
			Directory: tempDir,
			Embedded:  false,
			Name:      "standalone-rego",
		}

		err := Initialize(opts)
		require.NoError(t, err)

		assert.FileExists(t, filepath.Join(tempDir, "standalone-rego.yaml"))
		assert.FileExists(t, filepath.Join(tempDir, "standalone-rego.rego"))
	})

	t.Run("file exists and no force", func(t *testing.T) {
		opts := &InitOptions{
			Directory: tempDir,
			Name:      "duplicate",
		}

		// First time should succeed
		err := Initialize(opts)
		require.NoError(t, err)

		// Second time should fail
		err = Initialize(opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")

		// With force it should succeed
		opts.Force = true
		err = Initialize(opts)
		require.NoError(t, err)
	})

	t.Run("name and description are properly set", func(t *testing.T) {
		customName := "custom-policy-name"
		customDesc := "This is a custom policy description"

		opts := &InitOptions{
			Directory:   tempDir,
			Name:        customName,
			Description: customDesc,
			Embedded:    true,
		}

		err := Initialize(opts)
		require.NoError(t, err)

		policyPath := filepath.Join(tempDir, customName+".yaml")
		assert.FileExists(t, policyPath)

		content, err := os.ReadFile(policyPath)
		require.NoError(t, err)

		policyContent := string(content)

		assert.Contains(t, policyContent, "name: "+customName)

		assert.Contains(t, policyContent, "description: "+customDesc)

		assert.FileExists(t, filepath.Join(tempDir, customName+".yaml"))
	})
}

func TestLoadAndProcessTemplates(t *testing.T) {
	t.Run("embedded rego", func(t *testing.T) {
		opts := &InitOptions{
			Embedded: true,
			Name:     "embedded-test",
		}

		content, err := loadAndProcessTemplates(opts)
		require.NoError(t, err)
		assert.NotEmpty(t, content.YAML)
		assert.Empty(t, content.Rego) // Rego file should be empty for embedded
	})

	t.Run("separate rego file", func(t *testing.T) {
		opts := &InitOptions{
			Embedded: false,
			Name:     "separate-rego-test",
		}

		content, err := loadAndProcessTemplates(opts)
		require.NoError(t, err)
		assert.NotEmpty(t, content.YAML)
		assert.NotEmpty(t, content.Rego)
	})
}

func TestExecuteTemplate(t *testing.T) {
	testCases := []struct {
		name     string
		template string
		data     *TemplateData
		expected string
	}{
		{
			name:     "basic interpolation",
			template: "Hello {{.Name}}!",
			data:     &TemplateData{Name: "world"},
			expected: "Hello world!",
		},
		{
			name:     "sanitize function",
			template: "{{.Name | sanitize}}",
			data:     &TemplateData{Name: "My Policy"},
			expected: "my-policy",
		},
		{
			name:     "indent function",
			template: "{{indent 2 \"hello\"}}",
			expected: "  hello",
		},
		{
			name:     "multiple fields interpolation",
			template: "Name: {{.Name}}, Desc: {{.Description}}",
			data:     &TemplateData{Name: "test", Description: "description"},
			expected: "Name: test, Desc: description",
		},
		{
			name:     "trimSpace function",
			template: "{{.Name | trimSpace}}",
			data:     &TemplateData{Name: "  spaced  "},
			expected: "spaced",
		},
		{
			name:     "combined functions",
			template: "{{.Name | trimSpace | sanitize}}",
			data:     &TemplateData{Name: "  My Policy 123  "},
			expected: "my-policy-123",
		},
		{
			name:     "empty template",
			template: "",
			data:     &TemplateData{Name: "test"},
			expected: "",
		},
		{
			name:     "embedded rego flag",
			template: "Embedded: {{.Embedded}}",
			data:     &TemplateData{Embedded: true},
			expected: "Embedded: true",
		},
		{
			name:     "material kind",
			template: "Material: {{.MaterialKind}}",
			data:     &TemplateData{MaterialKind: "SBOM_CYCLONEDX_JSON"},
			expected: "Material: SBOM_CYCLONEDX_JSON",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := executeTemplate(tc.template, tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}

	errorCases := []struct {
		name     string
		template string
		data     *TemplateData
		errMsg   string
	}{
		{
			name:     "invalid template syntax",
			template: "{{.Name",
			data:     &TemplateData{Name: "test"},
			errMsg:   "template parsing error",
		},
		{
			name:     "missing field",
			template: "{{.MissingField}}",
			data:     &TemplateData{Name: "test"},
			errMsg:   "template execution error",
		},
		{
			name:     "invalid function",
			template: "{{.Name | invalidFunc}}",
			data:     &TemplateData{Name: "test"},
			errMsg:   "template parsing error",
		},
	}

	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := executeTemplate(tc.template, tc.data)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.errMsg)
		})
	}
}

func TestSanitizeName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"My Policy", "my-policy"},
		{"  Trim Spaces  ", "trim-spaces"},
		{"UPPER CASE", "upper-case"},
		{"Special!@#Chars", "special!@#chars"},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, sanitizeName(tc.input))
		})
	}
}
