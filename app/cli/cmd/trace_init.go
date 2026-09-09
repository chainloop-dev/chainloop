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
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/app/cli/pkg/action"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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

It installs the managed git hooks plus the hooks of the selected harnesses
(Claude Code when none is given), and creates the Chainloop workflow the
attestations target. Nothing is written to the repository until that workflow
exists, so you need to be logged in.

On a terminal it asks which organization and project to use, offering what
.chainloop.yml already holds so pressing Enter keeps it. A new project can be
named freely; the name is normalized to the lowercase, dash-separated form
Chainloop stores. It then asks which harnesses to trace, with Claude Code
ticked; use space to tick more. Passing --org, --project or a harness flag
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

			// The harness hooks come first of everything that is left behind.
			// They are what records a session, so a run where none of them lands
			// has achieved nothing, and stopping here leaves the configuration,
			// the git hooks and the initialized marker unwritten rather than
			// leaving a repository that looks set up and records nothing.
			//
			// Pre-push infers the owning harness per-session from the recorded
			// SessionRecord, so the list isn't persisted anywhere — only each
			// harness's own hook config file, written here, determines which of
			// them can register sessions.
			installed := make([]trace.Provider, 0, len(selectedProviders))
			// before holds each harness's configuration as it stood, so a later
			// failure puts back exactly what was there. Uninstalling instead
			// would strip the hooks of a repository that was already set up,
			// since installing over them changes nothing and still counts.
			before := make([]harnessConfig, 0, len(selectedProviders))

			for _, p := range selectedProviders {
				snapshot, err := readHarnessConfig(p, repoRoot)
				if err != nil {
					logger.Warn().Err(err).Str("harness", p.Name()).Msg("could not read the harness configuration")
					continue
				}

				if err := p.InstallHooks(repoRoot); err != nil {
					logger.Warn().Err(err).Str("harness", p.Name()).Msg("could not install harness hooks")
					continue
				}

				logger.Debug().Str("harness", p.Name()).Msg("harness hooks installed")
				installed = append(installed, p)
				before = append(before, snapshot)
			}

			if len(installed) == 0 {
				return fmt.Errorf("no harness hooks could be installed, so no sessions would be recorded")
			}

			// The workflow exists and something records into it. What remains is
			// local, but a failure in any of it would leave harness hooks behind
			// calling a repository that is not set up.
			if err := writeTraceInitState(cfg, repoRoot, gitDir); err != nil {
				for _, snapshot := range before {
					if rerr := snapshot.restore(); rerr != nil {
						logger.Debug().Err(rerr).Str("path", snapshot.path).
							Msg("could not put the harness configuration back")
					}
				}

				return err
			}

			workDir, err := os.Getwd()
			if err != nil {
				// Only used to render paths relative to where the user is; the
				// repository root still names them correctly.
				workDir = repoRoot
			}

			// Nothing pinned an organization when cfg has none, so the workflow
			// went to the one the CLI points at. That is the one to report: it is
			// what the connection used, not a guess.
			organization := cmp.Or(cfg.organization, viper.GetString(confOptions.organization.viperKey))

			// What the run produced, and what to do with it. Every step above logs
			// at debug, so this is what the user is left with.
			writeTraceInitSummary(os.Stdout, organization, cfg.project, cfg.workflow, providerNames(installed))
			writeTraceNextSteps(os.Stdout, repoRoot, workDir, installed)

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "chainloop project name")
	cmd.Flags().StringVar(&contract, "contract", "", traceContractFlagDesc)
	cmd.Flags().String("workflow", "", "chainloop workflow name used for trace attestations (defaults to \"ai-coding-session\")")
	cmd.Flags().Bool("require-trace", false, "block pushes when attestation fails for AI-assisted commits")
	cmd.Flags().BoolVar(&claudeFlag, "claude", false, "install Claude Code hooks (default when no harness flag is set)")
	cmd.Flags().BoolVar(&cursorFlag, "cursor", false, "install Cursor hooks")
	cmd.Flags().BoolVar(&opencodeFlag, "opencode", false, "install opencode hooks")

	return cmd
}

// traceDocsURL is the guide covering what this command set up and what can be
// done with it, which is more than belongs in a command's own output.
const traceDocsURL = "https://docs.chainloop.dev/guides/chainloop-trace"

// writeTraceInitSummary reports what the repository was set up with. It goes to
// stdout rather than through the logger because it is the command's result,
// not a note about something that happened on the way there.
func writeTraceInitSummary(w io.Writer, organization, project, workflow string, harnesses []string) {
	fmt.Fprint(w, "\nCongratulations, your repository is initialized\n\n")

	// Empty only when the CLI has no organization configured either, which
	// leaves nothing truthful to name.
	if organization != "" {
		fmt.Fprintf(w, "  organization  %s\n", organization)
	}

	fmt.Fprintf(w, "  project       %s\n", project)
	fmt.Fprintf(w, "  workflow      %s\n", workflow)
	fmt.Fprintf(w, "  harnesses     %s\n", strings.Join(harnesses, ", "))
}

