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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newTraceInitCmd creates the trace init subcommand.
func newTraceInitCmd() *cobra.Command {
	var (
		project       string
		contract      string
		onlineFlag    bool
		offlineFlag   bool
		tracePathFlag string
		claudeFlag    bool
		cursorFlag    bool
		opencodeFlag  bool
	)

	cmd := &cobra.Command{
		Use: "init", //nolint:goconst // cobra verb, kept literal like the other init subcommands
		// skipActionOptsInit keeps root from building the control-plane
		// connection up front. Control-plane mode builds it lazily with
		// ensureActionOpts below; the offline mode alone never does, so it
		// reaches no control plane at all (R-008).
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

The trace modes select where a session's evidence goes. --online (the default)
attests it to the Chainloop control plane. --offline stores it on disk, under
.git/chainloop-trace/local-sessions or the --trace-path directory, and needs no
login and no control plane. Pass both to do both. With no mode flag, init keeps
the modes in .chainloop.yml, or uses --online on a first run. The modes are
saved to .chainloop.yml as traceModes, so every clone records the same way.

With --offline alone, init installs the same hooks but creates no workflow,
needs no login, and reaches no control plane. --contract, --workflow, --org and
--project are control-plane settings and need --online; --require-trace does
not.

Run init once per repository. In a clone of an initialized repository, the
first traced session installs the git hooks.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			gitDir, repoRoot, err := tracegit.FindGitDirAndRoot()
			if err != nil {
				return err
			}

			// An explicit mode flag overrides the modes in .chainloop.yml, also
			// an unreadable value. With no mode flag, an unreadable value is an
			// error before anything is installed: every pre-push would fail on
			// it, so init must not report success over it.
			existingModes, existingErr := config.LoadTraceModesFromYML(repoRoot)
			modes, saveModes, err := resolveTraceModes(traceModeFlags{
				online:     onlineFlag,
				onlineSet:  cmd.Flags().Changed("online"),
				offline:    offlineFlag,
				offlineSet: cmd.Flags().Changed("offline"),
			}, existingModes, existingErr)
			if err != nil {
				return err
			}

			// The offline mode alone contacts no control plane, so the
			// control-plane identity and workflow flags are contradictions
			// rather than extra configuration. --require-trace is orthogonal and
			// allowed.
			if !modes.Online {
				for _, name := range []string{"contract", "workflow", "org", "project"} { //nolint:goconst // flag names, kept literal like the other flag references
					if cmd.Flags().Changed(name) {
						return fmt.Errorf("--%s cannot be used without --online: the offline mode contacts no control plane and creates no workflow", name)
					}
				}
			}

			cfg, err := resolveTraceInitConfig(cmd, repoRoot, project)
			if err != nil {
				return err
			}
			cfg.modes = modes
			cfg.saveModes = saveModes
			cfg.tracePath = tracePathFlag
			cfg.saveTracePath = tracePathFlag != ""

			// The online mode fills in the identity and opens an executor,
			// building the connection lazily (the command is
			// skipActionOptsInit). The offline mode alone resolves no identity
			// and opens no executor.
			var executor *action.AttestationExecutor
			if modes.Online {
				if err := ensureActionOpts(cmd); err != nil {
					return withOfflineHint(err)
				}

				executor, err = resolveTraceIdentity(cmd.Context(), cfg, repoRoot)
				if err != nil {
					return withOfflineHint(stopIfAborted(err))
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

			// The offline mode has no control plane to create the workflow in.
			if modes.Online {
				if err := ensureTraceInitWorkflow(cmd.Context(), executor, cfg, contract); err != nil {
					return withOfflineHint(err)
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
			// Read before writeTraceInitState marks the repository initialized:
			// a change of modes is news only to a repository that was set up.
			wasInitialized := state.NewGitStore(gitDir).IsTraceInitialized()

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
			summary := traceInitSummary{modes: modes, harnesses: providerNames(installed)}
			if modes.Online {
				// Nothing pinned an organization when cfg has none, so the
				// workflow went to the one the CLI points at. That is the one to
				// report: it is what the connection used, not a guess.
				summary.organization = cmp.Or(cfg.organization, viper.GetString(confOptions.organization.viperKey))
				summary.project = cfg.project
				summary.workflow = cfg.workflow
			}
			if modes.Offline {
				summary.sessionsPath = traceSessionsPathForDisplay(repoRoot, gitDir, workDir)
			}

			writeTraceInitSummary(os.Stdout, summary)

			// A change of modes in a repository that was already set up changes
			// how every push behaves, so say it rather than leave it to a debug
			// log the user never sees.
			if saveModes && existingErr == nil && (wasInitialized || existingModes != (config.TraceModes{Online: true})) {
				writeTraceModesChangedNote(os.Stdout, existingModes, modes, config.ChainloopYMLName(repoRoot))
			}

			writeTraceNextSteps(os.Stdout, modes, repoRoot, workDir, installed)

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "chainloop project name")
	cmd.Flags().StringVar(&contract, "contract", "", traceContractFlagDesc)
	cmd.Flags().BoolVar(&onlineFlag, "online", false, "attest sessions to the Chainloop control plane (the default when no mode flag is set and .chainloop.yml has no traceModes)")
	cmd.Flags().BoolVar(&offlineFlag, "offline", false, "store sessions on disk, with no control plane, login or workflow; combine with --online to do both")
	cmd.Flags().StringVar(&tracePathFlag, "trace-path", "", "where the offline mode stores sessions (default: .git/chainloop-trace/local-sessions)")
	cmd.Flags().String("workflow", "", "chainloop workflow name used for trace attestations (defaults to \"ai-coding-session\")")
	cmd.Flags().Bool("require-trace", false, "block pushes when a required trace is missing")
	cmd.Flags().BoolVar(&claudeFlag, "claude", false, "install Claude Code hooks (default when no harness flag is set)")
	cmd.Flags().BoolVar(&cursorFlag, "cursor", false, "install Cursor hooks")
	cmd.Flags().BoolVar(&opencodeFlag, "opencode", false, "install opencode hooks")

	return cmd
}

// traceDocsURL is the guide covering what this command set up and what can be
// done with it, which is more than belongs in a command's own output.
const traceDocsURL = "https://docs.chainloop.dev/ai-sessions"

// traceInitSummary is what init reports it set up. The online fields are empty
// for the offline mode alone, and sessionsPath is empty without the offline
// mode.
type traceInitSummary struct {
	modes                           config.TraceModes
	organization, project, workflow string
	sessionsPath                    string
	harnesses                       []string
}

// writeTraceInitSummary reports what the repository was set up with. It goes to
// stdout rather than through the logger because it is the command's result,
// not a note about something that happened on the way there.
func writeTraceInitSummary(w io.Writer, s traceInitSummary) {
	fmt.Fprint(w, "\nCongratulations, your repository is initialized\n\n")

	if s.modes.Online {
		fmt.Fprintf(w, "  modes         %s\n", s.modes)

		// Empty only when the CLI has no organization configured either,
		// which leaves nothing truthful to name.
		if s.organization != "" {
			fmt.Fprintf(w, "  organization  %s\n", s.organization)
		}

		fmt.Fprintf(w, "  project       %s\n", s.project)
		fmt.Fprintf(w, "  workflow      %s\n", s.workflow)
	} else {
		fmt.Fprintf(w, "  modes         %s (no control plane)\n", s.modes)
	}

	if s.modes.Offline {
		fmt.Fprintf(w, "  sessions      stored on disk under %s on push\n", s.sessionsPath)
	}

	fmt.Fprintf(w, "  harnesses     %s\n", strings.Join(s.harnesses, ", "))

	// The offline mode alone leaves the control plane out. Say how to bring
	// it in, so the choice does not look final.
	if !s.modes.Online {
		fmt.Fprint(w, "\nTo also send sessions to a Chainloop control plane later, run:\n       chainloop trace init --online --offline\n")
	}
}

// writeTraceModesChangedNote tells the user that this init changed the trace
// modes of a repository that was already set up. The change affects how every
// push behaves, so it is stated plainly here rather than left to a debug log,
// and the user is reminded to commit it like any other repository policy
// change.
func writeTraceModesChangedNote(w io.Writer, from, to config.TraceModes, ymlName string) {
	fmt.Fprintf(w, "\nTrace modes changed from %s to %s. Commit %s so every clone\nrecords the same way.\n", from, to, ymlName)
}

// traceSessionsPathForDisplay names where the offline mode stores sessions,
// relative to where the user ran init when that is shorter to read.
func traceSessionsPathForDisplay(repoRoot, gitDir, workDir string) string {
	path := action.ResolveTracePath(repoRoot, state.NewGitStore(gitDir), config.LoadTracePathFromYML(repoRoot))
	if rel, err := filepath.Rel(workDir, path); err == nil {
		return rel
	}

	return path
}

// writeTraceNextSteps says what is left for the user to do. Committing comes
// first: what init wrote into the repository is shared, and until it is
// committed this setup exists in one working copy only.
//
// With the online mode it stops short of promising that a teammate who pulls
// is set up: a teammate needs their own login. The offline mode needs nothing
// of the kind, and the first traced session in a clone installs the git hooks,
// so a teammate does not run init at all.
//
// workDir is where the user ran the command, which is what `git add` resolves
// its arguments against; it is not always the repository root.
func writeTraceNextSteps(w io.Writer, modes config.TraceModes, repoRoot, workDir string, installed []trace.Provider) {
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

	commit := "Commit these files. Teammates then only need to run chainloop trace init:"
	if !modes.Online {
		commit = "Commit these files so every clone records the same way:"
	}

	var push string
	switch {
	case modes.Online && modes.Offline:
		push = `The AI coding sessions behind those commits are
     recorded in Chainloop and written to disk automatically; the push tells
     you where`
	case modes.Offline:
		push = `The session evidence is written to disk, with no
     control plane; the push tells you where`
	default:
		push = `The AI coding sessions behind those commits are
     recorded and stored automatically`
	}

	fmt.Fprintf(w, `
What's next

  1. %s
       git add %s
  2. Start %s and write some code
  3. Commit and push as usual. %s

Learn more: %s
`, commit, strings.Join(files, " "), joinWithOr(providerNames(installed)), push, traceDocsURL)
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

	// modes are the trace modes of the repository after this run, written
	// when saveModes is set. tracePath is the optional override for where the
	// offline mode stores sessions.
	modes     config.TraceModes
	tracePath string

	// The save* fields mark the values the user set on this run. Only those
	// are written back to .chainloop.yml, leaving the rest of the file alone.
	saveProject, saveWorkflow, saveOrganization, saveRequireTrace bool
	saveModes, saveTracePath                                      bool
}

// traceModeFlags are the --online and --offline flags, and whether the user
// passed each one.
type traceModeFlags struct {
	online, onlineSet   bool
	offline, offlineSet bool
}

// resolveTraceModes decides the trace modes init sets up and whether it writes
// them to .chainloop.yml. A mode flag sets the modes outright: the passed flags
// are the modes. With no mode flag, init keeps the existing modes, which are
// online only when .chainloop.yml has none. The modes are written only when
// they change, so a fresh online repository keeps its config minimal.
// existingErr is the error from reading the existing modes: a mode flag
// overrides the unreadable value, and with no mode flag it is returned.
func resolveTraceModes(flags traceModeFlags, existing config.TraceModes, existingErr error) (config.TraceModes, bool, error) {
	if !flags.onlineSet && !flags.offlineSet {
		if existingErr != nil {
			return config.TraceModes{}, false, fmt.Errorf("existing .chainloop.yml: %w", existingErr)
		}

		return existing, false, nil
	}

	modes := config.TraceModes{Online: flags.online, Offline: flags.offline}
	if !modes.Online && !modes.Offline {
		return config.TraceModes{}, false, errors.New("set at least one trace mode: --online, --offline, or both")
	}

	return modes, existingErr != nil || modes != existing, nil
}

// offlineHint is added to an online init error when the control plane cannot
// be used, because the offline mode needs no control plane.
const offlineHint = `To trace without a Chainloop control plane, run "chainloop trace init --offline"`

// withOfflineHint adds offlineHint to an error that tells the control plane is
// not available: the credentials are missing or rejected, or the control plane
// cannot be reached. Any other error is returned unchanged.
func withOfflineHint(err error) error {
	if err == nil {
		return nil
	}

	if msg, ok := action.AuthErrorMessage(err); ok {
		return fmt.Errorf("%s\n%s", msg, offlineHint)
	}

	if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
		return fmt.Errorf("%w\n%s", err, offlineHint)
	}

	return err
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
	// Trace modes first: the offline mode is what makes the push hook store
	// on disk, so it must land even if nothing else is written.
	if c.saveModes {
		if err := config.SaveTraceModesToYML(repoRoot, c.modes); err != nil {
			return err
		}
		logger.Debug().Str("traceModes", c.modes.String()).Msg("traceModes saved to .chainloop.yml")
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
