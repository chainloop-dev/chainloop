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

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// traceInitTestCmd builds the trace init command with the root command's
// persistent --org flag, which resolveTraceInitConfig reads.
func traceInitTestCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()

	cmd := newTraceInitCmd()
	cmd.Flags().String("org", "", "organization name")
	cmd.SetArgs(args)
	require.NoError(t, cmd.ParseFlags(args))

	return cmd
}

// defaultTraceWorkflow is the workflow name trace init falls back to.
const defaultTraceWorkflow = "ai-coding-session"

// The files trace init leaves in the repository: the config it writes and the
// harness hooks it installs. The closing message has to name them so they get
// committed.
const (
	chainloopYMLName = ".chainloop.yml"
	claudeSettings   = ".claude/settings.json"
)

func TestResolveTraceInitConfig(t *testing.T) {
	const existingYML = "projectName: yml-project\norganization: yml-org\nworkflowName: yml-workflow\nrequireTrace: true\n"

	testCases := []struct {
		name        string
		yml         string
		args        []string
		projectFlag string

		wantErr          string
		wantProject      string
		wantWorkflow     string
		wantOrganization string
		wantRequireTrace bool
		wantSaved        []string
	}{
		{
			name:         "values come from .chainloop.yml when no flag is passed",
			yml:          existingYML,
			wantProject:  "yml-project",
			wantWorkflow: "yml-workflow",
			// The organization pins where the workflow is created, so it is
			// read back even when the user did not pass --org. requireTrace is
			// not, since it is only needed when it is being written.
			wantOrganization: "yml-org",
		},
		{
			name:         "the workflow name falls back to the trace default",
			yml:          "projectName: yml-project\n",
			wantProject:  "yml-project",
			wantWorkflow: defaultTraceWorkflow,
		},
		{
			name:             "flags win and are marked for saving",
			yml:              existingYML,
			args:             []string{"--workflow", "flag-workflow", "--org", "flag-org", "--require-trace"},
			projectFlag:      "flag-project",
			wantProject:      "flag-project",
			wantWorkflow:     "flag-workflow",
			wantOrganization: "flag-org",
			wantRequireTrace: true,
			wantSaved:        []string{"project", "workflow", "organization", "requireTrace"},
		},
		{
			name:             "require-trace can be turned off explicitly",
			yml:              existingYML,
			args:             []string{"--require-trace=false"},
			wantProject:      "yml-project",
			wantWorkflow:     "yml-workflow",
			wantOrganization: "yml-org",
			wantSaved:        []string{"requireTrace"},
		},
		{
			// Resolving no longer fails on a missing project: the interactive
			// path asks for one, and the non-interactive path reports it. See
			// TestResolveTraceIdentityNonInteractive.
			name:        "a missing project name is left for the identity step",
			yml:         "projectVersion: v1.0.0\n",
			wantProject: "",
			// The workflow still gets its default.
			wantWorkflow: defaultTraceWorkflow,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, chainloopYMLName)
			require.NoError(t, os.WriteFile(path, []byte(tc.yml), 0600))

			cfg, err := resolveTraceInitConfig(traceInitTestCmd(t, tc.args...), dir, tc.projectFlag)

			// Resolving must not touch the repository: the workflow is created
			// in between, and a failed init leaves nothing behind.
			data, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, tc.yml, string(data), "resolving must not write to .chainloop.yml")

			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantProject, cfg.project)
			assert.Equal(t, tc.wantWorkflow, cfg.workflow)
			assert.Equal(t, tc.wantOrganization, cfg.organization)
			assert.Equal(t, tc.wantRequireTrace, cfg.requireTrace)

			saved := map[string]bool{
				"project":      cfg.saveProject,
				"workflow":     cfg.saveWorkflow,
				"organization": cfg.saveOrganization,
				"requireTrace": cfg.saveRequireTrace,
			}
			for field, got := range saved {
				assert.Equal(t, slices.Contains(tc.wantSaved, field), got, "save flag for %s", field)
			}
		})
	}
}

