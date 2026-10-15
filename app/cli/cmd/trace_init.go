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

	"github.com/chainloop-dev/chainloop/app/cli/internal/repositoryconfig"
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
		project       string
		contract      string
		localFlag     bool
		tracePathFlag string
		claudeFlag    bool
		cursorFlag    bool
		opencodeFlag  bool
	)

	cmd := &cobra.Command{
		Use: "init", //nolint:goconst // cobra verb, kept literal like the other init subcommands
		// skipActionOptsInit keeps root from building the control-plane
		// connection up front. Control-plane mode builds it lazily with
		// ensureActionOpts below; local mode never does, so it reaches no
		// control plane at all (R-008).
		Annotations: map[string]string{skipActionOptsInit: trueString},
		Short:       "Initialize git hooks for automatic AI session tracing",
		Long: `Initialize git hooks that automatically trace AI coding sessions and
create Chainloop attestations when you push.

It installs the managed git hooks plus the hooks of the selected harnesses
(Claude Code when none is given), and creates the Chainloop workflow the
attestations target. Nothing is written to the repository until that workflow
exists, so you need to be logged in.

On a terminal it asks which organization and project to use, offering what
.chainloop.yml already holds so pressing Enter keeps it. A new project can be
named freely; the name is normalized to the lowercase, dash-separated form
Chainloop stores. It then asks which harnesses to trace, with the ones the
repository is already set up for ticked, or Claude Code on a first run; use
space to tick more. Passing --org, --project or a harness flag
(--claude, --cursor, --opencode) skips the matching question.

Nothing is asked in CI or when the output is redirected: there --project is
required unless .chainloop.yml carries projectName. Set CHAINLOOP_NO_PROMPT to
turn the questions off on a terminal too.

The organization, project, workflow and require-trace values are saved to
.chainloop.yml, and every push reads them from there.

With --local, init sets up the repository for local mode. It installs the same
hooks, but it creates no workflow, needs no login, and does not connect to a
control plane. It writes traceStorage: LOCAL to .chainloop.yml. Every clone
then records sessions on disk. On each push, the pre-push hook writes the
session evidence to the local-sessions folder of the trace directory. It does
not send an attestation.

--trace-path sets the trace directory, where Chainloop Trace keeps its data.
The default is .git/chainloop-trace. It works in every mode, and init writes it
to .chainloop.yml as tracePath. A relative path starts at the repository root.

--contract, --workflow, --org and --project are control-plane settings. You
cannot use them with --local. You can use --require-trace with --local.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gitDir, repoRoot, err := tracegit.FindGitDirAndRoot()
			if err != nil {
				return err
			}

			// Local mode contacts no control plane, so the control-plane
			// identity and workflow flags are contradictions rather than extra
			// configuration. --require-trace is orthogonal and allowed.
			if localFlag {
				for _, name := range []string{"contract", "workflow", "org", "project"} { //nolint:goconst // flag names, kept literal like the other flag references
					if cmd.Flags().Changed(name) {
						return fmt.Errorf("you cannot use --%s with --local: local mode does not connect to a control plane and does not create a workflow", name)
					}
				}
			}

			cfg, err := resolveTraceInitConfig(cmd, repoRoot, project)
			if err != nil {
				return err
			}
			cfg.local = localFlag
			cfg.tracePath = tracePathFlag
			cfg.saveTracePath = tracePathFlag != ""

			// A control-plane init over a repository previously set to LOCAL has
			// to clear the stale policy, or the pre-push hook keeps storing on
			// disk while init configured the control plane, and the two
			// disagree. A fresh control-plane repository writes nothing and keeps
			// relying on the CONTROL_PLANE default.
			existingMode, err := config.LoadTraceStorageFromYML(repoRoot)
			if err != nil && !localFlag {
				// An invalid traceStorage already in .chainloop.yml would make
				// every pre-push fail. A control-plane init cannot pick a mode
				// for a value it cannot read, so surface it now rather than
				// install hooks over a config that only errors. An explicit
				// --local run overrides the value and is let through.
				return fmt.Errorf("existing .chainloop.yml: %w", err)
			}
			cfg.traceStorage, cfg.saveTraceStorage = traceStorageToPersist(localFlag, existingMode)

			// Control-plane mode fills in the identity and opens an executor,
			// building the connection lazily (the command is skipActionOptsInit).
			// Local mode resolves no identity and opens no executor.
			var executor *action.AttestationExecutor
			if !localFlag {
				if err := ensureActionOpts(cmd); err != nil {
					return withLocalModeTip(err)
				}

				executor, err = resolveTraceIdentity(cmd.Context(), cfg, repoRoot)
				if err != nil {
					return withLocalModeTip(stopIfAborted(err))
				}
				defer func() { _ = executor.Close() }()
			}

			// Ask which agents to trace before anything is created or written,
			// so every question is answered up front and an abort leaves the
			// repository untouched.
			selected, err := resolveTraceProviders(newNamingPrompter(os.LookupEnv),
				traceProviderFlags{claude: claudeFlag, cursor: cursorFlag, opencode: opencodeFlag},
				traceInitCanPrompt(), func() []string { return configuredTraceProviders(repoRoot) })
			if err != nil {
				return stopIfAborted(err)
			}

			selectedProviders := providers.ByNames(selected)
			if len(selectedProviders) == 0 {
				return fmt.Errorf("no trace providers selected")
			}

			// Local mode has no control plane to create the workflow in.
			if !localFlag {
				if err := ensureTraceInitWorkflow(cmd.Context(), executor, cfg, contract); err != nil {
					return withLocalModeTip(err)
				}
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
			// before holds every repository file init is about to write, as it
			// stood, so a later failure puts back exactly what was there.
			// Uninstalling the hooks instead would strip a repository that was
			// already set up, since installing over hooks changes nothing and
			// still counts as installed.
			before := make([]fileSnapshot, 0, len(selectedProviders)+1)

			for _, p := range selectedProviders {
				snapshot, err := snapshotFile(p.SettingsFile(repoRoot))
				if err != nil {
					logger.Warn().Err(err).Str("harness", p.Name()).Msg("could not read the harness configuration")
					continue
				}

				if err := p.InstallHooks(repoRoot); err != nil {
					// A write that failed part-way through leaves the file as it
					// got to, so put it back before moving on.
					if rerr := snapshot.restore(); rerr != nil {
						logger.Debug().Err(rerr).Str("path", snapshot.path).
							Msg("could not put the harness configuration back")
					}

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

			// .chainloop.yml is written by the step below, which stops at the
			// first thing that fails, so it is recorded here alongside the rest.
			ymlSnapshot, err := snapshotFile(filepath.Join(repoRoot, config.ChainloopYMLName(repoRoot)))
			if err != nil {
				return err
			}

			before = append(before, ymlSnapshot)

			// The workflow exists and something records into it. What remains is
			// local, but a failure part-way through it would leave a repository
			// half set up: harness hooks calling a configuration that is not
			// there, or a configuration with nothing recording into it.
			if err := writeTraceInitState(cfg, repoRoot, gitDir); err != nil {
				for _, snapshot := range before {
					if rerr := snapshot.restore(); rerr != nil {
						logger.Debug().Err(rerr).Str("path", snapshot.path).
							Msg("could not put the file back")
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

			// What the run produced, and what to do with it. Every step above logs
			// at debug, so this is what the user is left with.
			if cfg.local {
				sessionsDir := state.NewRepoStore(gitDir, repoRoot).LocalSessionsDir()
				if rel, err := filepath.Rel(workDir, sessionsDir); err == nil {
					sessionsDir = rel
				}

				writeTraceInitLocalSummary(os.Stdout, sessionsDir, providerNames(installed))
				writeTraceLocalNextSteps(os.Stdout, repoRoot, workDir, installed)

				return nil
			}

			// Nothing pinned an organization when cfg has none, so the workflow
			// went to the one the CLI points at. That is the one to report: it is
			// what the connection used, not a guess.
			organization := cmp.Or(cfg.organization, viper.GetString(confOptions.organization.viperKey))

			writeTraceInitSummary(os.Stdout, organization, cfg.project, cfg.workflow, providerNames(installed))

			// A control-plane init that cleared a previous LOCAL policy changed
			// how every push behaves, so say it rather than leave it to a debug
			// log the user never sees.
			if existingMode == config.TraceStorageLocal {
				writeTraceStorageSwitchedNote(os.Stdout, config.ChainloopYMLName(repoRoot))
			}

			writeTraceNextSteps(os.Stdout, repoRoot, workDir, installed)

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "chainloop project name")
	cmd.Flags().StringVar(&contract, "contract", "", traceContractFlagDesc)
	cmd.Flags().BoolVar(&localFlag, "local", false, "set up local mode, with no control plane, login or workflow. The pre-push hook writes the session evidence to disk")
	cmd.Flags().StringVar(&tracePathFlag, "trace-path", "", "the directory where Chainloop Trace keeps its data, in every mode (default: .git/chainloop-trace). Local mode writes the sessions to its local-sessions folder")
	cmd.Flags().String("workflow", "", "chainloop workflow name used for trace attestations (defaults to \"ai-coding-session\")")
	cmd.Flags().Bool("require-trace", false, "block pushes when a required trace is missing")
	cmd.Flags().BoolVar(&claudeFlag, "claude", false, "install Claude Code hooks (default when no harness flag is set)")
	cmd.Flags().BoolVar(&cursorFlag, "cursor", false, "install Cursor hooks")
	cmd.Flags().BoolVar(&opencodeFlag, "opencode", false, "install opencode hooks")

	return cmd
}

// localModeTip is added to a control-plane init error when the user is not
// authenticated, because local mode needs no login.
const localModeTip = `Tip: run "chainloop trace init --local" to configure it in local mode`

// withLocalModeTip adds localModeTip to an authentication error. The message
// keeps the text the CLI shows for that error everywhere else. Any other error
// is returned unchanged.
func withLocalModeTip(err error) error {
	if err == nil {
		return nil
	}

	msg, ok := action.AuthErrorMessage(err)
	if !ok {
		return err
	}

	return fmt.Errorf("%s\n%s", msg, localModeTip)
}

// traceDocsURL is the guide covering what this command set up and what can be
// done with it, which is more than belongs in a command's own output.
const traceDocsURL = "https://docs.chainloop.dev/ai-sessions"

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

// writeTraceInitLocalSummary reports what a local init set up. It names no
// organization, project or workflow, because local mode has none: it records
// sessions and writes their evidence to disk, with no control plane.
//
// sessionsDir is the directory the sessions are written to, as the user should
// read it.
func writeTraceInitLocalSummary(w io.Writer, sessionsDir string, harnesses []string) {
	fmt.Fprint(w, "\nCongratulations, your repository is initialized in local mode\n\n")
	fmt.Fprint(w, "  storage       local (no control plane)\n")
	fmt.Fprintf(w, "  sessions      written to %s on each push\n", sessionsDir)

	fmt.Fprintf(w, "  harnesses     %s\n", strings.Join(harnesses, ", "))
}

// writeTraceStorageSwitchedNote tells the user that this control-plane init
// cleared a previous local-mode policy. The switch changes how every push
// behaves, so it is stated plainly here rather than left to a debug log, and the
// user is reminded to commit it like any other repository policy change.
func writeTraceStorageSwitchedNote(w io.Writer, ymlName string) {
	fmt.Fprintf(w, "\nThis repository was in local mode. Trace now sends the sessions to the control\nplane. Commit %s so that every clone also uses the control plane.\n", ymlName)
}

// writeTraceLocalNextSteps says what is left after a local init. Committing
// .chainloop.yml shares the choice: traceStorage: LOCAL is a repository policy a
// clone inherits. A teammate does not run init: in a clone, the first traced
// session installs the git hooks. Unlike the control-plane path there is
// nothing to push to a backend; the evidence lands on disk when the session's
// commits are pushed.
func writeTraceLocalNextSteps(w io.Writer, repoRoot, workDir string, installed []trace.Provider) {
	files := make([]string, 0, len(installed)+1)
	add := func(absolute string) {
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

  1. Commit these files. Every clone then uses local mode:
       git add %s
  2. Start %s and write some code
  3. Commit and push. The pre-push hook writes the session evidence to disk

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

// fileSnapshot is a file init is about to write to, as it stood beforehand.
// Every write init makes to the repository merges into or replaces something
// that may already be there — a harness's hooks sit beside the rest of its
// configuration, and installing over hooks that are present changes nothing at
// all — so putting the file back is the only way to undo a run without taking a
// working setup with it.
type fileSnapshot struct {
	path string
	// content is what the file held, and existed distinguishes an empty file
	// from one init created.
	content []byte
	mode    os.FileMode
	existed bool
	// existingAncestor is the closest directory that was already there, so
	// restoring removes the directories init created and no others.
	existingAncestor string
}

// snapshotFile records a file before it is written to.
func snapshotFile(path string) (fileSnapshot, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		ancestor, err := closestExistingDir(filepath.Dir(path))
		if err != nil {
			return fileSnapshot{}, err
		}

		return fileSnapshot{path: path, existingAncestor: ancestor}, nil
	}

	if err != nil {
		return fileSnapshot{}, err
	}

	// The mode is kept so restoring does not quietly widen or narrow it.
	info, err := os.Stat(path)
	if err != nil {
		return fileSnapshot{}, err
	}

	return fileSnapshot{path: path, content: content, mode: info.Mode().Perm(), existed: true}, nil
}

// closestExistingDir walks up from dir to the first directory that is there,
// which is the point below which anything is init's to remove again.
func closestExistingDir(dir string) (string, error) {
	for {
		info, err := os.Stat(dir)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("%s is not a directory", dir)
			}

			return dir, nil
		}

		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// The root, which always exists, so this is unreachable in practice.
			return dir, nil
		}

		dir = parent
	}
}

// restore puts the file back as it was, removing it when init was what created
// it, along with the directories that came with it. A directory holding
// anything else stays, which os.Remove sees to by refusing to remove it.
func (f fileSnapshot) restore() error {
	if f.existed {
		return os.WriteFile(f.path, f.content, f.mode)
	}

	if err := os.Remove(f.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	for dir := filepath.Dir(f.path); dir != f.existingAncestor; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			// Not empty, or already gone: either way there is nothing more of
			// init's to take back above it.
			return nil
		}
	}

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

	// Read after cfg.save, so a tracePath written on this run applies.
	store := state.NewRepoStore(gitDir, repoRoot)
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

	// local records that this is a local-mode init: it writes traceStorage:
	// LOCAL and reaches no control plane. tracePath is the optional override for
	// where local sessions are stored.
	local     bool
	tracePath string
	// traceStorage is the value written when saveTraceStorage is set. Local init
	// writes LOCAL. A control-plane init writes CONTROL_PLANE only to clear a
	// stale LOCAL left by an earlier local init.
	traceStorage string

	// The save* fields mark the values the user set on this run. Only those
	// are written back to .chainloop.yml, leaving the rest of the file alone.
	saveProject, saveWorkflow, saveOrganization, saveRequireTrace bool
	saveTraceStorage, saveTracePath                               bool
}

// traceStorageToPersist decides which traceStorage value trace init writes and
// whether it writes one at all. Local init always records LOCAL. A control-plane
// init writes CONTROL_PLANE only to clear a stale LOCAL an earlier local init
// left behind, so the pre-push hook never keeps storing on disk after a
// control-plane re-init. A fresh control-plane repository writes nothing and
// keeps its config minimal, relying on the CONTROL_PLANE default.
func traceStorageToPersist(localFlag bool, existingMode string) (value string, save bool) {
	switch {
	case localFlag:
		return config.TraceStorageLocal, true
	case existingMode == config.TraceStorageLocal:
		return config.TraceStorageControlPlane, true
	default:
		return "", false
	}
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
		cfg.project = repositoryconfig.LoadProjectFromYML(repoRoot)
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
	// Storage mode first: in local mode it is the field that makes the push
	// hook store on disk, so it must land even if nothing else is written.
	if c.saveTraceStorage {
		if err := config.SaveTraceStorageToYML(repoRoot, c.traceStorage); err != nil {
			return err
		}
		logger.Debug().Str("traceStorage", c.traceStorage).Msg("traceStorage saved to .chainloop.yml")
	}

	if c.saveTracePath {
		if err := config.SaveTracePathToYML(repoRoot, c.tracePath); err != nil {
			return err
		}
		logger.Debug().Str("tracePath", c.tracePath).Msg("tracePath saved to .chainloop.yml")
	}

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
		// Written as the current traceRequired field; the deprecated
		// requireTrace is only read back for an older config.
		if err := config.SaveTraceRequiredToYML(repoRoot, c.requireTrace); err != nil {
			return err
		}
		logger.Debug().Bool("traceRequired", c.requireTrace).Msg("traceRequired saved to .chainloop.yml")
	}

	return nil
}
