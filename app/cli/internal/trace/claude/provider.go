package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
)

// Name is the provider identifier emitted to evidence and used as the key
// in SessionRecord.Provider. Kept as an exported constant so CLI wiring
// can refer to it without instantiating the provider.
const Name = "claude-code"

// Compile-time check that Provider implements trace.Provider.
var _ trace.Provider = (*Provider)(nil)

// Provider implements trace.Provider for Claude Code sessions.
type Provider struct{}

// New creates a new Claude Code provider.
func New() *Provider {
	return &Provider{}
}

// Name returns the agent identifier.
func (p *Provider) Name() string {
	return Name
}

// DiscoverSession finds the most relevant Claude Code session for the given repo root.
func (p *Provider) DiscoverSession(repoRoot string) (*trace.DiscoveredSession, error) {
	session, err := discoverClaudeSession(repoRoot)
	if err != nil || session == nil {
		return nil, err
	}

	return &trace.DiscoveredSession{
		SessionID:  session.sessionID,
		SessionDir: filepath.Dir(session.jsonlPath),
		IsActive:   session.isActive,
	}, nil
}

// SessionDirForRepo returns the Claude Code project directory for a given repo root.
func (p *Provider) SessionDirForRepo(repoRoot string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".claude", "projects", encodeCWDForClaudePath(repoRoot))
}

// CopySessionData copies the Claude Code JSONL (and subagent files) from the
// Claude project directory into the store's raw/ directory so pre-push can
// parse them even if Claude rotates its own storage later.
func (p *Provider) CopySessionData(store *state.Store, repoRoot, sessionID string) error {
	sourceDir := p.SessionDirForRepo(repoRoot)
	if sourceDir == "" {
		return nil
	}

	return store.CopySessionJSONL(sessionID, sourceDir)
}

// CaptureFileSnapshot reads the file at input.FilePath and stores its
// content under the store's snapshots/ directory so the post-tool-use
// handler can compute line-range diffs after the edit. A missing file
// (e.g. Claude's Write creating a new file) is treated as a non-error;
// other read errors (permission denied, I/O failures) are surfaced.
func (p *Provider) CaptureFileSnapshot(store *state.Store, input *trace.HookInput) error {
	if input == nil || input.FilePath == "" {
		return nil
	}

	content, err := os.ReadFile(input.FilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("read %q: %w", input.FilePath, err)
	}

	return store.SaveFileSnapshot(input.SessionID, input.FilePath, content)
}

// ResolveBeforeContent returns the snapshot saved by CaptureFileSnapshot.
// The "after" argument is unused for Claude — we always rely on the pre-edit
// snapshot — but is part of the interface for providers that reconstruct
// from edits instead of snapshots.
func (p *Provider) ResolveBeforeContent(store *state.Store, input *trace.HookInput, _ []byte) []byte {
	if input == nil || input.FilePath == "" {
		return nil
	}

	snap, err := store.LoadFileSnapshot(input.SessionID, input.FilePath)
	if err != nil {
		return nil
	}

	return snap
}

// CleanupAfterEdit removes the per-edit snapshot. Safe to call when no
// snapshot was ever taken.
func (p *Provider) CleanupAfterEdit(store *state.Store, input *trace.HookInput) {
	if input == nil || input.FilePath == "" {
		return
	}

	store.DeleteFileSnapshot(input.SessionID, input.FilePath)
}

// IsFileWritingTool returns true if the named tool modifies files on disk.
func (p *Provider) IsFileWritingTool(toolName string) bool {
	return slices.Contains(fileWritingTools, toolName)
}

// IsCommandTool returns true if the named tool runs a shell command.
func (p *Provider) IsCommandTool(toolName string) bool {
	return slices.Contains(commandTools, toolName)
}

// SystemMessage writes a message to stdout for Claude Code to display on session start.
func (p *Provider) SystemMessage(msg string) error {
	if msg == "" {
		return nil
	}

	resp := struct {
		SystemMessage string `json:"systemMessage"`
	}{SystemMessage: msg}

	return json.NewEncoder(os.Stdout).Encode(resp)
}

// ParseSession parses a Claude Code session JSONL and returns structured evidence.
func (p *Provider) ParseSession(_ context.Context, opts *trace.ParseOpts) (*aicodingsession.Evidence, error) {
	jsonlPath, err := findJSONLPath(opts.SessionDir, opts.SessionID)
	if err != nil {
		return nil, err
	}

	data, rawMain, err := parseJSONL(jsonlPath)
	if err != nil {
		return nil, err
	}

	// The filename carries only the sanitized ID, so prefer the caller's.
	if data.sessionID == "" {
		data.sessionID = opts.SessionID
	}
	if data.sessionID == "" {
		data.sessionID = strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")
	}

	warnings := data.warnings
	subagents, subUsage, rawSubagents, err := processSubagents(opts.SessionDir, data.sessionID, &warnings)
	if err != nil {
		return nil, err
	}

	merged := mergeUsage(data.tokenUsageByModel, subUsage)
	cost := computeCost(merged, &warnings)

	rawSession := map[string][]json.RawMessage{
		"main": rawMain,
	}
	if rawMain == nil {
		rawSession["main"] = []json.RawMessage{}
	}
	for k, v := range rawSubagents {
		if k == "main" {
			warnings = append(warnings, "skipping subagent raw session with reserved key 'main'")
			continue
		}
		rawSession[k] = v
	}

	return buildOutput(data, subagents, merged, warnings, cost, rawSession), nil
}
