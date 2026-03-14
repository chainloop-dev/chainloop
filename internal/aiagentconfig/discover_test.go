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

// createFiles is a test helper that creates files relative to rootDir.
func createFiles(t *testing.T, rootDir string, files []string) {
	t.Helper()
	for _, f := range files {
		absPath := filepath.Join(rootDir, f)
		require.NoError(t, os.MkdirAll(filepath.Dir(absPath), 0o755))
		require.NoError(t, os.WriteFile(absPath, []byte("test content"), 0o600))
	}
}

func TestDiscoverAll(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected map[string][]string
	}{
		{
			name:     "no config files",
			files:    []string{"main.go", "README.md"},
			expected: map[string][]string{},
		},
		{
			name:  "claude only",
			files: []string{"CLAUDE.md", ".claude/settings.json"},
			expected: map[string][]string{
				"claude": {".claude/settings.json", "CLAUDE.md"},
			},
		},
		{
			name:  "cursor only",
			files: []string{".cursor/rules/coding.md", ".cursor/agents/test.md"},
			expected: map[string][]string{
				"cursor": {".cursor/agents/test.md", ".cursor/rules/coding.md"},
			},
		},
		{
			name: "cursor mdc extension",
			files: []string{
				".cursor/rules/react.mdc",
				".cursor/rules/api.md",
			},
			expected: map[string][]string{
				"cursor": {".cursor/rules/api.md", ".cursor/rules/react.mdc"},
			},
		},
		{
			name: "cursor nested rules",
			files: []string{
				".cursor/rules/frontend/components.md",
				".cursor/rules/backend/api.mdc",
			},
			expected: map[string][]string{
				"cursor": {".cursor/rules/backend/api.mdc", ".cursor/rules/frontend/components.md"},
			},
		},
		{
			name:  "cursor skills",
			files: []string{".cursor/skills/search/SKILL.md"},
			expected: map[string][]string{
				"cursor": {".cursor/skills/search/SKILL.md"},
			},
		},
		{
			name: "both agents - separate results",
			files: []string{
				"CLAUDE.md",
				".claude/settings.json",
				".cursor/rules/coding.md",
				".cursor/agents/reviewer.md",
			},
			expected: map[string][]string{
				"claude": {".claude/settings.json", "CLAUDE.md"},
				"cursor": {".cursor/agents/reviewer.md", ".cursor/rules/coding.md"},
			},
		},
		{
			name: "shared files included in each agent",
			files: []string{
				"CLAUDE.md",
				".cursor/rules/coding.md",
				".mcp.json",
				"AGENTS.md",
			},
			expected: map[string][]string{
				"claude": {".mcp.json", "AGENTS.md", "CLAUDE.md"},
				"cursor": {".cursor/rules/coding.md", ".mcp.json", "AGENTS.md"},
			},
		},
		{
			name:     "only shared files - no agents returned",
			files:    []string{".mcp.json", "AGENTS.md"},
			expected: map[string][]string{},
		},
		{
			name: "all claude patterns with shared",
			files: []string{
				"CLAUDE.md",
				".claude/CLAUDE.md",
				".claude/settings.json",
				".mcp.json",
				"AGENTS.md",
				".claude/rules/coding.md",
				".claude/rules/testing.md",
				".claude/agents/reviewer.md",
				".claude/commands/deploy.md",
				".claude/skills/search/SKILL.md",
			},
			expected: map[string][]string{
				"claude": {
					".claude/CLAUDE.md",
					".claude/agents/reviewer.md",
					".claude/commands/deploy.md",
					".claude/rules/coding.md",
					".claude/rules/testing.md",
					".claude/settings.json",
					".claude/skills/search/SKILL.md",
					".mcp.json",
					"AGENTS.md",
					"CLAUDE.md",
				},
			},
		},
		{
			name: "non-matching files are ignored",
			files: []string{
				"CLAUDE.md",
				".claude/rules/coding.txt",   // wrong extension
				".claude/other/something.md", // not a known pattern
				"some/nested/CLAUDE.md",      // nested too deep
				".cursor/other/random.md",    // not a known pattern
			},
			expected: map[string][]string{
				"claude": {"CLAUDE.md"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootDir := t.TempDir()
			createFiles(t, rootDir, tt.files)

			results, err := DiscoverAll(rootDir)
			require.NoError(t, err)

			if len(tt.expected) == 0 {
				assert.Empty(t, results)
			} else {
				assert.Equal(t, tt.expected, results)
			}
		})
	}
}

func TestDiscoverAllNeverTraversesParent(t *testing.T) {
	parentDir := t.TempDir()

	// Create a CLAUDE.md in the parent
	require.NoError(t, os.WriteFile(filepath.Join(parentDir, "CLAUDE.md"), []byte("parent"), 0o600))

	// Create a subdirectory to search from
	childDir := filepath.Join(parentDir, "subproject")
	require.NoError(t, os.MkdirAll(childDir, 0o755))

	results, err := DiscoverAll(childDir)
	require.NoError(t, err)
	assert.Empty(t, results, "should not find files in parent directory")
}
