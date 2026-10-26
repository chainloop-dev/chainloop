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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/repositoryconfig"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/hooks"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/spec"
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

	// Pre-push finds the transcript through the session record, and this
	// can be the first hook since its creation that reports the path.
	if input.TranscriptPath != "" {
		rec, err := store.LoadSessionRecord(sessionID)
		if err != nil {
			log.Debug().Err(err).Msg("load session record failed")
		}
		recordTranscriptPath(store, rec, input.TranscriptPath, log)
	}

	if err := provider.CopySessionData(store, sessionLocation(input.SessionID, input.Cwd, input.TranscriptPath, repoRoot)); err != nil {
		log.Debug().Err(err).Msg("copy session data failed")
	}

	// The last turn can have used a skill that no hook saw yet. The copies
	// stay after the session end, for a push that comes later.
	captureSkillLoads(provider, store, input, log)

	setSessionActive(store, sessionID, false, log)

	// The spec stays on disk for the whole session, so that every push of it
	// records the spec as it is at that moment. The session end is where it
	// goes, with the redacted copies that spared each later push a scan. A
	// push after this point attests the session without its spec.
	if err := spec.Remove(repoRoot, sessionID); err != nil {
		log.Debug().Err(err).Str("session_id", sessionID).Msg("session-end: could not remove the session spec")
	}
	if err := store.RemoveSpecRedactions(sessionID); err != nil {
		log.Debug().Err(err).Str("session_id", sessionID).Msg("session-end: could not remove the redacted spec copies")
	}

	log.Debug().Str("session_id", sessionID).Msg("session ended")

	return nil
}

// setSessionActive flips the Active flag on an existing session record. The
// record is otherwise written once and never upserted, so without this a
// session stays marked active for the life of the repository.
//
// Active is not a reliable liveness signal on its own and must not decide
// attribution: session-end is not installed for `trace run`, and never fires
// on SIGKILL or a closed terminal. It only narrows the set of sessions a push
// attests when there are no commits to go on.
func setSessionActive(store *state.Store, sessionID string, active bool, log zerolog.Logger) {
	rec, err := store.LoadSessionRecord(sessionID)
	if err != nil {
		log.Debug().Err(err).Msg("load session record failed")

		return
	}
	if rec == nil || rec.Active == active {
		return
	}

	rec.Active = active
	if err := store.SaveSessionRecord(rec); err != nil {
		log.Debug().Err(err).Bool("active", active).Msg("update session record failed")
	}
}

// sessionStartBanner is what the developer sees at the top of a traced
// session. Its audience includes someone who cloned the repository and never
// ran init, so it says what is happening in plain words and, when we know it,
// where the evidence goes. Chainloop's own vocabulary is deliberately absent:
// "attested" does not tell a newcomer whether something is recorded,
// uploaded, or signed, let alone to where.
//
// Each fact is dropped rather than guessed at when it is unknown, so the
// banner never promises a destination that was not confirmed.
func sessionStartBanner(dashboardURL, org, project string) string {
	var identity []string
	if org != "" {
		identity = append(identity, "organization: "+org)
	}
	if project != "" {
		identity = append(identity, "project: "+project)
	}
	where := strings.Join(identity, ", ")

	// Destination and identity share a line, since they answer one question
	// between them: where this is going. The space before the parenthesis
	// matters, as it is what lets a terminal linkify the URL without
	// swallowing the punctuation that follows it.
	switch {
	case dashboardURL != "" && where != "":
		where = "Evidence will be sent to " + strings.TrimRight(dashboardURL, "/") + " (" + where + ")"
	case dashboardURL != "":
		where = "Evidence will be sent to " + strings.TrimRight(dashboardURL, "/")
	}

	banner := "Chainloop Trace is recording this session."
	if where != "" {
		banner += "\n" + where
	}

	return banner
}

// localSessionStartBanner is the session-start banner for a local-mode
// repository. It names the local mode outright and says where the evidence
// lands, with no dashboard or attestation vocabulary and no control-plane round
// trip: local mode has no dashboard to point at (R-012).
func localSessionStartBanner(tracePath string, traceRunActive bool) string {
	banner := "Chainloop Trace is recording this session in local mode."
	if tracePath != "" {
		// A git-hook session stores its evidence on the next push. A `trace run`
		// session stores it as soon as the wrapped command exits, so the timing
		// has to match how the session is driven, not always claim a push.
		when := "when you push"
		if traceRunActive {
			when = "when this run finishes"
		}
		banner += "\nChainloop Trace writes the session evidence to " + tracePath + " " + when + "."
	}

	return banner
}

// TraceAuthCheck asks the control plane whether the CLI credentials are
// accepted for the organization. An empty organization uses the default one.
type TraceAuthCheck func(ctx context.Context, organization string) error

