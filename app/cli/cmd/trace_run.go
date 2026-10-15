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
	"errors"
	"fmt"
	"os"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
)

const traceRunLong = `Wrap an external command (typically an AI coding agent) and emit a
Chainloop attestation for the session once the command exits successfully.
Sessions without git commits are still attested.

trace run does not read the attestation identity from .chainloop.yml. The
--org, --project and --workflow flags (mandatory) and the optional --version
flag set it.

trace run reads one value from .chainloop.yml: traceStorage. In a local-mode
repository (traceStorage: LOCAL), trace run writes the session to disk when the
command exits. It does not connect to a control plane, and the --org, --project
and --workflow flags are optional.

When the wrapped command exits, trace run removes the temporary hooks and the
trace state. It keeps the stored local session evidence. Thus no setup from one
session stays for the next session.

The workflow is created before the command runs. That part is best-effort: the
session runs either way and the attestation creates the workflow if this did
not, except with --contract, which trace run cannot honor later.`

// newTraceRunCmd creates the `trace run` subcommand.
func newTraceRunCmd() *cobra.Command {
	var (
		projectFlag  string
		workflowFlag string
		versionFlag  string
		contractFlag string
		claudeFlag   bool
		cursorFlag   bool
		opencodeFlag bool
	)

	cmd := &cobra.Command{
		Use:   "run -- <command> [args...]",
		Short: "Run a single-shot AI coding session and attest it",
		Long:  traceRunLong,
		Args:  cobra.MinimumNArgs(1),
		// skipActionOptsInit keeps root from building the control-plane
		// connection up front; the attestation path builds it lazily with
		// ensureActionOpts, and a local-mode run never does (R-005).
		Annotations: map[string]string{skipActionOptsInit: trueString},
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
				store = state.NewRepoStore(gitDir, repoRoot)
			}

			// The storage mode is a repository policy, read even though trace
			// run keeps its identity flag-only: a LOCAL repository stores to
			// disk and needs no identity and no control plane.
			storageMode, err := config.LoadTraceStorageFromYML(repoRoot)
			if err != nil {
				return err
			}
			local := storageMode == config.TraceStorageLocal

			var (
				organization     string
				contractName     string
				contractRequired bool
			)

			if !local {
				// A control-plane run is reported in telemetry as it was before
				// local mode made this command skipActionOptsInit. A local run
				// is not: it leaves the annotation unset and stays excluded.
				cmd.Annotations[reportTelemetryAnyway] = trueString

				// --org is a persistent flag declared on the root command,
				// so we cannot MarkFlagRequired it on this subcommand.
				organization, err = cmd.Flags().GetString("org")
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

				contractName, contractRequired = config.ResolveContract(contractFlag)

				// Build ActionOpts lazily (this command is skipActionOptsInit),
				// so a LOCAL run never opens a control-plane connection.
				if err := ensureActionOpts(cmd); err != nil {
					return err
				}
			}

			return action.TraceRun(cmd.Context(), logger, action.TraceRunOpts{
				Store:            store,
				RepoRoot:         repoRoot,
				Providers:        traceProviderFlags{claude: claudeFlag, cursor: cursorFlag, opencode: opencodeFlag}.namesOrDefault(),
				Command:          args,
				ProjectName:      projectFlag,
				Organization:     organization,
				WorkflowName:     workflowFlag,
				ProjectVersion:   versionFlag,
				ContractName:     contractName,
				ContractRequired: contractRequired,
				ActionOpts:       ActionOpts,
				CLIVersion:       Version,
				Local:            local,
			})
		},
	}

	cmd.Flags().StringVar(&projectFlag, "project", "", "chainloop project name (required, except in local mode). trace run does not read it from .chainloop.yml")
	cmd.Flags().StringVar(&workflowFlag, "workflow", "", "chainloop workflow name for the trace attestations (required, except in local mode). trace run does not read it from .chainloop.yml")
	cmd.Flags().StringVar(&versionFlag, "version", "", "chainloop project version (optional; defaults to the latest version)")
	cmd.Flags().StringVar(&contractFlag, "contract", "", traceContractFlagDesc)
	cmd.Flags().BoolVar(&claudeFlag, "claude", false, "install Claude Code hooks (default when no provider flag is set)")
	cmd.Flags().BoolVar(&cursorFlag, "cursor", false, "install Cursor hooks")
	cmd.Flags().BoolVar(&opencodeFlag, "opencode", false, "install opencode hooks")

	// project and workflow are not required in local mode, so the check is done
	// in RunE (after the storage mode is known) rather than by cobra up front.

	return cmd
}
