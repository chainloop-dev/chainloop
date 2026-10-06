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

package trace

import (
	"context"
	"errors"
	"io"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// ErrAnnounceUnsupported is returned by AnnounceToUser when the agent has no
// channel for showing the user a message. It means nothing was displayed, as
// opposed to a delivery that was attempted and failed, so a caller holding
// single-use content can keep it rather than throw it away unseen.
var ErrAnnounceUnsupported = errors.New("agent cannot show messages to the user")

// SessionStartMessage is everything the session-start hook has to say, on the
// two channels an agent offers: one the user reads, one the model reads.
//
// It is one value rather than two calls because an agent parses a hook's stdout
// as a single JSON document. A second write would be discarded, silently,
// together with whatever it carried — so the two channels are made inseparable
// here rather than left to each caller to remember.
type SessionStartMessage struct {
	// Banner is shown to the user verbatim by the agent client, without
	// costing a model turn. Empty means nothing is shown.
	Banner string

	// Instruction is addressed to the model rather than the user: it reaches
	// the session's context so the agent can act on it. Empty means the model
	// is told nothing.
	Instruction string
}

// Empty reports that there is nothing to deliver on either channel.
func (m SessionStartMessage) Empty() bool {
	return m.Banner == "" && m.Instruction == ""
}

// Provider discovers and parses AI coding sessions for a specific agent.
//
// Providers are stateless singletons from a registry, so the state-touching
// methods below take a *state.Store per call rather than holding one. The store
// already knows where trace state lives — the .git directory inside a
// repository, or the out-of-tree directory `chainloop trace run` picks outside
// one — so implementations never reason about that distinction.
type Provider interface {
	// Name returns the agent identifier (e.g., "claude-code", "cursor").
	Name() string

	// DiscoverSession finds the most relevant session for the given repo root.
	// Returns nil, nil if no matching session is found.
	DiscoverSession(repoRoot string) (*DiscoveredSession, error)

	// ParseSession parses a session and returns structured evidence.
	ParseSession(ctx context.Context, opts *ParseOpts) (*aicodingsession.Evidence, error)

	// SessionDirForRepo returns the agent's session data directory for a given repo root.
	SessionDirForRepo(repoRoot string) string

	// CopySessionData copies the agent's on-disk session artifacts into
	// the store's raw/ directory so pre-push can parse them independently of
	// the agent's own storage (which may be rotated/cleaned later).
	// loc says where the agent keeps the session's transcripts; it need not
	// be in the checkout that owns store.
	CopySessionData(store *state.Store, loc SessionLocation) error

	// CaptureFileSnapshot is invoked from the pre-edit hook to record any
	// state the provider needs to later reconstruct the file's pre-edit
	// content. Providers whose post-edit hook delivers the edit payload
	// directly (e.g. Cursor's afterFileEdit with old/new strings)
	// implement this as a no-op.
	CaptureFileSnapshot(store *state.Store, input *HookInput) error

	// ResolveBeforeContent reconstructs the file's content as it was
	// before the edit, given its current ("after") content. Returns nil
	// when no reconstruction is possible (no snapshot, no edits, or the
	// file was newly created); callers treat nil as "all lines are AI".
	ResolveBeforeContent(store *state.Store, input *HookInput, after []byte) []byte

	// CleanupAfterEdit releases any per-edit state captured in
	// CaptureFileSnapshot. Called once the post-edit handler is done with
	// the file, regardless of whether ranges were recorded.
	CleanupAfterEdit(store *state.Store, input *HookInput)

	// InstallHooks installs the agent's hooks in the repo (e.g., .claude/settings.json).
	InstallHooks(repoRoot string) error

	// InstallHooksForTraceRun installs the data-gathering subset of hooks
	// used by `chainloop trace run`. End-of-session hooks are omitted —
	// trace run drives attestation itself.
	InstallHooksForTraceRun(repoRoot string) error

	// SettingsFile returns the absolute path to the agent's on-disk
	// hooks/settings file (e.g. .claude/settings.json). Callers use it to
	// back up the file before install and restore it afterwards.
	SettingsFile(repoRoot string) string

	// UninstallHooks removes the agent's hooks from the repo.
	UninstallHooks(repoRoot string) error

	// ReadHookInput reads hook invocation input from the given reader.
	ReadHookInput(r io.Reader) (*HookInput, error)

	// IsFileWritingTool returns true if the named tool modifies files on disk.
	IsFileWritingTool(toolName string) bool

	// IsCommandTool returns true if the named tool runs a shell command (e.g.
	// Claude's "Bash"). Such tools can create or modify arbitrary files without
	// firing the file-writing hooks, so they are captured via a before/after
	// working-tree snapshot instead of a per-file snapshot.
	IsCommandTool(toolName string) bool

	// AnnounceSessionStart writes the session-start hook response to stdout,
	// as the one document the agent will read. A message with nothing on
	// either channel emits nothing, so the hook stays a no-op rather than
	// handing the agent an empty envelope to parse.
	AnnounceSessionStart(msg SessionStartMessage) error

	// SupportsSessionStartBanner reports whether the Banner of a message
	// handed to AnnounceSessionStart reaches the user rather than being
	// discarded. Callers check it before composing the banner, which costs a
	// control-plane round trip.
	SupportsSessionStartBanner() bool

	// SupportsSessionStartInstruction reports whether the Instruction of a
	// message handed to AnnounceSessionStart reaches the model rather than
	// being discarded.
	SupportsSessionStartInstruction() bool

	// AnnouncePromptSubmit writes the prompt-submit hook response to stdout,
	// so that reminder reaches the model's context for this turn. An empty
	// reminder emits nothing.
	AnnouncePromptSubmit(reminder string) error

	// SupportsPromptReminder reports whether a reminder handed to
	// AnnouncePromptSubmit reaches the model at each user prompt. An agent
	// without it gets the session-start instruction only.
	SupportsPromptReminder() bool

	// AnnounceToUser writes a hook response to stdout so the agent puts msg
	// in front of the user, after a shell command the agent ran. Which
	// channel that uses is the provider's business: agents differ in whether
	// they render text directly, relay it through the model, or both.
	//
	// Providers with no way to reach the user return ErrAnnounceUnsupported,
	// so callers can tell "shown" apart from "nothing happened" and avoid
	// discarding a message nobody saw.
	AnnounceToUser(msg string) error
}

// HookInput represents parsed hook invocation data from an AI agent.
type HookInput struct {
	// SessionID is the agent-assigned identifier for the session.
	SessionID string `json:"session_id"`
	// HookEventName is the agent-specific event name (e.g., "PreToolUse", "afterFileEdit").
	HookEventName string `json:"hook_event_name,omitempty"`
	// ToolName is the name of the tool being invoked, if applicable.
	ToolName string `json:"tool_name,omitempty"`
	// FilePath is the absolute path of the file being edited, set by provider's ReadHookInput.
	FilePath string `json:"-"`
	// Cwd is the directory the agent session runs in, which is where the
	// agent files its transcripts. It can differ from the checkout that owns
	// FilePath, e.g. for an edit in a linked git worktree. Empty when the
	// agent does not report it.
	Cwd string `json:"cwd,omitempty"`
	// TranscriptPath is the path of the session transcript as the agent
	// reports it. Unlike Cwd, it does not move when the agent works in
	// another directory (e.g. a subagent in its own git worktree). Empty
	// when the agent does not report it.
	TranscriptPath string `json:"transcript_path,omitempty"`
	// AgentID identifies the subagent the hook fires in. Subagents share
	// their parent's SessionID, so this is what tells concurrent agents of
	// one session apart. Empty for the main agent.
	AgentID string `json:"agent_id,omitempty"`
	// ToolUseID is the agent's identifier for one tool call. Its pre and post
	// hooks carry the same value, so it tells overlapping calls of one agent
	// apart. Empty when the agent does not report it.
	ToolUseID string `json:"tool_use_id,omitempty"`
	// AgentVersion is the agent runtime version reported in the hook payload
	// (e.g., Cursor's cursor_version). Captured at session-start so parsing
	// can set Agent.Version even when the transcript itself doesn't carry it.
	AgentVersion string `json:"-"`
	// Model is the model identifier reported in the hook payload (e.g.,
	// Cursor's "model" field). Captured at session-start so parsing can
	// set Model.Primary even when the transcript itself doesn't carry it.
	Model string `json:"-"`
	// Edits carries per-file string replacements reported by the agent.
	// Providers that snapshot files pre-edit leave this empty; providers that
	// only emit post-edit events (e.g., Cursor's afterFileEdit) populate it so
	// consumers can reconstruct the "before" content via reverse application.
	Edits []HookEdit `json:"-"`
	// ToolFailed reports that the hook fires after a tool call that failed
	// (Claude's PostToolUseFailure). The tool can still have changed files,
	// so its changes are recorded. But the agent expects a different hook
	// response for a failure, so nothing is written back to it.
	ToolFailed bool `json:"-"`
}

// HookEdit represents a single old_string → new_string replacement applied to a file.
type HookEdit struct {
	// OldString is the text that was replaced.
	OldString string
	// NewString is the text that replaced OldString.
	NewString string
}

// SessionLocation tells a provider where to find a session's transcripts.
type SessionLocation struct {
	// SessionID identifies the session.
	SessionID string
	// Cwd is the directory the session runs in. Providers that key their
	// storage by directory use it when TranscriptPath is empty.
	Cwd string
	// TranscriptPath is the transcript path the agent reported, if any.
	// Providers that get one prefer it over Cwd.
	TranscriptPath string
}

// DiscoveredSession represents a discovered AI coding session (agent-agnostic).
type DiscoveredSession struct {
	SessionID  string
	SessionDir string
	IsActive   bool
}

// ParseOpts configures session parsing.
type ParseOpts struct {
	// SessionDir is the directory holding the copied session transcript.
	SessionDir string
	// SessionID identifies the session to parse.
	SessionID string
	// AgentVersion is the runtime version captured at session-start (when
	// the agent reports it via hook payload). Providers whose transcripts
	// don't embed a version can use this to populate Agent.Version.
	AgentVersion string
	// Model is the model identifier captured at session-start. Providers
	// whose transcripts don't embed model info can use this to populate
	// Model.Primary.
	Model string
}

// DefaultProviderName is the provider used when a SessionRecord predates the
// Provider field or when no explicit provider is selected. Kept here so
// both the providers registry and the action package can reference it
// without a package import.
const DefaultProviderName = "claude-code"