// sessionAuthProblem runs checkAuth and returns the message the CLI shows for
// an authentication failure, or an empty string. The control plane decides
// whether the credentials are valid. Any other failure, for example an
// unreachable control plane or a check that takes longer than the timeout,
// returns an empty string: the hook does not guess.
func sessionAuthProblem(checkAuth TraceAuthCheck, organization string, log zerolog.Logger) string {
	if checkAuth == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), hookAuthCheckTimeout)
	defer cancel()

	// The check can ignore ctx, so it runs in its own goroutine. The hook
	// process exits soon after, so an abandoned check stops with it.
	done := make(chan error, 1)
	go func() { done <- checkAuth(ctx, organization) }()

	var err error
	select {
	case err = <-done:
	case <-ctx.Done():
		log.Debug().Msg("session-start: the auth check timed out")
		return ""
	}

	msg, isAuth := AuthErrorMessage(err)
	if !isAuth {
		if err != nil {
			log.Debug().Err(err).Msg("session-start: the auth check failed")
		}

		return ""
	}

	return msg
}

// authProblemBanner tells the developer that the session will not be uploaded
// because the control plane does not accept the credentials. The push is the
// first step that uses them, and it comes after the session work is done.
func authProblemBanner(authMsg string) string {
	return "Chainloop Trace cannot upload this session when you push: " + authMsg + "."
}

// authProblemInstruction is the authProblemBanner for an agent that has no
// banner. The model is the only channel to the developer.
func authProblemInstruction(authMsg string) string {
	return "Chainloop Trace cannot upload this session when the user pushes, because the Chainloop CLI credentials are not accepted: " +
		authMsg + ". Tell the user now, in one line, to fix this before they push."
}

// HandleAgentSessionStart handles the agent session-start hook. In
// control-plane mode, checkAuth asks the control plane whether the credentials
// that the pre-push hook will use are accepted, so that the developer knows
// about a problem before the session work is done. A nil checkAuth skips the
// check.
func HandleAgentSessionStart(provider trace.Provider, checkAuth TraceAuthCheck, log zerolog.Logger) error {
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

	// An agent can resume a session after its end hook ran, and the record is
	// created once and never upserted, so revive the flag here. Only
	// session-start may do this: the tool hooks share ensureSessionTracked, and
	// one of those arriving late would resurrect a session that really has
	// ended. Revival keeps StartedAt and the metadata captured on the first
	// call intact.
	setSessionActive(store, input.SessionID, true, log)

	// Each part is composed only for an agent that can receive it. The banner
	// in particular costs a control-plane round trip, and an agent that
	// discards it would make the developer pay the wait for nothing.
	var msg trace.SessionStartMessage

	// A `trace run` session takes its mode from the run. Other sessions take
	// it from traceStorage in .chainloop.yml.
	local := config.IsLocalStorage(repoRoot)
	if store.IsTraceRunActive() {
		local = store.IsTraceRunLocal()
	}

	// Local mode uploads nothing, so only the control-plane mode needs
	// credentials.
	var authMsg string
	if !local {
		authMsg = sessionAuthProblem(checkAuth, config.LoadOrganizationFromYML(repoRoot), log)
	}

	if provider.SupportsSessionStartInstruction() {
		remindsEachTurn := provider.SupportsPromptReminder()
		deduplicator, ok := provider.(interface{ DeduplicatesSessionStartInstruction() bool })
		if ok && deduplicator.DeduplicatesSessionStartInstruction() {
			// Pi persists its own per-session marker. Give it the full instruction
			// after reload/resume and let that marker decide whether delivery is new.
			remindsEachTurn = false
		}
		msg.Instruction = sessionSpecInstruction(repoRoot, input.SessionID, remindsEachTurn, log)
		// An agent with a banner tells the developer directly.
		if authMsg != "" && !provider.SupportsSessionStartBanner() {
			msg.Instruction = strings.TrimSpace(authProblemInstruction(authMsg) + "\n\n" + msg.Instruction)
		}
	}

	if provider.SupportsSessionStartBanner() {
		var banner string
		if local {
			// Local mode reaches no control plane, so the banner names the
			// on-disk destination and makes no dashboard round trip (R-012).
			banner = localSessionStartBanner(
				store.LocalSessionsDir(),
				store.IsTraceRunActive(),
			)
		} else {
			banner = sessionStartBanner(
				hookDashboardURL(log),
				config.LoadOrganizationFromYML(repoRoot),
				repositoryconfig.LoadProjectFromYML(repoRoot),
			)
			if authMsg != "" {
				banner += "\n\n" + authProblemBanner(authMsg)
			}
		}
		msg.Banner = "\n\n" + banner + "\n"
	}

	if msg.Empty() {
		return nil
	}

	if err := provider.AnnounceSessionStart(msg); err != nil {
		log.Debug().Err(err).Msg("session-start: failed to send the session-start message")
	}

	return nil
}

