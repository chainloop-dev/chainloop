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
	"os"
	"path/filepath"
	"slices"
	"testing"

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
			wantWorkflow: "ai-coding-session",
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
			wantWorkflow: "ai-coding-session",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, ".chainloop.yml")
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
		path := filepath.Join(dir, ".chainloop.yml")
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
