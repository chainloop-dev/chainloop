//
// Copyright 2026 The Chainloop Authors.
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

package aiagentconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected []string
	}{
		{
			name:     "no config files",
			files:    []string{"main.go", "README.md"},
			expected: nil,
		},
		{
			name:     "top-level CLAUDE.md only",
			files:    []string{"CLAUDE.md"},
			expected: []string{"CLAUDE.md"},
		},
		{
			name: "all claude patterns",
			files: []string{
				"CLAUDE.md",
				".claude/CLAUDE.md",
				".claude/settings.json",
				".mcp.json",
				".claude/rules/coding.md",
				".claude/rules/testing.md",
				".claude/agents/reviewer.md",
				".claude/commands/deploy.md",
				".claude/skills/search/SKILL.md",
			},
			expected: []string{
				".claude/CLAUDE.md",
				".claude/agents/reviewer.md",
				".claude/commands/deploy.md",
				".claude/rules/coding.md",
				".claude/rules/testing.md",
				".claude/settings.json",
				".claude/skills/search/SKILL.md",
				".mcp.json",
				"CLAUDE.md",
			},
		},
		{
			name: "non-matching files are ignored",
			files: []string{
				"CLAUDE.md",
				".claude/rules/coding.md",
				".claude/rules/coding.txt",   // wrong extension for rules pattern
				".claude/other/something.md", // not in a known pattern
				"some/nested/CLAUDE.md",      // nested too deep
			},
			expected: []string{
				".claude/rules/coding.md",
				"CLAUDE.md",
			},
		},
		{
			name: "results are sorted and deduplicated",
			files: []string{
				".mcp.json",
				"CLAUDE.md",
				".claude/settings.json",
			},
			expected: []string{
				".claude/settings.json",
				".mcp.json",
				"CLAUDE.md",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootDir := t.TempDir()

			for _, f := range tt.files {
				absPath := filepath.Join(rootDir, f)
				require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o755))
				require.NoError(t, os.WriteFile(absPath, []byte("test content"), 0o600))
			}

			results, err := Discover(rootDir)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, results)
		})
	}
}

func TestDiscoverNeverTraversesParent(t *testing.T) {
	parentDir := t.TempDir()

	// Create a CLAUDE.md in the parent
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "CLAUDE.md"), []byte("parent"), 0o600))

	// Create a subdirectory to search from
	childDir := filepath.Join(parentDir, "subproject")
	require.NoError(t, os.MkdirAll(childDir, 0o755))

	results, err := Discover(childDir)
	require.NoError(t, err)
	assert.Empty(t, results, "should not find files in parent directory")
}
