package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
)

// SubprocessExitError carries the exit code of a wrapped command so the
// CLI can propagate it to the parent shell.
type SubprocessExitError struct {
	Command  string
	ExitCode int
}

func (e *SubprocessExitError) Error() string {
	return fmt.Sprintf("%s exited with status %d", e.Command, e.ExitCode)
}

// TraceRunOpts configures a single TraceRun invocation.
type TraceRunOpts struct {
	// Store owns the chainloop-trace state directory, parented by the
	// .git directory inside a repository and otherwise by the out-of-tree
	// directory returned by state.NonGitDir. Outside a repository (IsGit
	// false) the managed git hooks are disabled, since git could never
	// invoke them anyway.
	Store *state.Store
	// RepoRoot is the repository root containing .chainloop.yml, or the
	// working directory when running outside a git repository.
	RepoRoot string
	// Providers is the list of agent provider names (e.g. "claude-code",
	// "cursor") whose info-gathering hooks should be installed.
	Providers []string
	// Command is the wrapped command: Command[0] is the program,
	// Command[1:] are its arguments.
	Command []string
	// ProjectName, Organization, WorkflowName identify the attestation
	// for this run. trace run is isolated from .chainloop.yml, so callers
	// must pass these values directly from CLI flags.
	ProjectName  string
	Organization string
	WorkflowName string
	// ProjectVersion, when set, targets a specific project version.
	// Empty means use the latest version.
	ProjectVersion string

	// ActionOpts is the root command's initialized options, used to build
	// the attestation executor. Required.
	ActionOpts *ActionsOpts
	// CLIVersion is the bare CLI version recorded in the attestation
	// predicate.
	CLIVersion string
}

// TraceRun wraps a single-shot agent invocation: it cleans any prior
// trace state, installs the trace-run subset of git and agent hooks,
// runs the wrapped command, then attests the session and tears
// everything down. Errors from the wrapped command are surfaced as
// SubprocessExitError so the caller can propagate the exit code.
func TraceRun(ctx context.Context, log zerolog.Logger, opts TraceRunOpts) error {
	if len(opts.Command) == 0 {
		return fmt.Errorf("no command to run")
	}

	selectedProviders := providers.ByNames(opts.Providers)
	if len(selectedProviders) == 0 {
		return fmt.Errorf("no trace providers selected")
	}

	var authExecOpts []ExecutorOption
	if opts.Organization != "" {
		authExecOpts = append(authExecOpts, WithForcedOrganization(opts.Organization))
	}
	executor, err := NewAttestationExecutor(opts.ActionOpts, opts.CLIVersion, authExecOpts...)
	if err != nil {
		return err
	}
	if err := executor.CheckAuth(ctx); err != nil {
		log.Warn().Err(err).Msg("authentication check failed; attestation will fail after the session")
	}
	if err := executor.Close(); err != nil {
		log.Debug().Err(err).Msg("closing auth-check executor")
	}

	// Snapshot the agent settings files before we touch anything else
	// so we can restore them bit-for-bit on exit, even if the user had
	// pre-existing customisations.
	backups := snapshotAgentSettings(selectedProviders, opts.RepoRoot, log)

	// Cleanup must precede restoreAgentSettings: it strips every chainloop hook
	// entry from the agent settings files, undoing the restore otherwise. Its
	// errors are logged inside CleanupTrace and ignored here, because failing
	// trace run on partial cleanup is worse than surfacing the warnings.
	// Outside a repository the state directory is trace run's alone —
	// `chainloop trace init` only ever targets a .git directory — so
	// leftovers from a killed run are always ours to reclaim.
	ownsState := !opts.Store.IsGit() || traceRunOwnsState(opts.Store)
	if ownsState {
		_ = CleanupTrace(opts.Store, opts.RepoRoot, log)
	}
	defer func() {
		if ownsState {
			_ = CleanupTrace(opts.Store, opts.RepoRoot, log)
		} else if err := opts.Store.ClearTraceRunActive(); err != nil {
			log.Warn().Err(err).Msg("could not clear trace-run sentinel")
		}
		restoreAgentSettings(backups, log)
	}()

	if err := opts.Store.InitTraceDir(); err != nil {
		return fmt.Errorf("create trace directory: %w", err)
	}

	// Outside a git repository there is nothing to invoke the git hooks.
	if opts.Store.IsGit() {
		hooksDir, err := hooks.Install(opts.Store.GitDir(), true)
		if err != nil {
			return err
		}
		log.Debug().Str("path", hooksDir).Msg("git hooks installed (commit-msg, post-commit)")
	}

	for _, p := range selectedProviders {
		if err := p.InstallHooksForTraceRun(opts.RepoRoot); err != nil {
			log.Warn().Err(err).Str("provider", p.Name()).Msg("could not install agent hooks")
			continue
		}
		log.Debug().Str("provider", p.Name()).Msg("agent hooks installed")
	}

	if err := opts.Store.MarkTraceInitialized(); err != nil {
		return fmt.Errorf("mark trace initialized: %w", err)
	}

	if err := opts.Store.MarkTraceRunActive(); err != nil {
		return fmt.Errorf("mark trace run active: %w", err)
	}

	// The wrapped command is supplied by the user on their own machine
	// (`chainloop trace run -- <cmd>`); running it verbatim is the feature and
	// crosses no trust boundary.
	//nolint:gosec // see above: running the user-supplied command verbatim is the feature
	sub := exec.CommandContext(ctx, opts.Command[0], opts.Command[1:]...) // nosemgrep
	sub.Stdin = os.Stdin
	sub.Stdout = os.Stdout
	sub.Stderr = os.Stderr

	log.Debug().Strs("command", opts.Command).Msg("running wrapped command")

	if err := sub.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return &SubprocessExitError{Command: opts.Command[0], ExitCode: exitErr.ExitCode()}
		}

		return fmt.Errorf("run %s: %w", opts.Command[0], err)
	}

	log.Debug().Msg("wrapped command completed; attesting session")

	return RunTracePush(ctx, log, RunTracePushOpts{
		AllowEmpty:     true,
		ProjectName:    opts.ProjectName,
		Organization:   opts.Organization,
		WorkflowName:   opts.WorkflowName,
		ProjectVersion: opts.ProjectVersion,
		IgnoreYAML:     true,
		ActionOpts:     opts.ActionOpts,
		CLIVersion:     opts.CLIVersion,
	})
}

