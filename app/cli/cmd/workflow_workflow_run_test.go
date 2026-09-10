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
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The workflow run command is reachable both as "run" and through its legacy
// "workflow-run" alias, but the usage/help output must always advertise the
// canonical "chainloop workflow run" path.
func TestWorkflowRunCommandPath(t *testing.T) {
	const (
		workflowCmd  = "workflow"
		canonicalRun = "chainloop workflow run"
	)

	testCases := []struct {
		name         string
		args         []string
		expectedPath string
	}{
		{
			name:         "canonical name",
			args:         []string{workflowCmd, "run"},
			expectedPath: canonicalRun,
		},
		{
			name:         "legacy workflow-run alias",
			args:         []string{workflowCmd, "workflow-run"},
			expectedPath: canonicalRun,
		},
		{
			name:         "canonical name through the workflow alias",
			args:         []string{"wf", "run"},
			expectedPath: canonicalRun,
		},
		{
			name:         "subcommand keeps the canonical parent path",
			args:         []string{workflowCmd, "workflow-run", "describe"},
			expectedPath: "chainloop workflow run describe",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := &cobra.Command{Use: appName}
			root.AddCommand(newWorkflowCmd())

			cmd, _, err := root.Find(tc.args)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedPath, cmd.CommandPath())
		})
	}
}
