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
	"context"
	"fmt"
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

// newTraceInitCmd creates the trace init subcommand.
func newTraceInitCmd() *cobra.Command {
	var (
		project      string
		contract     string
		claudeFlag   bool
		cursorFlag   bool
		opencodeFlag bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize git hooks for automatic AI session tracing",
		Long: `Initialize git hooks that automatically trace AI coding sessions and
create Chainloop attestations when you push.

It installs the managed git hooks plus the hooks of the selected agent
providers (Claude Code when none is given), and creates the Chainloop workflow
the attestations target. Nothing is written to the repository until that
workflow exists, so you need to be logged in.

On a terminal it asks which organization and project to use, offering what
.chainloop.yml already holds so pressing Enter keeps it. A new project can be
named freely; the name is normalized to the lowercase, dash-separated form
Chainloop stores. It then asks which agents to trace, with Claude Code
ticked; use space to tick more. Passing --org, --project or a provider flag
(--claude, --cursor, --opencode) skips the matching question.

Nothing is asked in CI or when the output is redirected: there --project is
required unless .chainloop.yml carries projectName. Set CHAINLOOP_NO_PROMPT to
turn the questions off on a terminal too.

The organization, project, workflow and require-trace values are saved to
.chainloop.yml, and every push reads them from there.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gitDir, repoRoot, err := tracegit.FindGitDirAndRoot()
			if err != nil {
				return err
			}

			cfg, err := resolveTraceInitConfig(cmd, repoRoot, project)
			if err != nil {
				return err
			}

			// Fill in whatever the flags and .chainloop.yml did not settle,
			// asking the user when there is one to ask. The executor it hands
			// back is pinned to the resolved organization and reused below.
			executor, err := resolveTraceIdentity(cmd.Context(), cfg, repoRoot)
			if err != nil {
				return stopIfAborted(err)
			}
			defer func() { _ = executor.Close() }()

			// Ask which agents to trace before anything is created or written,
			// so every question is answered up front and an abort leaves the
			// repository untouched.
			selected, err := resolveTraceProviders(newHuhPrompter(os.LookupEnv),
				traceProviderFlags{claude: claudeFlag, cursor: cursorFlag, opencode: opencodeFlag},
				traceInitCanPrompt())
			if err != nil {
				return stopIfAborted(err)
			}

			selectedProviders := providers.ByNames(selected)
			if len(selectedProviders) == 0 {
				return fmt.Errorf("no trace providers selected")
			}

			if err := ensureTraceInitWorkflow(cmd.Context(), executor, cfg, contract); err != nil {
				return err
			}

			// The workflow exists: from here on it is safe to leave hooks and
			// configuration behind.
			if err := cfg.save(repoRoot); err != nil {
				return err
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
			logger.Debug().
				Str("path", hooksDir).
				Msg("git hooks installed (post-commit, pre-push)")

			// Mark trace as initialized
			if err := store.MarkTraceInitialized(); err != nil {
				return fmt.Errorf("mark trace initialized: %w", err)
			}

			// Install the agent-side hooks for the providers resolved above.
			// Pre-push infers the owning provider per-session from the recorded
			// SessionRecord, so the list isn't persisted anywhere — only each
			// agent's own hook config file, written here, determines which
			// providers can register sessions.
			for _, p := range selectedProviders {
				if err := p.InstallHooks(repoRoot); err != nil {
					logger.Warn().Err(err).Str("provider", p.Name()).Msg("could not install agent hooks")
					continue
				}
				logger.Debug().Str("provider", p.Name()).Msg("agent hooks installed")
			}

			// The one line worth printing on a successful run: every step above
			// logs at debug, so this is what the user is left with. The
			// organization is absent when nothing pinned one and the CLI's own
			// default was used, in which case naming it here would be a guess.
			done := logger.Info()
			if cfg.organization != "" {
				done = done.Str("organization", cfg.organization)
			}

			done.
				Str("project", cfg.project).
				Str("workflow", cfg.workflow).
				Strs("providers", selected).
				Msg("repository initialized")

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "chainloop project name")
	cmd.Flags().StringVar(&contract, "contract", "", traceContractFlagDesc)
	cmd.Flags().String("workflow", "", "chainloop workflow name used for trace attestations (defaults to \"ai-coding-session\")")
	cmd.Flags().Bool("require-trace", false, "block pushes when attestation fails for AI-assisted commits")
	cmd.Flags().BoolVar(&claudeFlag, "claude", false, "install Claude Code hooks (default when no provider flag is set)")
	cmd.Flags().BoolVar(&cursorFlag, "cursor", false, "install Cursor hooks")
	cmd.Flags().BoolVar(&opencodeFlag, "opencode", false, "install opencode hooks")

	return cmd
}

// ensureTraceInitWorkflow makes sure the workflow the trace attestations target
// exists, creating it up front while someone is watching the output. Left to
// the pre-push hook, the control plane creates it implicitly with an empty
// contract and no one reads the result. Anything other than the workflow
// already existing is fatal, so a failed init leaves nothing behind.
//
// It runs on the executor resolveTraceIdentity opened, which is already pinned
// to the resolved organization and authenticated.
func ensureTraceInitWorkflow(ctx context.Context, executor *action.AttestationExecutor, cfg *traceInitConfig, contractFlag string) error {
	contractName, contractRequired := config.ResolveContract(contractFlag)
	wf, err := executor.EnsureWorkflow(ctx, action.EnsureTraceWorkflowOpts{
		ProjectName:      cfg.project,
		WorkflowName:     cfg.workflow,
		ContractName:     contractName,
		ContractRequired: contractRequired,
	})
	if err != nil {
		return err
	}

	if wf.Created {
		// The workflow name is the same in every project, so the project is what
		// identifies what was just created.
		logger.Debug().
			Str("project", cfg.project).
			Str("workflow", cfg.workflow).
			Str("contract", wf.ContractName).
			Msg("workflow created")
	}

	return nil
}

// traceInitConfig is the trace setup resolved from the command flags and the
// existing .chainloop.yml. Resolving and saving are separate steps so nothing
// lands in the user's repository before the Chainloop workflow exists.
type traceInitConfig struct {
	project      string
	workflow     string
	organization string
	requireTrace bool

	// The save* fields mark the values the user set on this run. Only those
	// are written back to .chainloop.yml, leaving the rest of the file alone.
	saveProject, saveWorkflow, saveOrganization, saveRequireTrace bool
}

// resolveTraceInitConfig reads the trace settings from the flags, falling back
// to the values already in .chainloop.yml.
func resolveTraceInitConfig(cmd *cobra.Command, repoRoot, projectFlag string) (*traceInitConfig, error) {
	// The organization is read back even when the user did not pass --org,
	// since it pins where the workflow is created. The other persisted values
	// are only needed when they are being written.
	cfg := &traceInitConfig{
		project:      projectFlag,
		saveProject:  projectFlag != "",
		organization: config.LoadOrganizationFromYML(repoRoot),
	}

	// A project may still be missing here. resolveTraceIdentity either asks for
	// one or, when nobody can be asked, reports that it is required.
	if cfg.project == "" {
		cfg.project = config.LoadProjectFromYML(repoRoot)
	}

	// --org is inherited from the root command, so it is only meant for this
	// repository when the user passed it explicitly.
	if cmd.Flags().Changed("org") {
		org, err := cmd.Flags().GetString("org")
		if err != nil {
			return nil, fmt.Errorf("reading --org flag: %w", err)
		}
		cfg.organization = org
		cfg.saveOrganization = true
	}

	workflow := config.LoadWorkflowFromYML(repoRoot)
	if cmd.Flags().Changed("workflow") {
		workflowFlag, err := cmd.Flags().GetString("workflow")
		if err != nil {
			return nil, fmt.Errorf("reading --workflow flag: %w", err)
		}
		workflow = workflowFlag
		cfg.saveWorkflow = true
	}
	cfg.workflow = config.ResolveWorkflowName(workflow)

	if cmd.Flags().Changed("require-trace") {
		requireTrace, err := cmd.Flags().GetBool("require-trace")
		if err != nil {
			return nil, fmt.Errorf("reading --require-trace flag: %w", err)
		}
		cfg.requireTrace = requireTrace
		cfg.saveRequireTrace = true
	}

	return cfg, nil
}

// save persists the values the user passed to .chainloop.yml so the trace
// hooks target the same organization, project and workflow on every push.
func (c *traceInitConfig) save(repoRoot string) error {
	if c.saveProject {
		if err := config.SaveProjectToYML(repoRoot, c.project); err != nil {
			return err
		}
		logger.Debug().Str("project", c.project).Msg("project name saved to .chainloop.yml")
	}

	if c.saveOrganization {
		if err := config.SaveOrganizationToYML(repoRoot, c.organization); err != nil {
			return err
		}
		logger.Debug().Str("organization", c.organization).Msg("organization saved to .chainloop.yml")
	}

	if c.saveWorkflow {
		if err := config.SaveWorkflowToYML(repoRoot, c.workflow); err != nil {
			return err
		}
		logger.Debug().Str("workflow", c.workflow).Msg("workflow saved to .chainloop.yml")
	}

	if c.saveRequireTrace {
		if err := config.SaveRequireTraceToYML(repoRoot, c.requireTrace); err != nil {
			return err
		}
	}

	return nil
}
