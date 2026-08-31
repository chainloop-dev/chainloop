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

package action

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
)

// HandleAgentSessionEnd handles the agent session-end hook.
func HandleAgentSessionEnd(provider trace.Provider, log zerolog.Logger) error {
	input, err := provider.ReadHookInput(os.Stdin)
	if err != nil || !state.ValidSessionID(input.SessionID) {
		log.Debug().Err(err).Msg("session-end: no valid input")
		return nil
	}

	sessionID := input.SessionID
	log.Debug().Str("session_id", sessionID).Msg("session-end hook invoked")

	store, repoRoot, err := state.Locate()
	if err != nil {
		log.Debug().Err(err).Msg("session-end: no trace state located")
		return nil
	}

	if err := provider.CopySessionData(store, repoRoot, sessionID); err != nil {
		log.Debug().Err(err).Msg("copy session data failed")
	}

	log.Debug().Str("session_id", sessionID).Msg("session ended")

	return nil
}

// HandleAgentSessionStart handles the agent session-start hook.
func HandleAgentSessionStart(provider trace.Provider, log zerolog.Logger) error {
	input, err := provider.ReadHookInput(os.Stdin)
	if err != nil || !state.ValidSessionID(input.SessionID) {
		log.Debug().Err(err).Msg("session-start: no valid input")
		return nil
	}

	log.Debug().Str("session_id", input.SessionID).Msg("session-start hook invoked")

	store, repoRoot, err := state.Locate()
	if err != nil {
		log.Debug().Err(err).Msg("session-start: no trace state located")
		return nil
	}

	ensureSessionTracked(provider, store, repoRoot, input, log)

	if err := provider.SystemMessage("\n\n*** This session will be attested by Chainloop ***"); err != nil {
		log.Debug().Err(err).Msg("session-start: failed to send system message")
	}

	return nil
}

// HandleAgentPreToolUse handles the agent pre-tool-use hook.
func HandleAgentPreToolUse(provider trace.Provider, log zerolog.Logger) error {
	input, err := provider.ReadHookInput(os.Stdin)
	if err != nil || !state.ValidSessionID(input.SessionID) {
		log.Debug().Err(err).Msg("pre-tool-use: no valid input")
		return nil
	}

	log.Debug().Str("session_id", input.SessionID).Msg("pre-tool-use hook invoked")

	store, repoRoot, err := state.Locate()
	if err != nil {
		log.Debug().Err(err).Msg("pre-tool-use: no trace state located")
		return nil
	}

	ensureSessionTracked(provider, store, repoRoot, input, log)

	switch {
	case provider.IsCommandTool(input.ToolName):
		// Shell command: snapshot the whole worktree so the post hook can diff
		// it and attribute the command's file changes to the AI.
		captureWorktreeSnapshot(store, repoRoot, input.SessionID, log)
	case provider.IsFileWritingTool(input.ToolName):
		if input.FilePath == "" {
			log.Debug().Str("tool", input.ToolName).Msg("pre-tool-use: file-writing tool produced no file path, skipping")
			return nil
		}
		if err := provider.CaptureFileSnapshot(store, input); err != nil {
			log.Debug().Err(err).Str("file", input.FilePath).Msg("pre-tool-use: capture snapshot failed")
		}
	default:
		log.Debug().Str("tool", input.ToolName).Msg("pre-tool-use: not a tracked tool, skipping")
	}

	return nil
}

// captureWorktreeSnapshot records the working-tree signature before a shell
// command runs, so HandleAgentPostToolUse can diff it and attribute the files
// the command changed. Best-effort: failures are logged and never block the agent.
func captureWorktreeSnapshot(store *state.Store, repoRoot, sessionID string, log zerolog.Logger) {
	sig, err := tracegit.NewGoGitClient().SnapshotWorktree(repoRoot)
	if err != nil {
		log.Debug().Err(err).Msg("pre-command: worktree snapshot failed")
		return
	}

	if err := store.SaveShellPreSignature(sessionID, sig); err != nil {
		log.Debug().Err(err).Msg("pre-command: save worktree signature failed")
	}
}