// HandleAgentPromptSubmit handles the agent prompt-submit hook. It adds the
// short spec capture reminder to the context of the turn, so that a spec given
// or changed after the session start also reaches the evidence. It never
// blocks the prompt: any failure only drops the reminder.
func HandleAgentPromptSubmit(provider trace.Provider, log zerolog.Logger) error {
	if !provider.SupportsPromptReminder() {
		return nil
	}

	input, err := provider.ReadHookInput(os.Stdin)
	if err != nil || !state.ValidSessionID(input.SessionID) {
		log.Debug().Err(err).Msg("prompt-submit: no valid input")
		return nil
	}

	log.Debug().Str("session_id", input.SessionID).Msg("prompt-submit hook invoked")

	store, repoRoot, err := state.Locate()
	if err != nil {
		log.Debug().Err(err).Msg("prompt-submit: no trace state located")
		return nil
	}

	// A skill that the user started with a slash command fires no tool hook.
	// This is the first hook after it when the skill calls no tool.
	captureSkillLoads(provider, store, input, log)

	reminder := sessionSpecReminder(repoRoot, input.SessionID, log)
	if err := provider.AnnouncePromptSubmit(reminder); err != nil {
		log.Debug().Err(err).Msg("prompt-submit: failed to send the spec reminder")
	}

	return nil
}

// hookDashboardURL asks the control plane where its web dashboard lives, so
// the session banner can name the destination the evidence is bound for.
// Returns an empty string when there is no dashboard, no reachable control
// plane, or no time to find out, in which case the banner simply omits the
// line.
//
// This is the one network call the session-start hook makes, and it is
// deliberately cheap to abandon: Infoz needs no credentials, so no token is
// loaded, and the timeout is short because a developer waiting to type is a
// worse cost than a missing line.
func hookDashboardURL(log zerolog.Logger) string {
	conn, err := newControlPlaneConnection("", "")
	if err != nil {
		log.Debug().Err(err).Msg("session-start: no control plane connection for the banner")
		return ""
	}
	defer func() { _ = conn.Close() }()

	return fetchUIDashboardURL(context.Background(), conn, hookDashboardURLTimeout)
}

// HandleAgentPreToolUse handles the agent pre-tool-use hook.
func HandleAgentPreToolUse(provider trace.Provider, log zerolog.Logger) error {
	input, err := provider.ReadHookInput(os.Stdin)
	if err != nil || !state.ValidSessionID(input.SessionID) {
		log.Debug().Err(err).Msg("pre-tool-use: no valid input")
		return nil
	}

	for _, fileInput := range fileHookInputs(input) {
		if err := handleAgentPreToolUse(provider, fileInput, log); err != nil {
			return err
		}
	}
	return nil
}

func handleAgentPreToolUse(provider trace.Provider, input *trace.HookInput, log zerolog.Logger) error {
	log.Debug().Str("session_id", input.SessionID).Msg("pre-tool-use hook invoked")

	store, repoRoot, err := locateForHook(provider, input)
	if err != nil {
		log.Debug().Err(err).Msg("pre-tool-use: no trace state located")
		return nil
	}

	ensureSessionTracked(provider, store, repoRoot, input, log)
	captureSkillLoads(provider, store, input, log)

	switch {
	case provider.IsCommandTool(input.ToolName):
		// Shell command: snapshot the whole worktree so the post hook can diff
		// it and attribute the command's file changes to the AI. Register the
		// checkouts the command names first, so the snapshot covers them.
		registerCommandCheckouts(provider, input, repoRoot, log)
		captureWorktreeSnapshot(store, repoRoot, shellCallKey(input), log)
		tagShellCommand(provider, input, log)
	case provider.IsFileWritingTool(input.ToolName):
		if input.FilePath == "" {
			log.Debug().Str("tool", input.ToolName).Msg("pre-tool-use: file-writing tool produced no file path, skipping")
			return nil
		}
		if err := provider.CaptureFileSnapshot(store, input); err != nil {
			log.Debug().Err(err).Str("file", input.FilePath).Msg("pre-tool-use: capture snapshot failed")
		}
		registerSessionCheckout(input.SessionID, repoRoot, log)
	default:
		log.Debug().Str("tool", input.ToolName).Msg("pre-tool-use: not a tracked tool, skipping")
	}

	return nil
}

