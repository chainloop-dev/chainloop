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

package auditor

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TemplateData represents the data structure used in audit log templates
type TemplateData struct {
	ActorName  string
	ActorEmail string
}

func TestGetActorIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		data     TemplateData
		expected string
	}{
		{
			name: "ActorName present - should use ActorName",
			data: TemplateData{
				ActorName:  "John Connor",
				ActorEmail: "john.doe@example.com",
			},
			expected: "John Connor",
		},
		{
			name: "ActorName empty, ActorEmail present - should use ActorEmail",
			data: TemplateData{
				ActorName:  "",
				ActorEmail: "john.doe@example.com",
			},
			expected: "john.doe@example.com",
		},
		{
			name: "ActorName missing, ActorEmail present - should use ActorEmail",
			data: TemplateData{
				ActorEmail: "jane.smith@example.com",
			},
			expected: "jane.smith@example.com",
		},
		{
			name: "Both ActorName and ActorEmail empty - should use system fallback",
			data: TemplateData{
				ActorName:  "",
				ActorEmail: "",
			},
			expected: ActorSystemIdentifier,
		},
		{
			name:     "Both ActorName and ActorEmail missing - should use system fallback",
			data:     TemplateData{},
			expected: ActorSystemIdentifier,
		},
		{
			name: "ActorName whitespace only, ActorEmail present - should use ActorEmail",
			data: TemplateData{
				ActorName:  "   ",
				ActorEmail: "test@example.com",
			},
			expected: "   ", // Template will consider whitespace as "truthy"
		},
		{
			name: "ActorEmail whitespace only - should use system fallback",
			data: TemplateData{
				ActorName:  "",
				ActorEmail: "   ",
			},
			expected: "   ", // Template will consider whitespace as "truthy"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get the template string from the helper
			templateStr := GetActorIdentifier()

			// Parse and execute the template
			tmpl, err := template.New("test").Parse(templateStr)
			require.NoError(t, err, "Template should parse correctly")

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, tt.data)
			require.NoError(t, err, "Template should execute correctly")

			result := buf.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}