// traceRunOwnsState reports whether the store's trace state is trace run's
// to wipe. A pre-existing install (from trace init or an agent hook's
// auto-install) owns the git hooks and the trace state dir, which holds
// unpushed AI attribution, so trace run must leave it intact. Leftover state
// from a dead trace run still carries the run-active sentinel and is ours.
func traceRunOwnsState(store *state.Store) bool {
	return !store.IsTraceInitialized() || store.IsTraceRunActive()
}

// settingsBackup captures a provider's settings file before trace run
// mutates it, so the file can be restored verbatim on exit.
type settingsBackup struct {
	path       string
	content    []byte
	mode       os.FileMode
	existed    bool
	dirExisted bool
}

// snapshotAgentSettings reads each selected provider's settings file
// into memory. A missing file is recorded so the restore step can
// remove anything install created. Read failures are warned and the
// provider is dropped from the backup list (best-effort).
func snapshotAgentSettings(provs []trace.Provider, repoRoot string, log zerolog.Logger) []settingsBackup {
	backups := make([]settingsBackup, 0, len(provs))
	for _, p := range provs {
		b := settingsBackup{path: p.SettingsFile(repoRoot)}
		b.dirExisted = dirExists(filepath.Dir(b.path))
		info, err := os.Stat(b.path)
		switch {
		case err == nil:
			content, rerr := os.ReadFile(b.path)
			if rerr != nil {
				log.Warn().Err(rerr).Str("path", b.path).Msg("could not back up agent settings; install will proceed without restore")
				continue
			}
			b.existed = true
			b.content = content
			b.mode = info.Mode().Perm()
		case errors.Is(err, os.ErrNotExist):
			// File didn't exist — restore will delete whatever install creates.
		default:
			log.Warn().Err(err).Str("path", b.path).Msg("could not stat agent settings; install will proceed without restore")
			continue
		}
		backups = append(backups, b)
	}

	return backups
}

// restoreAgentSettings writes each backed-up settings file back to its
// original path (with original mode), or removes the file if it didn't
// exist before snapshotting. If trace run created the parent directory
// (it did not exist at snapshot time), the directory is also removed
// once empty so trace run leaves no on-disk trace.
func restoreAgentSettings(backups []settingsBackup, log zerolog.Logger) {
	for _, b := range backups {
		if b.existed {
			if err := os.WriteFile(b.path, b.content, b.mode); err != nil {
				log.Warn().Err(err).Str("path", b.path).Msg("could not restore agent settings backup")
			}
			continue
		}
		if err := os.Remove(b.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Warn().Err(err).Str("path", b.path).Msg("could not remove agent settings file created by trace run")
			continue
		}
		if b.dirExisted {
			continue
		}
		// Parent directory didn't exist before the run — install created
		// it. Remove it now; os.Remove fails with ENOTEMPTY when other
		// content has appeared, which is the no-op we want there.
		_ = os.Remove(filepath.Dir(b.path))
	}
}

// dirExists reports whether path resolves to an existing directory.
// Returns false on any error (missing, permission denied, not-a-dir).
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// CleanupTrace removes the store's chainloop-trace state, the managed git
// hooks, and every known provider's agent hooks. A store outside a repository
// had no git hooks installed in the first place, and the state directory itself
// is trace run's to remove. Every step runs even when an earlier one failed so
// partial cleanup still progresses; the joined error is returned for callers
// that need to fail loudly (e.g. trace uninstall). Callers that want
// best-effort cleanup (e.g. trace run's defer) can ignore the result.
func CleanupTrace(store *state.Store, repoRoot string, log zerolog.Logger) error {
	var errs []error

	if store.IsGit() {
		if hooksDir, err := hooks.Uninstall(store.GitDir()); err != nil {
			log.Warn().Err(err).Msg("could not remove git hooks")
			errs = append(errs, fmt.Errorf("remove git hooks: %w", err))
		} else {
			log.Debug().Str("path", hooksDir).Msg("git hooks removed")
		}
	}

	if err := store.RemoveTraceDir(); err != nil {
		log.Warn().Err(err).Msg("could not remove trace directory")
		errs = append(errs, fmt.Errorf("remove trace directory: %w", err))
	}

	// The out-of-tree state directory is ours alone, so drop it too rather
	// than leaving one empty directory per working directory behind.
	// os.Remove fails with ENOTEMPTY when anything survived, which is the
	// no-op we want there.
	if !store.IsGit() {
		_ = os.Remove(store.Dir())
	}

	for _, p := range providers.All() {
		if err := p.UninstallHooks(repoRoot); err != nil {
			log.Warn().Err(err).Str("provider", p.Name()).Msg("could not remove agent hooks")
			errs = append(errs, fmt.Errorf("uninstall %s hooks: %w", p.Name(), err))
			continue
		}
		log.Debug().Str("provider", p.Name()).Msg("agent hooks removed")
	}

	return errors.Join(errs...)
}