// shellCallKey returns the key that pairs the pre and post hooks of one shell
// command.
func shellCallKey(input *trace.HookInput) state.ShellCallKey {
	return state.ShellCallKey{SessionID: input.SessionID, AgentID: input.AgentID, ToolUseID: input.ToolUseID}
}

// captureWorktreeSnapshot records the working-tree signatures before a shell
// command runs, so HandleAgentPostToolUse can diff them and attribute the
// files the command changed. It snapshots the session's own checkout and
// every other checkout that the session edited with a file tool: a command
// like `cd <other checkout> && …` changes files there, and shell hooks only
// run in the session's own checkout. Best-effort: failures are logged and
// never block the agent.
func captureWorktreeSnapshot(store *state.Store, repoRoot string, key state.ShellCallKey, log zerolog.Logger) {
	client := tracegit.NewGitClient()
	for _, root := range shellSnapshotRoots(store, repoRoot, key.SessionID) {
		worktreeSig, err := client.SnapshotWorktree(root)
		if err != nil {
			log.Debug().Err(err).Str("root", root).Msg("pre-command: worktree snapshot failed")
			continue
		}
		indexSig, err := client.SnapshotIndex(root)
		if err != nil {
			log.Debug().Err(err).Str("root", root).Msg("pre-command: index snapshot failed")
		}

		rootStore := store
		if !sameDir(root, repoRoot) {
			s, resolvedRoot, err := state.LocateFrom(root)
			if err != nil {
				log.Debug().Err(err).Str("root", root).Msg("pre-command: no trace state for checkout")
				continue
			}
			rootStore, root = s, resolvedRoot
			if err := copySessionRecord(store, rootStore, key.SessionID); err != nil {
				log.Debug().Err(err).Str("session_id", key.SessionID).Msg("pre-command: copy session record failed")
			}
		}

		heads := make(map[string]string)
		// A checkout with no commit yet has no HEAD; its files count as new.
		if sha, _, err := client.CommitHeadInfo(root); err == nil {
			heads[root] = sha
		}

		if err := rootStore.SaveShellPreSignature(key, state.WorktreeSignatures{root: worktreeSig}); err != nil {
			log.Debug().Err(err).Str("root", root).Msg("pre-command: save worktree signature failed")
		}
		if indexSig != nil {
			if err := rootStore.SaveShellPreIndexSignature(key, state.WorktreeSignatures{root: indexSig}); err != nil {
				log.Debug().Err(err).Str("root", root).Msg("pre-command: save index signature failed")
			}
		}
		if err := rootStore.SaveShellPreHeads(key, heads); err != nil {
			log.Debug().Err(err).Str("root", root).Msg("pre-command: save HEAD commits failed")
		}
	}
}

func copySessionRecord(source, target *state.Store, sessionID string) error {
	rec, err := source.LoadSessionRecord(sessionID)
	if err != nil {
		return fmt.Errorf("load session record: %w", err)
	}
	if rec == nil {
		return nil
	}

	recordCopy := *rec
	if err := target.SaveSessionRecord(&recordCopy); err != nil {
		return fmt.Errorf("save session record: %w", err)
	}

	return nil
}

// shellSnapshotRoots returns the checkouts that a shell command of the session
// is diffed in: the session's own checkout, then the other checkouts on its
// session record that still exist.
func shellSnapshotRoots(store *state.Store, repoRoot, sessionID string) []string {
	roots := []string{repoRoot}

	rec, err := store.LoadSessionRecord(sessionID)
	if err != nil || rec == nil {
		return roots
	}

	for _, root := range rec.Checkouts {
		if sameDir(root, repoRoot) {
			continue
		}
		if _, err := os.Stat(root); err != nil {
			continue
		}
		roots = append(roots, root)
	}

	return roots
}

// registerSessionCheckout adds fileRoot to the checkout list on the session
// record of the session's own checkout, when a file tool edits a file in a
// different checkout. Shell hooks then also snapshot that checkout.
//
// The record is never created here: it is missing only when the session's
// directory is not a repository (shell hooks record nothing then) or the hooks
// were installed partway through the session, and creating it would also copy
// the transcripts into that checkout. Best-effort: failures are logged and
// never block the agent.
func registerSessionCheckout(sessionID, fileRoot string, log zerolog.Logger) {
	homeStore, homeRoot, err := state.Locate()
	if err != nil || sameDir(homeRoot, fileRoot) {
		return
	}

	rec, err := homeStore.LoadSessionRecord(sessionID)
	if err != nil || rec == nil {
		log.Debug().Err(err).Str("root", fileRoot).Msg("no session record in the session's checkout; shell commands will not snapshot this checkout")
		return
	}

	if slices.Contains(rec.Checkouts, fileRoot) {
		return
	}

	rec.Checkouts = append(rec.Checkouts, fileRoot)
	slices.Sort(rec.Checkouts)
	if err := homeStore.SaveSessionRecord(rec); err != nil {
		log.Debug().Err(err).Str("root", fileRoot).Msg("register session checkout failed")
	}
}

