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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
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

// claudeProjectsDir returns ~/.claude/projects, or "" when the home
// directory is unknown.
func claudeProjectsDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".claude", "projects")
}

// CopySessionData copies the Claude Code JSONL (and subagent files) from the
// Claude project directory into the store's raw/ directory so pre-push can
// parse them even if Claude rotates its own storage later.
//
// Claude files the transcript under the project directory of the directory
// the session started in, and reports that file as transcript_path. The hook
// cwd can be somewhere else, e.g. for a subagent in its own git worktree, so
// the reported path wins and the cwd encoding is only a fallback for records
// that lack it.
func (p *Provider) CopySessionData(store *state.Store, loc trace.SessionLocation) error {
	projectsDir := claudeProjectsDir()
	if projectsDir == "" {
		return nil
	}

	sourceDir, err := transcriptSourceDir(projectsDir, loc.TranscriptPath, loc.SessionID)
	if err != nil {
		sourceDir = filepath.Join(projectsDir, encodeCWDForClaudePath(loc.Cwd))
	}

	return store.CopySessionJSONL(loc.SessionID, sourceDir)
}

// transcriptSourceDir returns the Claude project directory that holds the
// transcripts of sessionID, given the transcript_path Claude reported. That
// is either the main transcript, <dir>/<sessionID>.jsonl, or a subagent
// transcript, <dir>/<sessionID>/subagents/<agent>.jsonl.
//
// The path comes from a hook payload, so it is checked before anything is
// read from it: it must belong to sessionID and stay inside a project
// directory under projectsDir.
func transcriptSourceDir(projectsDir, transcriptPath, sessionID string) (string, error) {
	if !state.ValidSessionID(sessionID) {
		return "", fmt.Errorf("session ID %q cannot name a transcript file", sessionID)
	}
	if !filepath.IsAbs(transcriptPath) {
		return "", fmt.Errorf("transcript path %q is not absolute", transcriptPath)
	}

	clean := filepath.Clean(transcriptPath)
	parent := filepath.Dir(clean)

	var dir string
	switch {
	case filepath.Base(clean) == sessionID+".jsonl":
		dir = parent
	case filepath.Base(parent) == "subagents" && filepath.Base(filepath.Dir(parent)) == sessionID:
		dir = filepath.Dir(filepath.Dir(parent))
	default:
		return "", fmt.Errorf("transcript path %q does not belong to session %q", transcriptPath, sessionID)
	}

	// The directory must be a project directory: a direct child of projectsDir.
	if filepath.Dir(dir) != filepath.Clean(projectsDir) {
		return "", fmt.Errorf("transcript path %q is outside %q", transcriptPath, projectsDir)
	}

	return dir, nil
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

	return store.SaveFileSnapshot(fileSnapshotKey(input), content)
}

// ResolveBeforeContent returns the snapshot saved by CaptureFileSnapshot.
// The "after" argument is unused for Claude — we always rely on the pre-edit
// snapshot — but is part of the interface for providers that reconstruct
// from edits instead of snapshots.
func (p *Provider) ResolveBeforeContent(store *state.Store, input *trace.HookInput, _ []byte) []byte {
	if input == nil || input.FilePath == "" {
		return nil
	}

	snap, err := store.LoadFileSnapshot(fileSnapshotKey(input))
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

	store.DeleteFileSnapshot(fileSnapshotKey(input))
}

func fileSnapshotKey(input *trace.HookInput) state.FileSnapshotKey {
	return state.FileSnapshotKey{SessionID: input.SessionID, FilePath: input.FilePath, ToolUseID: input.ToolUseID}
}

// IsFileWritingTool returns true if the named tool modifies files on disk.
func (p *Provider) IsFileWritingTool(toolName string) bool {
	return slices.Contains(fileWritingTools, toolName)
}

// IsCommandTool returns true if the named tool runs a shell command.
func (p *Provider) IsCommandTool(toolName string) bool {
	return slices.Contains(commandTools, toolName)
}

// SupportsSessionStartBanner is true for Claude Code: it renders the
// systemMessage field of a hook response directly to the user.
func (p *Provider) SupportsSessionStartBanner() bool {
	return true
}

// SupportsSessionStartInstruction is true for Claude Code: it feeds the
// additionalContext field of a SessionStart hook response to the model.
func (p *Provider) SupportsSessionStartInstruction() bool {
	return true
}

// AnnounceSessionStart emits a SessionStart hook response carrying both of
// Claude Code's delivery channels in one document: systemMessage, which the
// client prints to the user without involving the model, and additionalContext,
// which the model reads and acts on.
//
// systemMessage must stay top-level; nested inside hookSpecificOutput it is
// silently ignored. The event name is the one that fired, not the one
// AnnounceToUser hardcodes for its own, different hook.
func (p *Provider) AnnounceSessionStart(msg trace.SessionStartMessage) error {
	if msg.Empty() {
		return nil
	}

	type hookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext,omitempty"`
	}

	resp := struct {
		SystemMessage      string             `json:"systemMessage,omitempty"`
		HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
	}{
		SystemMessage: msg.Banner,
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:     eventSessionStart,
			AdditionalContext: msg.Instruction,
		},
	}

	return json.NewEncoder(os.Stdout).Encode(resp)
}

// SupportsPromptReminder is true for Claude Code: it adds the
// additionalContext field of a UserPromptSubmit hook response to the context
// of the turn.
func (p *Provider) SupportsPromptReminder() bool {
	return true
}

// AnnouncePromptSubmit emits a UserPromptSubmit hook response that carries
// the reminder in additionalContext only. It has no systemMessage: a banner
// at every turn would only distract the user.
func (p *Provider) AnnouncePromptSubmit(reminder string) error {
	if reminder == "" {
		return nil
	}

	type hookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	}

	resp := struct {
		HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
	}{
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:     eventUserPromptSubmit,
			AdditionalContext: reminder,
		},
	}

	return json.NewEncoder(os.Stdout).Encode(resp)
}

// AnnounceToUser emits a PostToolUse hook response on both of Claude Code's
// delivery channels: systemMessage, which the client prints to the user
// without involving the model, and additionalContext, which reaches the model
// so it can repeat the message in its own reply.
//
// Both are used because only the second is confirmed to render in every
// build. Should systemMessage prove universally reliable, dropping
// additionalContext here would spare the model a turn, and this is the one
// place that would have to change.
func (p *Provider) AnnounceToUser(msg string) error {
	if msg == "" {
		return nil
	}

	type hookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext,omitempty"`
	}

	resp := struct {
		SystemMessage      string             `json:"systemMessage"`
		HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
	}{
		SystemMessage: msg,
		HookSpecificOutput: hookSpecificOutput{
			HookEventName:     eventPostToolUse,
			AdditionalContext: "Tell the user the following, including any link verbatim: " + msg,
		},
	}

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