func TestTraceInitConfigSave(t *testing.T) {
	t.Run("only the values the user passed are written", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, chainloopYMLName)
		require.NoError(t, os.WriteFile(path, []byte("projectName: yml-project\n"), 0600))

		cfg := &traceInitConfig{
			project:      "yml-project",
			workflow:     "ai-coding-session",
			organization: "yml-org",
			saveWorkflow: true,
		}
		require.NoError(t, cfg.save(dir))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(data), "workflowName: ai-coding-session")
		assert.NotContains(t, string(data), "organization:")
	})
}

// TestWriteTraceNextSteps checks the closing message names the files that have
// to be committed. Everything init wrote lives in the repository, so leaving
// them uncommitted keeps the tracing on one working copy.
func TestWriteTraceNextSteps(t *testing.T) {
	testCases := []struct {
		name      string
		providers []string
		// wantFiles are the paths the git add line must carry
		wantFiles []string
		// wantHarnesses is how the harnesses are named in the sentence
		wantHarnesses string
	}{
		{
			name:          "one harness",
			providers:     []string{providerClaudeCode},
			wantFiles:     []string{chainloopYMLName, claudeSettings},
			wantHarnesses: providerClaudeCode,
		},
		{
			name:          "two harnesses",
			providers:     []string{providerClaudeCode, "cursor"},
			wantFiles:     []string{chainloopYMLName, claudeSettings, ".cursor/hooks.json"},
			wantHarnesses: "claude-code or cursor",
		},
		{
			name:      "every harness",
			providers: []string{providerClaudeCode, "cursor", "opencode"},
			// Named in full: ".opencode" alone would pass on any file under that
			// directory, so it would not catch the plugin being renamed.
			wantFiles: []string{
				chainloopYMLName, claudeSettings, ".cursor/hooks.json",
				".opencode/plugins/chainloop-trace.ts",
			},
			wantHarnesses: "claude-code, cursor or opencode",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			repoRoot := t.TempDir()
			writeTraceNextSteps(out, repoRoot, repoRoot, providers.ByNames(tc.providers))

			got := out.String()
			for _, f := range tc.wantFiles {
				assert.Contains(t, got, f, "the file has to be committed, so it has to be named")
			}

			assert.Contains(t, got, tc.wantHarnesses)
			assert.Contains(t, got, traceDocsURL)
		})
	}
}

// TestWriteTraceNextStepsFromSubdirectory pins down that the git add line works
// where it is printed. Its arguments are resolved against the working
// directory, so repository-root paths would stage nothing from a subdirectory.
func TestWriteTraceNextStepsFromSubdirectory(t *testing.T) {
	repoRoot := t.TempDir()
	workDir := filepath.Join(repoRoot, "services", "api")
	require.NoError(t, os.MkdirAll(workDir, 0750))

	out := &bytes.Buffer{}
	writeTraceNextSteps(out, repoRoot, workDir, providers.ByNames([]string{providerClaudeCode}))

	got := out.String()
	assert.Contains(t, got, filepath.Join("..", "..", chainloopYMLName))
	assert.Contains(t, got, filepath.Join("..", "..", claudeSettings))
}

func TestWriteTraceInitSummary(t *testing.T) {
	t.Run("every value is reported", func(t *testing.T) {
		out := &bytes.Buffer{}
		writeTraceInitSummary(out, "acme", "backend-api", defaultTraceWorkflow,
			[]string{providerClaudeCode, "cursor"})

		got := out.String()
		assert.Contains(t, got, "your repository is initialized")
		assert.Contains(t, got, "acme")
		assert.Contains(t, got, "backend-api")
		assert.Contains(t, got, defaultTraceWorkflow)
		assert.Contains(t, got, "claude-code, cursor")
	})

	t.Run("no organization anywhere leaves the line out rather than guessing", func(t *testing.T) {
		out := &bytes.Buffer{}
		writeTraceInitSummary(out, "", "backend-api", defaultTraceWorkflow, []string{providerClaudeCode})

		assert.NotContains(t, out.String(), "organization")
	})
}

func TestJoinWithOr(t *testing.T) {
	testCases := []struct {
		names []string
		want  string
	}{
		{names: nil, want: ""},
		{names: []string{"a"}, want: "a"},
		{names: []string{"a", "b"}, want: "a or b"},
		{names: []string{"a", "b", "c"}, want: "a, b or c"},
	}

	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, joinWithOr(tc.names))
		})
	}
}