// registerCommandCheckouts handles each checkout that a shell command names as
// a file tool edit there is handled: it installs the git hooks, writes the
// session record, and adds the checkout to the session's checkout list
// (PFM-7614). Shell hooks run in the session's directory, so without this a
// session that works in another checkout through the shell only, for example
// in a fresh clone, never reaches it, and its commits there carry no session.
// homeRoot is the session's own checkout, which needs none of this.
func registerCommandCheckouts(provider trace.Provider, input *trace.HookInput, homeRoot string, log zerolog.Logger) {
	if input.Command == "" {
		return
	}

	cwd := input.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	for _, dir := range commandCheckoutDirs(input.Command, cwd) {
		// A directory that does not exist yet (the target of a clone, at the
		// pre hook) must not resolve to a repository above it.
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		store, root, err := state.LocateFrom(dir)
		if err != nil || !store.IsGit() || sameDir(root, homeRoot) {
			continue
		}

		log.Debug().Str("root", root).Msg("shell command works in another checkout")
		ensureSessionTracked(provider, store, root, input, log)
		registerSessionCheckout(input.SessionID, root, log)
	}
}

// sameDir reports whether a and b name the same directory, ignoring symlinks
// (e.g. /var and /private/var on macOS).
func sameDir(a, b string) bool {
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}

	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)

	return errA == nil && errB == nil && ra == rb
}

// ensureSessionTracked auto-installs git hooks (unconditionally, on every
// invocation — itself idempotent via hooks.IsInstalled), then creates a
// session record if one doesn't already exist and copies the session data.
// Idempotent so any hook entry point can call it safely. Per-input metadata
// (e.g., AgentVersion, Cwd) is captured on the first call only — subsequent
// hooks for the same session are no-ops, no upsert. The one exception is
// TranscriptPath, which a later hook fills in when the first one lacked it,
// or replaces when the recorded file is gone.
func ensureSessionTracked(provider trace.Provider, store *state.Store, repoRoot string, input *trace.HookInput, log zerolog.Logger) {
	// Run install before any early return so hooks recover from missing
	// state (deleted .git/hooks, project YAML appearing after session start).
	// Outside a repository there is nothing to invoke git hooks, so there is
	// nothing to install.
	if store.IsGit() {
		autoInstallGitHooks(store, repoRoot, log)
	}

	sessionID := input.SessionID
	// A hook that reports a transcript path has to read the record, to see
	// whether it still lacks one. Without a path, a stat is enough.
	if input.TranscriptPath == "" {
		if store.SessionRecordExists(sessionID) {
			return
		}
	} else {
		rec, err := store.LoadSessionRecord(sessionID)
		if err != nil {
			log.Debug().Err(err).Msg("load session record failed")
			return
		}
		if rec != nil {
			backfillTranscriptPath(provider, store, repoRoot, rec, input.TranscriptPath, log)
			return
		}
	}

	log.Debug().Str("session_id", sessionID).Str("state_dir", store.Dir()).Str("provider", provider.Name()).Msg("tracking new session")

	rec := &state.SessionRecord{
		SessionID:      sessionID,
		Provider:       provider.Name(),
		AgentVersion:   input.AgentVersion,
		Model:          input.Model,
		Cwd:            input.Cwd,
		TranscriptPath: input.TranscriptPath,
		Active:         true,
		StartedAt:      state.NowTimestamp(),
	}
	if err := store.SaveSessionRecord(rec); err != nil {
		log.Debug().Err(err).Msg("save session record failed")
		return
	}

	if err := provider.CopySessionData(store, sessionLocation(input.SessionID, input.Cwd, input.TranscriptPath, repoRoot)); err != nil {
		log.Debug().Err(err).Msg("copy session data failed")
	}
}

// backfillTranscriptPath records the transcript path on an existing session
// record that lacks it, or whose transcript moved, and copies the session
// data from there. Without it, a record created by a hook that carried no
// transcript path would fall back to the cwd lookup for the life of the
// session, and a moved transcript would never reach pre-push.
func backfillTranscriptPath(provider trace.Provider, store *state.Store, repoRoot string, rec *state.SessionRecord, transcriptPath string, log zerolog.Logger) {
	if !recordTranscriptPath(store, rec, transcriptPath, log) {
		return
	}

	if err := provider.CopySessionData(store, sessionLocation(rec.SessionID, rec.Cwd, rec.TranscriptPath, repoRoot)); err != nil {
		log.Debug().Err(err).Msg("copy session data failed")
	}
}

