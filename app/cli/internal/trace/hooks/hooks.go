package hooks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// HookMarker is the comment line embedded in managed hook scripts.
	HookMarker       = "# chainloop-trace-managed"
	hookBackupSuffix = ".chainloop-backup"
)

// managedHooks defines every git hook that Install creates.
// Order matters: hooks are installed in this order.
var managedHooks = []hookDef{
	{"commit-msg", true},
	{"post-commit", false},
	{"pre-push", false},
}

type hookDef struct {
	// name is both the git hook filename and the "chainloop trace hook git <name>" subcommand.
	name     string
	passArgs bool
}

// selectedHooks returns managedHooks with pre-push filtered out when
// skipPrePush is true.
func selectedHooks(skipPrePush bool) []hookDef {
	if !skipPrePush {
		return managedHooks
	}
	out := make([]hookDef, 0, len(managedHooks))
	for _, h := range managedHooks {
		if h.name == "pre-push" {
			continue
		}
		out = append(out, h)
	}

	return out
}

// IsInstalled reports whether the managed hooks are present and carry
// the chainloop marker. Pass skipPrePush=true to exclude pre-push from
// the check, matching what trace run installs.
//
// gitDir may point at a per-worktree gitdir; the common hooks dir is
// resolved internally (see resolveHooksDir).
func IsInstalled(gitDir string, skipPrePush bool) bool {
	hooksDir, err := resolveHooksDir(gitDir)
	if err != nil {
		return false
	}

	for _, h := range selectedHooks(skipPrePush) {
		content, err := os.ReadFile(filepath.Join(hooksDir, h.name))
		if err != nil || !strings.Contains(string(content), HookMarker) {
			return false
		}
	}

	return true
}

// hookContent generates a hook script for the given command.
// When passArgs is true, "$@" is appended so the hook receives its positional arguments
// (e.g. commit-msg receives the message file path as $1).
func hookContent(hookCmd string, passArgs bool) string {
	args := ""
	if passArgs {
		args = ` "$@"`
	}

	// The trailing exit 0 keeps tracing from ever blocking a commit or push:
	// without it a missing chainloop binary makes sh exit 127 and git aborts.
	return fmt.Sprintf("#!/bin/sh\n%s\nchainloop trace hook git %s%s\nexit 0\n", HookMarker, hookCmd, args)
}

// hookContentWithChain generates a hook script that chains to a backup.
func hookContentWithChain(hookCmd, backupPath string, passArgs bool) string {
	args := ""
	if passArgs {
		args = ` "$@"`
	}

	// exec makes the chained hook's status the hook's status. The trailing
	// exit 0 covers a backup that lost its executable bit, where the && list
	// would otherwise fail the hook (see hookContent).
	return fmt.Sprintf("#!/bin/sh\n%s\nchainloop trace hook git %s%s\n[ -x \"%s\" ] && exec \"%s\" \"$@\"\nexit 0\n",
		HookMarker, hookCmd, args, backupPath, backupPath)
}

// Install installs git hooks for trace automation. gitDir may be either
// the main repo's .git or a per-worktree gitdir (.git/worktrees/<name>);
// the resolved hooks directory is returned so callers can log it.
//
// Hooks are always written to the *common* hooks dir, since git only reads
// hooks/ from the shared .git/ even when invoked from a linked worktree.
//
// Pass skipPrePush=true to install only commit-msg and post-commit; used
// by trace run, which drives the attestation push itself.
func Install(gitDir string, skipPrePush bool) (string, error) {
	hooksDir, err := resolveHooksDir(gitDir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return "", fmt.Errorf("create hooks directory: %w", err)
	}

	for _, h := range selectedHooks(skipPrePush) {
		hookPath := filepath.Join(hooksDir, h.name)
		backupPath := hookPath + hookBackupSuffix

		if err := installSingleHook(hookPath, backupPath, h.name, h.passArgs); err != nil {
			return "", fmt.Errorf("install %s hook: %w", h.name, err)
		}
	}

	return hooksDir, nil
}