// ensureSessionTracked auto-installs git hooks (unconditionally, on every
// invocation — itself idempotent via hooks.IsInstalled), then creates a
// session record if one doesn't already exist and copies the session data.
// Idempotent so any hook entry point can call it safely. Per-input metadata
// (e.g., AgentVersion) is captured on the first call only — subsequent
// hooks for the same session are no-ops, no upsert.
func ensureSessionTracked(provider trace.Provider, store *state.Store, repoRoot string, input *trace.HookInput, log zerolog.Logger) {
	// Run install before any early return so hooks recover from missing
	// state (deleted .git/hooks, project YAML appearing after session start).
	// Outside a repository there is nothing to invoke git hooks, so there is
	// nothing to install.
	if store.IsGit() {
		autoInstallGitHooks(store, repoRoot, log)
	}

	sessionID := input.SessionID
	if store.SessionRecordExists(sessionID) {
		return
	}

	log.Debug().Str("session_id", sessionID).Str("state_dir", store.Dir()).Str("provider", provider.Name()).Msg("tracking new session")

	rec := &state.SessionRecord{
		SessionID:    sessionID,
		Provider:     provider.Name(),
		AgentVersion: input.AgentVersion,
		Model:        input.Model,
		Active:       true,
		StartedAt:    state.NowTimestamp(),
	}
	if err := store.SaveSessionRecord(rec); err != nil {
		log.Debug().Err(err).Msg("save session record failed")
		return
	}

	if err := provider.CopySessionData(store, repoRoot, sessionID); err != nil {
		log.Debug().Err(err).Msg("copy session data failed")
	}
}

// HandleAgentPostToolUse handles post-edit hooks across providers
// (Claude's post-tool-use, Cursor's afterFileEdit) and records AI-attributed
// line ranges for the edited file.
func HandleAgentPostToolUse(provider trace.Provider, log zerolog.Logger) error {
	input, err := provider.ReadHookInput(os.Stdin)
	if err != nil || !state.ValidSessionID(input.SessionID) {
		log.Debug().Err(err).Msg("post-tool-use: no valid input")
		return nil
	}

	sessionID := input.SessionID

	isCommand := provider.IsCommandTool(input.ToolName)
	isFileWriting := provider.IsFileWritingTool(input.ToolName)
	if !isCommand && !isFileWriting {
		log.Debug().Str("tool", input.ToolName).Msg("post-tool-use: not a tracked tool, skipping")
		return nil
	}
	if isFileWriting && input.FilePath == "" {
		log.Debug().Str("tool", input.ToolName).Msg("post-tool-use: file-writing tool produced no file path, skipping")
		return nil
	}

	log.Debug().Str("session_id", sessionID).Str("tool", input.ToolName).Str("file", input.FilePath).Msg("post-tool-use hook invoked")

	store, repoRoot, err := state.Locate()
	if err != nil {
		log.Debug().Err(err).Msg("post-tool-use: no trace state located")
		return nil
	}

	// Guarantees session tracking + git-hook install when afterFileEdit is
	// the first hook we see (Cursor has no pre-tool-use).
	ensureSessionTracked(provider, store, repoRoot, input, log)

	if isCommand {
		// Shell command: diff the before/after worktree snapshots and attribute
		// every file the command changed to the AI.
		recordCommandLineRanges(store, repoRoot, sessionID, log)

		return nil
	}

	after, err := os.ReadFile(input.FilePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Debug().Err(err).Str("file", input.FilePath).Msg("post-tool-use: cannot read file after edit")
			provider.CleanupAfterEdit(store, input)
			return nil
		}
		// File was deleted — treat after as empty so attribution records
		// the deletion as AI-authored (ComputeLineRanges returns changed=true
		// with nil ranges for before≠nil, after=empty).
		after = nil
	}

	before := provider.ResolveBeforeContent(store, input, after)

	ranges, changed := attribution.ComputeLineRanges(before, after)

	// Normalize to repo-relative path for storage — git diffs use relative paths,
	// so ai-lines keys must match for commit correlation and attribution to work.
	relPath, err := filepath.Rel(repoRoot, input.FilePath)
	if err != nil {
		relPath = input.FilePath
	}

	if changed {
		if err := store.RecordLineRanges(sessionID, relPath, ranges); err != nil {
			log.Debug().Err(err).Msg("post-tool-use: record line ranges failed")
		} else {
			log.Debug().Int("ranges", len(ranges)).Str("file", relPath).Msg("line ranges recorded")
		}
	}

	provider.CleanupAfterEdit(store, input)

	return nil
}