// recordTranscriptPath saves transcriptPath on rec when rec has none yet, or
// when the file of its path is gone, and reports whether it did. The agent
// can move the transcript of a running session: Claude Code files a resumed
// session under the project folder of the directory it resumes from. While
// the recorded file exists, another path, such as the transcript of a
// subagent, does not replace it.
func recordTranscriptPath(store *state.Store, rec *state.SessionRecord, transcriptPath string, log zerolog.Logger) bool {
	if rec == nil || transcriptPath == "" || rec.TranscriptPath == transcriptPath {
		return false
	}
	// Only a file that is gone is replaced. Another stat error does not show
	// that the transcript moved.
	if rec.TranscriptPath != "" {
		if _, err := os.Stat(rec.TranscriptPath); !errors.Is(err, os.ErrNotExist) {
			return false
		}
	}

	rec.TranscriptPath = transcriptPath
	if err := store.SaveSessionRecord(rec); err != nil {
		log.Debug().Err(err).Msg("backfill transcript path failed")
		return false
	}

	return true
}

// locateForHook returns the trace state an agent hook records into. A file
// edit belongs to the checkout that owns the file, which is not necessarily
// the one the hook runs from: agents run every hook from the directory the
// session started in, even when the edit lands in a linked git worktree, and
// that worktree's commits only read its own state (PFM-7412). Everything else
// (session lifecycle, shell commands) has no file to go by and stays with cwd.
//
// Only file-writing tools qualify. Other tools can carry a path too (Claude's
// Read does), and reading a file in another repository must not start a
// session there.
func locateForHook(provider trace.Provider, input *trace.HookInput) (*state.Store, string, error) {
	if input.FilePath != "" && provider.IsFileWritingTool(input.ToolName) {
		return state.LocateFrom(filepath.Dir(input.FilePath))
	}

	return state.Locate()
}

// sessionLocation returns where to find a session's transcripts, from what
// the agent reported (in a hook payload or a session record).
//
// agentCwd falls back to the checkout root for agents that do not report
// one. It must not be replaced by the checkout root otherwise: for a session
// recorded in a linked worktree, the worktree root names a transcript
// directory the agent never wrote to.
func sessionLocation(sessionID, agentCwd, transcriptPath, repoRoot string) trace.SessionLocation {
	if agentCwd == "" {
		agentCwd = repoRoot
	}

	return trace.SessionLocation{SessionID: sessionID, Cwd: agentCwd, TranscriptPath: transcriptPath}
}

// notifyPendingSessionLinks hands any session links left by a just-completed
// trace push to the agent, so it can put them in front of the user.
//
// Best effort throughout: this is a notification, and neither a missing link
// nor a provider that cannot deliver one is worth failing an agent's tool
// call over.
func notifyPendingSessionLinks(provider trace.Provider, store *state.Store, log zerolog.Logger) {
	links := store.PendingLinks()
	if len(links) == 0 {
		return
	}

	// Same wording as the pre-push log line, so the two channels match.
	lines := make([]string, 0, len(links))
	for _, link := range links {
		lines = append(lines, sessionLinkMessage(link))
	}

	err := provider.AnnounceToUser(strings.Join(lines, "\n"))
	if errors.Is(err, trace.ErrAnnounceUnsupported) {
		// Nothing was shown, so leave the links for an agent that can show
		// them. Their expiry bounds how long they linger.
		log.Debug().Msg("agent cannot show messages; leaving the session links for later")

		return
	}

	if err != nil {
		log.Debug().Err(err).Msg("could not surface session links through the agent")
	} else {
		log.Debug().Int("links", len(links)).Msg("session links handed to the agent")
	}

	// Success or failure, the attempt is spent: retrying on every later shell
	// command would nag far longer than one dropped notification costs.
	//
	// Announcing before clearing makes this at-least-once by choice. The
	// agent never acknowledges what it rendered, so exactly-once is not
	// available at any price, and the other ordering trades a repeated line
	// for a link nobody ever sees.
	store.ClearPendingLinks()
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

	for _, fileInput := range fileHookInputs(input) {
		if err := handleAgentPostToolUse(provider, fileInput, log); err != nil {
			return err
		}
	}
	return nil
}