func installSingleHook(hookPath, backupPath, hookCmd string, passArgs bool) error {
	existing, err := os.ReadFile(hookPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read existing hook: %w", err)
	}

	// Lstat: a dangling symlink still occupies backupPath and must not be
	// renamed over.
	_, statErr := os.Lstat(backupPath)
	if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("check hook backup: %w", statErr)
	}
	backupExists := statErr == nil

	if err == nil && !strings.Contains(string(existing), HookMarker) {
		// Foreign hook, back it up and chain. An existing backup is the user's
		// original hook, saved by an install whose uninstall never restored it,
		// so it must never be overwritten.
		if backupExists {
			return fmt.Errorf("backup %s already exists; restore or remove it before installing", backupPath)
		}
		if err := os.Rename(hookPath, backupPath); err != nil {
			return fmt.Errorf("backup existing hook: %w", err)
		}
		backupExists = true
	}

	// Preserve chaining if a backup exists (idempotent reinstall)
	if backupExists {
		//nolint:gosec // git only runs hooks that are executable, so 0600 would silently disable them
		return os.WriteFile(hookPath, []byte(hookContentWithChain(hookCmd, backupPath, passArgs)), 0755)
	}

	//nolint:gosec // git only runs hooks that are executable, so 0600 would silently disable them
	return os.WriteFile(hookPath, []byte(hookContent(hookCmd, passArgs)), 0755)
}

// Uninstall removes git hooks installed by trace. gitDir semantics match
// Install.
func Uninstall(gitDir string) (string, error) {
	hooksDir, err := resolveHooksDir(gitDir)
	if err != nil {
		return "", err
	}

	for _, h := range managedHooks {
		hookPath := filepath.Join(hooksDir, h.name)
		backupPath := hookPath + hookBackupSuffix

		if err := uninstallSingleHook(hookPath, backupPath); err != nil {
			return "", fmt.Errorf("uninstall %s hook: %w", h.name, err)
		}
	}

	return hooksDir, nil
}

// resolveHooksDir returns the hooks directory git will actually read from,
// given a (possibly worktree-private) gitDir. In a linked worktree, gitDir
// is .git/worktrees/<name>, but git classifies hooks/ as a common path:
// hooks placed under the per-worktree dir are silently ignored. The
// commondir pointer file inside the worktree-private dir tells us where
// the shared .git lives.
func resolveHooksDir(gitDir string) (string, error) {
	commonDir, err := resolveCommonDir(gitDir)
	if err != nil {
		return "", err
	}

	return filepath.Join(commonDir, "hooks"), nil
}

// resolveCommonDir reads the commondir pointer file that git writes inside
// linked worktrees and returns the resolved common gitdir. If the file is
// absent (main checkout), gitDir is returned unchanged.
func resolveCommonDir(gitDir string) (string, error) {
	commonDirFile := filepath.Join(gitDir, "commondir")
	data, err := os.ReadFile(commonDirFile)
	if errors.Is(err, os.ErrNotExist) {
		return gitDir, nil
	}
	if err != nil {
		return "", fmt.Errorf("read commondir file: %w", err)
	}

	commonDir := strings.TrimSpace(string(data))
	if commonDir == "" {
		return "", fmt.Errorf("commondir file is empty: %s", commonDirFile)
	}

	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(gitDir, commonDir)
	}

	commonDir = filepath.Clean(commonDir)
	if _, err := os.Stat(commonDir); err != nil {
		return "", fmt.Errorf("commondir path does not exist: %w", err)
	}

	return commonDir, nil
}

func uninstallSingleHook(hookPath, backupPath string) error {
	content, err := os.ReadFile(hookPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read hook: %w", err)
	}

	if !strings.Contains(string(content), HookMarker) {
		// Not our hook, leave it alone
		return nil
	}

	if err := os.Remove(hookPath); err != nil {
		return err
	}

	// Restore backup if it exists
	if err := os.Rename(backupPath, hookPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}