// recordCommandLineRanges diffs the worktree signature captured before a shell
// command against the current worktree, and records every created/modified file
// (whole-file range) and every deleted file as AI-attributed. Enrich later caps
// the AI line count to each file's committed diff totals, so whole-file ranges
// yield correct counts. Best-effort: never blocks the agent.
func recordCommandLineRanges(store *state.Store, repoRoot, sessionID string, log zerolog.Logger) {
	before, err := store.LoadShellPreSignature(sessionID)
	if err != nil {
		// No pre-command snapshot (missed pre hook, parallel overwrite) — skip.
		log.Debug().Err(err).Msg("post-command: no pre-command worktree signature")
		return
	}
	defer store.DeleteShellPreSignature(sessionID)

	after, err := tracegit.NewGoGitClient().SnapshotWorktree(repoRoot)
	if err != nil {
		log.Debug().Err(err).Msg("post-command: worktree snapshot failed")
		return
	}

	changed, deleted := attribution.ChangedPaths(before, after)
	for _, relPath := range changed {
		content, err := os.ReadFile(filepath.Join(repoRoot, relPath))
		if err != nil {
			continue
		}
		// ChangedPaths already proved the file changed, so record it even when
		// the result is empty (created empty or truncated) — ComputeLineRanges
		// returns nil ranges for empty content, which marks the file AI-touched
		// with zero added lines, matching the deletion-only treatment.
		ranges, _ := attribution.ComputeLineRanges(nil, content)
		if err := store.RecordLineRanges(sessionID, relPath, ranges); err != nil {
			log.Debug().Err(err).Str("file", relPath).Msg("post-command: record line ranges failed")
		}
	}
	for _, relPath := range deleted {
		// nil ranges records the file as AI-touched (deletion-only), matching
		// the file-writing hook's treatment of deletions.
		if err := store.RecordLineRanges(sessionID, relPath, nil); err != nil {
			log.Debug().Err(err).Str("file", relPath).Msg("post-command: record deletion failed")
		}
	}

	log.Debug().Int("changed", len(changed)).Int("deleted", len(deleted)).Str("session_id", sessionID).Msg("command line ranges recorded")
}

// autoInstallGitHooks installs git hooks if they're not already present
// and a project name is discoverable. Under `chainloop trace run`,
// pre-push is omitted because trace run drives attestation itself; a
// pre-push installed here would double-attest on in-session `git push`.
//
// Trace-run mode is detected via the run-active sentinel file under
// .git/chainloop-trace/, which is robust to whatever environment each
// agent happens to propagate to its hook subprocesses.
// Only meaningful for a store inside a repository; GitDir is empty otherwise.
func autoInstallGitHooks(store *state.Store, repoRoot string, log zerolog.Logger) {
	gitDir := store.GitDir()
	skipPrePush := store.IsTraceRunActive()

	if hooks.IsInstalled(gitDir, skipPrePush) {
		return
	}

	if config.LoadProjectFromYML(repoRoot) == "" {
		return // can't auto-install without a project name
	}

	if _, err := hooks.Install(gitDir, skipPrePush); err != nil {
		// Warn, not debug: some failures (e.g. a leftover hook backup) keep
		// tracing off until the user intervenes, so they must be visible.
		log.Warn().Err(err).Msg("auto-install git hooks failed")
		return
	}

	if err := store.MarkTraceInitialized(); err != nil {
		log.Debug().Err(err).Msg("mark trace initialized failed")
	}
}