// writeTraceNextSteps says what is left for the user to do. Committing comes
// first: what init wrote into the repository is shared, and until it is
// committed this setup exists in one working copy only.
//
// It stops short of promising that a teammate who pulls is set up: the git
// hooks live in .git/hooks, which git does not carry between clones, so they
// still have to run init themselves. What they gain is having nothing to answer
// when they do.
//
// workDir is where the user ran the command, which is what `git add` resolves
// its arguments against; it is not always the repository root.
func writeTraceNextSteps(w io.Writer, repoRoot, workDir string, installed []trace.Provider) {
	files := make([]string, 0, len(installed)+1)
	add := func(absolute string) {
		// An absolute path stages just as well, so it is the fallback for the
		// rare case where no relative one exists.
		if rel, err := filepath.Rel(workDir, absolute); err == nil {
			files = append(files, rel)
			return
		}

		files = append(files, absolute)
	}

	add(filepath.Join(repoRoot, config.ChainloopYMLName(repoRoot)))

	for _, p := range installed {
		add(p.SettingsFile(repoRoot))
	}

	fmt.Fprintf(w, `
What's next

  1. Commit these files. Teammates then only need to run chainloop trace init:
       git add %s
  2. Start %s and write some code
  3. Commit and push as usual. The AI coding sessions behind those commits are
     recorded and stored automatically

Learn more: %s
`, strings.Join(files, " "), joinWithOr(providerNames(installed)), traceDocsURL)
}

// providerNames names the harnesses, in the order they were installed.
func providerNames(installed []trace.Provider) []string {
	names := make([]string, 0, len(installed))
	for _, p := range installed {
		names = append(names, p.Name())
	}

	return names
}

// joinWithOr renders a list the way a sentence needs it: "a", "a or b", or
// "a, b or c".
func joinWithOr(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	default:
		return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
	}
}

// harnessConfig is a harness's configuration file as it stood before init
// touched it. Installing hooks merges into whatever is already there, and
// installing over hooks that are present changes nothing at all, so putting
// the file back is the only way to undo a run without taking a working setup
// with it.
type harnessConfig struct {
	path string
	// content is what the file held, and existed distinguishes an empty file
	// from one init created.
	content []byte
	mode    os.FileMode
	existed bool
}

// harnessSettings is the slice of a provider readHarnessConfig needs: where the
// harness keeps the configuration its hooks are written into.
type harnessSettings interface {
	SettingsFile(repoRoot string) string
}

// readHarnessConfig records a harness's configuration before it is written to.
func readHarnessConfig(p harnessSettings, repoRoot string) (harnessConfig, error) {
	path := p.SettingsFile(repoRoot)

	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return harnessConfig{path: path}, nil
	}

	if err != nil {
		return harnessConfig{}, err
	}

	// The mode is kept so restoring does not quietly widen or narrow it.
	info, err := os.Stat(path)
	if err != nil {
		return harnessConfig{}, err
	}

	return harnessConfig{path: path, content: content, mode: info.Mode().Perm(), existed: true}, nil
}

// restore puts the configuration back as it was, removing the file when init
// was what created it. An empty directory left over from creating it goes too;
// a directory holding anything else is left alone, which os.Remove does by
// refusing to remove it.
func (h harnessConfig) restore() error {
	if h.existed {
		return os.WriteFile(h.path, h.content, h.mode)
	}

	if err := os.Remove(h.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	_ = os.Remove(filepath.Dir(h.path))

	return nil
}

// writeTraceInitState leaves the repository set up: the configuration, the
// trace directory, the managed git hooks and the marker that says init ran. It
// is one step so its caller can undo the harness hooks if any part of it fails,
// rather than leaving a half-configured repository behind.
func writeTraceInitState(cfg *traceInitConfig, repoRoot, gitDir string) error {
	if err := cfg.save(repoRoot); err != nil {
		return err
	}

	store := state.NewGitStore(gitDir)
	if err := store.InitTraceDir(); err != nil {
		return fmt.Errorf("create trace directory: %w", err)
	}

	hooksDir, err := hooks.Install(gitDir, false)
	if err != nil {
		return err
	}

	logger.Debug().Str("path", hooksDir).Msg("git hooks installed (post-commit, pre-push)")

	if err := store.MarkTraceInitialized(); err != nil {
		return fmt.Errorf("mark trace initialized: %w", err)
	}

	return nil
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
