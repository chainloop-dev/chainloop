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
	"fmt"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/opencode"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

// NewTraceInitCmd creates the trace init subcommand.
func newTraceInitCmd() *cobra.Command {
	var (
		project      string
		claudeFlag   bool
		cursorFlag   bool
		opencodeFlag bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize git hooks for automatic AI session tracing",
		Long: `Initialize git hooks that automatically trace AI coding sessions and
create Chainloop attestations when you push.

This installs the managed git hooks plus agent-specific hooks for the
selected providers. Pass --claude, --cursor, and/or --opencode to pick
providers; when none is set, Claude Code is used as the default.

Pass --org to pin the Chainloop organization trace attestations target; the
value is saved to .chainloop.yml and overrides the CLI default on every push.

Pass --workflow to override the workflow name used when initializing trace
attestations. The value is saved to .chainloop.yml and defaults to
"ai-coding-session" when unset.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gitDir, repoRoot, err := tracegit.FindGitDirAndRoot()
			if err != nil {
				return err
			}

			// If --project was given, persist it to .chainloop.yml
			if project != "" {
				if err := config.SaveProjectToYML(repoRoot, project); err != nil {
					return err
				}
				logger.Info().Str("project", project).Msg("project name saved to .chainloop.yml")
			} else {
				project = config.LoadProjectFromYML(repoRoot)
			}
			if project == "" {
				return fmt.Errorf("--project is required (or add projectName to .chainloop.yml)")
			}

			// When the user explicitly passes the inherited persistent --org flag,
			// persist it to .chainloop.yml so trace hooks target it on every push.
			if cmd.Flags().Changed("org") {
				orgFlag, err := cmd.Flags().GetString("org")
				if err != nil {
					return fmt.Errorf("reading --org flag: %w", err)
				}
				if err := config.SaveOrganizationToYML(repoRoot, orgFlag); err != nil {
					return err
				}
				logger.Info().Str("organization", orgFlag).Msg("organization saved to .chainloop.yml")
			}

			if cmd.Flags().Changed("workflow") {
				workflowFlag, err := cmd.Flags().GetString("workflow")
				if err != nil {
					return fmt.Errorf("reading --workflow flag: %w", err)
				}
				if err := config.SaveWorkflowToYML(repoRoot, workflowFlag); err != nil {
					return err
				}
				logger.Info().Str("workflow", workflowFlag).Msg("workflow saved to .chainloop.yml")
			}
			workflowName := config.ResolveWorkflowName(config.LoadWorkflowFromYML(repoRoot))

			// Persist --require-trace when explicitly set, otherwise
			// load the persisted value from .chainloop.yml.
			requireTrace := config.LoadRequireTraceFromYML(repoRoot)
			if cmd.Flags().Changed("require-trace") {
				val, err := cmd.Flags().GetBool("require-trace")
				if err != nil {
					return fmt.Errorf("reading --require-trace flag: %w", err)
				}
				requireTrace = val
				if err := config.SaveRequireTraceToYML(repoRoot, requireTrace); err != nil {
					return err
				}
			}

			// Verify authentication. If the repo pins an organization, use it so
			// the auth check reflects the org the hooks will actually target.
			var authExecOpts []action.ExecutorOption
			if forcedOrg := config.LoadOrganizationFromYML(repoRoot); forcedOrg != "" {
				authExecOpts = append(authExecOpts, action.WithForcedOrganization(forcedOrg))
			}
			executor, err := action.NewAttestationExecutor(ActionOpts, Version, authExecOpts...)
			if err != nil {
				return err
			}
			defer func() { _ = executor.Close() }()

			if err := executor.CheckAuth(cmd.Context()); err != nil {
				logger.Warn().Err(err).Msg("authentication check failed")
				if requireTrace {
					logger.Warn().Msg("require-trace is enabled: pushes with AI-assisted commits will fail until authenticated")
				} else {
					logger.Warn().Msg("hooks will be installed, but attestation commands will fail until authenticated")
				}
			}

			// Create trace directory structure
			store := state.NewGitStore(gitDir)
			if err := store.InitTraceDir(); err != nil {
				return fmt.Errorf("create trace directory: %w", err)
			}

			// Install git hooks
			hooksDir, err := hooks.Install(gitDir, false)
			if err != nil {
				return err
			}
			logger.Info().
				Str("path", hooksDir).
				Msg("git hooks installed (post-commit, pre-push)")

			// Mark trace as initialized
			if err := store.MarkTraceInitialized(); err != nil {
				return fmt.Errorf("mark trace initialized: %w", err)
			}

			// Resolve which providers to install. Pre-push will infer the
			// owning provider per-session from the recorded SessionRecord,
			// so the list isn't persisted anywhere — only the agent-side
			// hook config files (.claude/settings.json, .cursor/hooks.json)
			// determine which providers can register sessions.
			selected := selectedTraceProviders(claudeFlag, cursorFlag, opencodeFlag)
			selectedProviders := providers.ByNames(selected)
			if len(selectedProviders) == 0 {
				return fmt.Errorf("no trace providers selected")
			}

			for _, p := range selectedProviders {
				if err := p.InstallHooks(repoRoot); err != nil {
					logger.Warn().Err(err).Str("provider", p.Name()).Msg("could not install agent hooks")
					continue
				}
				logger.Info().Str("provider", p.Name()).Msg("agent hooks installed")
			}

			logger.Info().
				Str("project", project).
				Str("workflow", workflowName).
				Strs("providers", selected).
				Msg("trace initialized")

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "chainloop project name")
	cmd.Flags().String("workflow", "", "chainloop workflow name used for trace attestations (defaults to \"ai-coding-session\")")
	cmd.Flags().Bool("require-trace", false, "block pushes when attestation fails for AI-assisted commits")
	cmd.Flags().BoolVar(&claudeFlag, "claude", false, "install Claude Code hooks (default when no provider flag is set)")
	cmd.Flags().BoolVar(&cursorFlag, "cursor", false, "install Cursor hooks")
	cmd.Flags().BoolVar(&opencodeFlag, "opencode", false, "install opencode hooks")

	return cmd
}

// selectedTraceProviders resolves the provider names to install based on the
// --claude, --cursor, and --opencode flags. When none is set, Claude Code is
// used as the default so existing users get the same behavior.
func selectedTraceProviders(claudeFlag, cursorFlag, opencodeFlag bool) []string {
	if !claudeFlag && !cursorFlag && !opencodeFlag {
		return []string{providers.DefaultProvider}
	}

	var out []string
	if claudeFlag {
		out = append(out, claude.Name)
	}
	if cursorFlag {
		out = append(out, cursor.Name)
	}
	if opencodeFlag {
		out = append(out, opencode.Name)
	}

	return out
}