func handleAgentPostToolUse(provider trace.Provider, input *trace.HookInput, log zerolog.Logger) error {
	sessionID := input.SessionID

	isCommand := provider.IsCommandTool(input.ToolName)
	isFileWriting := provider.IsFileWritingTool(input.ToolName)
	if !isCommand && !isFileWriting {
		// The hook fires for the skill tool too, whose skill is loaded only
		// after the call. The skill is the one thing to record for it.
		if store, _, err := state.Locate(); err == nil {
			captureSkillLoads(provider, store, input, log)
		}

		log.Debug().Str("tool", input.ToolName).Msg("post-tool-use: not a tracked tool, skipping")
		return nil
	}
	if isFileWriting && input.FilePath == "" {
		log.Debug().Str("tool", input.ToolName).Msg("post-tool-use: file-writing tool produced no file path, skipping")
		return nil
	}

	log.Debug().Str("session_id", sessionID).Str("tool", input.ToolName).Str("file", input.FilePath).Msg("post-tool-use hook invoked")

	store, repoRoot, err := locateForHook(provider, input)
	if err != nil {
		log.Debug().Err(err).Msg("post-tool-use: no trace state located")
		return nil
	}

	// Guarantees session tracking + git-hook install when afterFileEdit is
	// the first hook we see (Cursor has no pre hook for file edits).
	ensureSessionTracked(provider, store, repoRoot, input, log)

	if isCommand {
		// Shell command: diff the before/after worktree snapshots and attribute
		// every file the command changed to the AI.
		recordCommandLineRanges(provider, input, store, repoRoot, log)

		// A checkout that the command created (`git clone`) did not exist
		// at the pre hook.
		registerCommandCheckouts(provider, input, repoRoot, log)

		// The command may have been a `git push`, whose pre-push hook attested
		// a session and left its link behind. Show it now: the pre-push output
		// went to this tool call's captured stderr, which the user does not
		// necessarily read. A failed call expects a different hook response,
		// so its links stay on disk for the next command that succeeds.
		if !input.ToolFailed {
			notifyPendingSessionLinks(provider, store, log)
		}

		return nil
	}

	// Cursor has no pre hook for file edits, so the post hook registers the
	// checkout too.
	registerSessionCheckout(sessionID, repoRoot, log)

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

	// Normalize to repo-relative path for storage — git diffs use relative paths,
	// so ai-lines keys must match for commit correlation and attribution to work.
	relPath, err := filepath.Rel(repoRoot, input.FilePath)
	if err != nil {
		relPath = input.FilePath
	}

	recordFileEdit(store, sessionID, relPath, before, after, log)

	provider.CleanupAfterEdit(store, input)

	return nil
}

// recordFileEdit records one agent edit of a file in the session's ledger.
func recordFileEdit(store *state.Store, sessionID, relPath string, before, after []byte, log zerolog.Logger) {
	// Lines of the session that changed since its last edit of the file were
	// changed by someone else (R-002). They close before this edit records its
	// own lines, so that a line this edit writes again is the edit's.
	replaced := attribution.ReplacedLines(store.LoadAILineAttribution(sessionID).Open[relPath], before)
	if err := store.SettleLines(sessionID, relPath, replaced); err != nil {
		log.Debug().Err(err).Msg("post-tool-use: settle replaced lines failed")
	}

	edit := attribution.ComputeEdit(before, after)
	if !edit.Changed {
		return
	}

	if err := store.RecordEdit(sessionID, relPath, editRecord(edit)); err != nil {
		log.Debug().Err(err).Msg("post-tool-use: record line ranges failed")
	} else {
		log.Debug().Int("ranges", len(edit.Ranges)).Str("file", relPath).Msg("line ranges recorded")
	}
}

// recordCommandLineRanges diffs each worktree signature captured before a
// shell command against its current worktree, and records the changes in the
// ledger of the checkout that holds them. store and repoRoot are the
// session's own checkout, which holds the signatures. Best-effort: never
// blocks the agent.
func recordCommandLineRanges(provider trace.Provider, input *trace.HookInput, store *state.Store, repoRoot string, log zerolog.Logger) {
	key := shellCallKey(input)
	for _, root := range shellSnapshotRoots(store, repoRoot, key.SessionID) {
		rootStore := store
		if !sameDir(root, repoRoot) {
			s, r, err := state.LocateFrom(root)
			if err != nil {
				log.Debug().Err(err).Str("root", root).Msg("post-command: no trace state for checkout")
				continue
			}
			ensureSessionTracked(provider, s, r, input, log)
			rootStore, root = s, r
		}

		before, err := rootStore.LoadShellPreSignature(key)
		if err != nil {
			log.Debug().Err(err).Str("root", root).Msg("post-command: no pre-command worktree signature")
			continue
		}
		// A pre hook from an older CLI saved no HEAD. That case, and a heads
		// file that cannot be read, fall back to the current HEAD.
		heads, err := rootStore.LoadShellPreHeads(key)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Debug().Err(err).Str("root", root).Msg("post-command: cannot read pre-command HEAD commits; using the current HEAD")
		}

		for snapshotRoot, rootBefore := range before {
			if sameDir(snapshotRoot, root) {
				base := heads[snapshotRoot]
				if base == "" {
					base = "HEAD"
				}
				recordRootChanges(rootStore, root, base, rootBefore, key.SessionID, log)
				break
			}
		}
		rootStore.DeleteShellPreSignature(key)
	}
}

