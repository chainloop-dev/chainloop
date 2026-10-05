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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/repositoryconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveProjectToYML(t *testing.T) {
	t.Run("creates new file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, SaveProjectToYML(dir, "new-project"))

		got := repositoryconfig.LoadProjectFromYML(dir)
		assert.Equal(t, "new-project", got)
	})

	t.Run("preserves existing fields", func(t *testing.T) {
		dir := t.TempDir()
		existing := "projectName: old-project\nprojectVersion: v1.0.0\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte(existing), 0600))

		require.NoError(t, SaveProjectToYML(dir, "updated-project"))

		data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "projectName: updated-project")
		assert.Contains(t, string(data), "projectVersion: v1.0.0")
	})

	t.Run("updates existing projectName", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: old\n"), 0600))

		require.NoError(t, SaveProjectToYML(dir, "new"))
		assert.Equal(t, "new", repositoryconfig.LoadProjectFromYML(dir))
	})

	t.Run("respects existing .chainloop.yaml extension", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yaml"), []byte("projectName: yaml-ext\nprojectVersion: v2.0.0\n"), 0600))

		require.NoError(t, SaveProjectToYML(dir, "updated"))

		// Should have updated the .yaml file, not created a new .yml file
		_, err := os.Stat(filepath.Join(dir, ".chainloop.yml"))
		assert.True(t, os.IsNotExist(err), "should not create .yml when .yaml exists")

		data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "projectName: updated")
		assert.Contains(t, string(data), "projectVersion: v2.0.0")
	})

	t.Run("returns error on malformed yaml", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte(":\ninvalid: [yaml\n"), 0600))

		err := SaveProjectToYML(dir, "proj")
		assert.Error(t, err)
	})
}

