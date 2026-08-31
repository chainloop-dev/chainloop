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
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

// NewTraceUninstallCmd creates the trace uninstall subcommand.
func newTraceUninstallCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove trace git hooks and configuration",
		Annotations: map[string]string{
			"skipActionOptsInit": "true",
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			gitDir, err := tracegit.FindGitDir()
			if err != nil {
				return err
			}

			if !yes {
				if !confirmationPrompt("This will remove trace git hooks and configuration. Are you sure?") {
					return nil
				}
			}

			repoRoot, _ := tracegit.RepoRoot()
			if err := action.CleanupTrace(state.NewGitStore(gitDir), repoRoot, logger); err != nil {
				return err
			}
			logger.Info().Msg("trace uninstalled")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")

	return cmd
}