// recordRootChanges diffs the signature of one checkout captured before a
// shell command against its current worktree, and records every
// created/modified file and every deleted file as AI-attributed.
//
// The signature holds hashes, not content, so the content before the command
// is not known. The file at base, the HEAD commit before the command, stands in
// for it: lines the command did not touch are then not claimed, also when the
// command commits its own change. Uncommitted lines are in the diff too. Those
// that an earlier edit of the session recorded are left out (DropOpen); other
// ones, such as uncommitted human lines, are claimed. An untracked file counts
// as new.
func recordRootChanges(store *state.Store, repoRoot, base string, before map[string]string, sessionID string, log zerolog.Logger) {
	client := tracegit.NewGitClient()
	after, err := client.SnapshotWorktree(repoRoot)
	if err != nil {
		log.Debug().Err(err).Str("root", repoRoot).Msg("post-command: worktree snapshot failed")
		return
	}

	atBase := func(relPath string) []byte {
		content, err := client.FileAt(repoRoot, base, filepath.ToSlash(relPath))
		if err != nil {
			return nil
		}

		return content
	}

	open := store.LoadAILineAttribution(sessionID).Open
	record := func(relPath string, edit attribution.Edit) error {
		edit = attribution.DropOpen(edit, open[relPath])
		if err := store.RecordEdit(sessionID, relPath, editRecord(edit)); err != nil {
			return err
		}
		// Fingerprints intentionally ignore whitespace. Keep a legacy touch
		// marker when a real line diff therefore has counts but no fingerprints,
		// so binary CRLF changes still stay pending for a later commit.
		if len(edit.Added) == 0 && len(edit.Removed) == 0 &&
			(edit.AddedCount > 0 || edit.RemovedCount > 0) {
			return store.RecordLineRanges(sessionID, relPath, nil)
		}

		return nil
	}

	changed, deleted := attribution.ChangedPaths(before, after)
	for _, relPath := range changed {
		content, err := os.ReadFile(filepath.Join(repoRoot, relPath))
		if err != nil {
			continue
		}
		beforeContent := atBase(relPath)
		if _, existed := before[relPath]; !existed {
			// The call created this path after its latest checkpoint. HEAD can
			// still hold the old file when that checkpoint staged and committed
			// a deletion, so it is not the content before this new edit.
			beforeContent = nil
		}
		// ChangedPaths already proved the file changed, so record it even when
		// the diff finds nothing (created empty, or restored to base): the
		// file is then AI-touched with zero lines, as a deletion-only edit.
		if err := record(relPath, attribution.ComputeEdit(beforeContent, content)); err != nil {
			log.Debug().Err(err).Str("file", relPath).Msg("post-command: record line ranges failed")
		}
	}
	for _, relPath := range deleted {
		// The removed lines are the file at base, matching the file-writing
		// hook's treatment of deletions.
		if err := record(relPath, attribution.ComputeEdit(atBase(relPath), nil)); err != nil {
			log.Debug().Err(err).Str("file", relPath).Msg("post-command: record deletion failed")
		}
	}

	log.Debug().Int("changed", len(changed)).Int("deleted", len(deleted)).Str("session_id", sessionID).Str("root", repoRoot).Msg("command line ranges recorded")
}

// editRecord converts a computed edit into its ledger record.
func editRecord(edit attribution.Edit) state.EditRecord {
	return state.EditRecord{
		Ranges:       edit.Ranges,
		Lines:        state.LineSet{Added: edit.Added, Removed: edit.Removed},
		AddedCount:   edit.AddedCount,
		RemovedCount: edit.RemovedCount,
	}
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

	// Auto-install needs a repository that is set up for trace: a control-plane
	// project name, or local storage mode. A local-mode repository writes no
	// projectName, so without the local check its hooks would never self-heal
	// after a re-clone or a manual delete.
	if repositoryconfig.LoadProjectFromYML(repoRoot) == "" && !config.IsLocalStorage(repoRoot) {
		return
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

// fileHookInputs expands a multi-file edit into the existing per-file path.
func fileHookInputs(input *trace.HookInput) []*trace.HookInput {
	if len(input.FilePaths) == 0 {
		return []*trace.HookInput{input}
	}
	inputs := make([]*trace.HookInput, 0, len(input.FilePaths))
	for _, path := range input.FilePaths {
		one := *input
		one.FilePath = path
		one.FilePaths = nil
		inputs = append(inputs, &one)
	}
	return inputs
}
