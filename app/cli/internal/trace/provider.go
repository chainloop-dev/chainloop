package trace

import (
	"context"
	"io"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

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
	CopySessionData(store *state.Store, repoRoot, sessionID string) error

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

	// SystemMessage writes a message to stdout for the agent to display on session start.
	SystemMessage(msg string) error
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
}

// HookEdit represents a single old_string → new_string replacement applied to a file.
type HookEdit struct {
	// OldString is the text that was replaced.
	OldString string
	// NewString is the text that replaced OldString.
	NewString string
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
