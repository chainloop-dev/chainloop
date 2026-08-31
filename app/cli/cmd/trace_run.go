package cmd

import (
	"errors"
	"fmt"
	"os"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

const traceRunLong = `Wrap an external command (typically an AI coding agent) and emit a
Chainloop attestation for the session once the command exits successfully.
Sessions without git commits are still attested.

trace run is isolated: it ignores .chainloop.yml entirely. The attestation
identity must come from the --org, --project, --workflow flags (mandatory)
and the optional --version flag. It does not read from or write to
.chainloop.yml, and removes every hook and local trace artifact when the
wrapped command exits, so a session never leaks setup into the next one.`

// NewTraceRunCmd creates the `trace run` subcommand.
func newTraceRunCmd() *cobra.Command {
	var (
		projectFlag  string
		workflowFlag string
		versionFlag  string
		claudeFlag   bool
		cursorFlag   bool
		opencodeFlag bool
	)

	cmd := &cobra.Command{
		Use:   "run -- <command> [args...]",
		Short: "Run a single-shot AI coding session and attest it",
		Long:  traceRunLong,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Outside a git repository trace state lives in a deterministic
			// out-of-tree directory keyed by the working directory. The store
			// carries which of the two it is, and an out-of-tree one disables
			// the git hooks, which git could never invoke anyway.
			gitDir, repoRoot, err := tracegit.FindGitDirAndRoot()

			var store *state.Store
			switch {
			case err != nil && !errors.Is(err, tracegit.ErrNotARepository):
				// A repository is there but unusable — don't paper over it
				// by silently dropping commit attribution.
				return err
			case err != nil:
				cwd, cwdErr := os.Getwd()
				if cwdErr != nil {
					return fmt.Errorf("get working directory: %w", cwdErr)
				}

				stateDir, dirErr := state.NonGitDir(cwd)
				if dirErr != nil {
					return dirErr
				}

				store, repoRoot = state.NewOutOfTreeStore(stateDir), cwd
				logger.Info().Msg("no git repository found; recording the session without commit attribution")
			default:
				store = state.NewGitStore(gitDir)
			}

			// --org is a persistent flag declared on the root command,
			// so we cannot MarkFlagRequired it on this subcommand.
			organization, err := cmd.Flags().GetString("org")
			if err != nil {
				return fmt.Errorf("reading --org flag: %w", err)
			}

			// MarkFlagRequired and Changed() only check that a flag was
			// passed, so an empty value like --workflow "" would slip
			// through and silently fall back to the default workflow.
			// Validate the actual values here.
			if organization == "" {
				return fmt.Errorf("--org is required for trace run")
			}
			if projectFlag == "" {
				return fmt.Errorf("--project is required for trace run")
			}
			if workflowFlag == "" {
				return fmt.Errorf("--workflow is required for trace run")
			}

			return action.TraceRun(cmd.Context(), logger, action.TraceRunOpts{
				Store:          store,
				RepoRoot:       repoRoot,
				Providers:      selectedTraceProviders(claudeFlag, cursorFlag, opencodeFlag),
				Command:        args,
				ProjectName:    projectFlag,
				Organization:   organization,
				WorkflowName:   workflowFlag,
				ProjectVersion: versionFlag,
				ActionOpts:     ActionOpts,
				CLIVersion:     Version,
			})
		},
	}

	cmd.Flags().StringVar(&projectFlag, "project", "", "chainloop project name (required; .chainloop.yml is ignored)")
	cmd.Flags().StringVar(&workflowFlag, "workflow", "", "chainloop workflow name used for trace attestations (required; .chainloop.yml is ignored)")
	cmd.Flags().StringVar(&versionFlag, "version", "", "chainloop project version (optional; defaults to the latest version)")
	cmd.Flags().BoolVar(&claudeFlag, "claude", false, "install Claude Code hooks (default when no provider flag is set)")
	cmd.Flags().BoolVar(&cursorFlag, "cursor", false, "install Cursor hooks")
	cmd.Flags().BoolVar(&opencodeFlag, "opencode", false, "install opencode hooks")

	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("workflow")

	return cmd
}