// TestUpdateChainloopYMLPreservesFormatting pins that updating one field keeps
// the rest of the file as the user wrote it: comments, key order and
// indentation. .chainloop.yml is checked into the user's repository, so a
// rewrite that reformats it shows up as noise in their next diff. The fixture
// is deliberately in non-alphabetical order, so re-marshalling through a map
// cannot pass unnoticed, and the whole output is compared.
func TestUpdateChainloopYMLPreservesFormatting(t *testing.T) {
	// projectVersion, projectName, organization: reverse-alphabetical.
	const existing = `# This indicates the [current version]+next
projectVersion: v1.0.0+next
projectName: old-project
# organization and projectName are used for Chainloop Trace
# https://docs.chainloop.dev/reference/operator/trace
organization: chainloop
# Maps material names to a location on disk
scorecards:
  - name: sarif-results
    path: metadata/results.sarif
`

	testCases := []struct {
		name   string
		update func(dir string) error
		want   string
	}{
		{
			name:   "updating an existing field touches only that value",
			update: func(dir string) error { return SaveProjectToYML(dir, "new-project") },
			want: `# This indicates the [current version]+next
projectVersion: v1.0.0+next
projectName: new-project
# organization and projectName are used for Chainloop Trace
# https://docs.chainloop.dev/reference/operator/trace
organization: chainloop
# Maps material names to a location on disk
scorecards:
  - name: sarif-results
    path: metadata/results.sarif
`,
		},
		{
			name:   "a new field is appended",
			update: func(dir string) error { return SaveWorkflowToYML(dir, "my-workflow") },
			want: `# This indicates the [current version]+next
projectVersion: v1.0.0+next
projectName: old-project
# organization and projectName are used for Chainloop Trace
# https://docs.chainloop.dev/reference/operator/trace
organization: chainloop
# Maps material names to a location on disk
scorecards:
  - name: sarif-results
    path: metadata/results.sarif
workflowName: my-workflow
`,
		},
		{
			name:   "a new boolean field is appended",
			update: func(dir string) error { return SaveRequireTraceToYML(dir, true) },
			want: `# This indicates the [current version]+next
projectVersion: v1.0.0+next
projectName: old-project
# organization and projectName are used for Chainloop Trace
# https://docs.chainloop.dev/reference/operator/trace
organization: chainloop
# Maps material names to a location on disk
scorecards:
  - name: sarif-results
    path: metadata/results.sarif
requireTrace: true
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".chainloop.yml")
			require.NoError(t, os.WriteFile(path, []byte(existing), 0600))

			require.NoError(t, tc.update(dir))

			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(data))
		})
	}
}

// TestUpdateChainloopYMLKeepsIndentation pins that nested blocks keep the
// indentation width the file already uses, rather than being reindented to
// whatever this package prefers.
func TestUpdateChainloopYMLKeepsIndentation(t *testing.T) {
	testCases := []struct {
		name     string
		existing string
		want     string
	}{
		{
			name: "two spaces",
			existing: `projectName: p
scorecards:
  - name: sarif
    path: results.sarif
`,
			want: `projectName: p
scorecards:
  - name: sarif
    path: results.sarif
workflowName: w
`,
		},
		{
			name: "four spaces",
			existing: `projectName: p
scorecards:
    - name: sarif
      path: results.sarif
`,
			want: `projectName: p
scorecards:
    - name: sarif
      path: results.sarif
workflowName: w
`,
		},
		{
			name:     "nothing nested falls back to two",
			existing: "projectName: p\n",
			want:     "projectName: p\nworkflowName: w\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".chainloop.yml")
			require.NoError(t, os.WriteFile(path, []byte(tc.existing), 0600))

			require.NoError(t, SaveWorkflowToYML(dir, "w"))

			data, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(data))
		})
	}
}

func TestRequireTrace(t *testing.T) {
	t.Run("defaults to false when field missing", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\n"), 0600))
		assert.False(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("defaults to false when no config file", func(t *testing.T) {
		dir := t.TempDir()
		assert.False(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("returns false when explicitly set", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\nrequireTrace: false\n"), 0600))
		assert.False(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("reads from file without projectName", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("requireTrace: false\n"), 0600))
		assert.False(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("returns true when explicitly set", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\nrequireTrace: true\n"), 0600))
		assert.True(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("save and load round-trip", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\n"), 0600))

		require.NoError(t, SaveRequireTraceToYML(dir, false))
		assert.False(t, LoadRequireTraceFromYML(dir))

		require.NoError(t, SaveRequireTraceToYML(dir, true))
		assert.True(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("preserves other fields", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: my-proj\nprojectVersion: v1.0.0\n"), 0600))

		require.NoError(t, SaveRequireTraceToYML(dir, false))

		data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "projectName: my-proj")
		assert.Contains(t, string(data), "projectVersion: v1.0.0")
		assert.Contains(t, string(data), "requireTrace: false")
	})
}

func TestWorkflow(t *testing.T) {
	t.Run("defaults to empty when field missing", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\n"), 0600))
		assert.Empty(t, LoadWorkflowFromYML(dir))
	})

	t.Run("defaults to empty when no config file", func(t *testing.T) {
		dir := t.TempDir()
		assert.Empty(t, LoadWorkflowFromYML(dir))
	})

	t.Run("reads from file without projectName", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("workflowName: my-flow\n"), 0600))
		assert.Equal(t, "my-flow", LoadWorkflowFromYML(dir))
	})

	t.Run("save and load round-trip", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\n"), 0600))

		require.NoError(t, SaveWorkflowToYML(dir, "my-flow"))
		assert.Equal(t, "my-flow", LoadWorkflowFromYML(dir))

		require.NoError(t, SaveWorkflowToYML(dir, "another"))
		assert.Equal(t, "another", LoadWorkflowFromYML(dir))
	})

	t.Run("preserves other fields", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: my-proj\nprojectVersion: v1.0.0\nrequireTrace: true\n"), 0600))

		require.NoError(t, SaveWorkflowToYML(dir, "my-flow"))

		data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "projectName: my-proj")
		assert.Contains(t, string(data), "projectVersion: v1.0.0")
		assert.Contains(t, string(data), "requireTrace: true")
		assert.Contains(t, string(data), "workflowName: my-flow")
	})
}

func TestResolveContract(t *testing.T) {
	testCases := []struct {
		name         string
		flag         string
		want         string
		wantRequired bool
	}{
		{name: "defaults to the shipped contract, which may be absent", want: "chainloop-ai-coding-session"},
		{name: "the flag wins and is required", flag: "my-contract", want: "my-contract", wantRequired: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			name, required := ResolveContract(tc.flag)
			assert.Equal(t, tc.want, name)
			assert.Equal(t, tc.wantRequired, required)
		})
	}
}
