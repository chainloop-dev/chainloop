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

package cursor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
)

const (
	// hookMarker identifies Chainloop-installed entries inside .cursor/hooks.json.
	hookMarker = "chainloop trace hook cursor"
	// settingsFile is the project-local Cursor hooks configuration file.
	settingsFile = ".cursor/hooks.json"
	// hooksSchemaVersion is the schema version Cursor expects in hooks.json.
	hooksSchemaVersion = 1
	// defaultHookTimeoutSeconds is the per-hook timeout persisted in hooks.json.
	defaultHookTimeoutSeconds = 30

	// Cursor hook event names.
	eventSessionStart  = "sessionStart"
	eventSessionEnd    = "sessionEnd"
	eventAfterFileEdit = "afterFileEdit"
	eventPreToolUse    = "preToolUse"
	eventPostToolUse   = "postToolUse"

	// syntheticEditToolName is the synthetic tool name set on HookInput for
	// afterFileEdit events so generic handlers can treat them like
	// Claude-style file-writing tool invocations.
	syntheticEditToolName = "edit"

	// shellToolName is the Cursor tool that runs shell commands. It is also
	// the matcher of the preToolUse and postToolUse hooks, so Cursor runs
	// them for shell commands only.
	shellToolName = "Shell"
)

type cursorHookEvent struct {
	event   string
	command string
	// matcher limits a tool hook to the tools whose name it matches. Empty
	// for hooks that are not tool hooks.
	matcher string
}

// cursorHookEvents is the full list of hooks installed by InstallHooks.
var cursorHookEvents = []cursorHookEvent{
	{event: eventSessionStart, command: "chainloop trace hook cursor session-start"},
	{event: eventSessionEnd, command: "chainloop trace hook cursor session-end"},
	{event: eventAfterFileEdit, command: "chainloop trace hook cursor after-file-edit"},
	{event: eventPreToolUse, command: "chainloop trace hook cursor pre-tool-use", matcher: shellToolName},
	{event: eventPostToolUse, command: "chainloop trace hook cursor post-tool-use", matcher: shellToolName},
}

// SettingsFile returns the absolute path to .cursor/hooks.json for the given repo.
func (p *Provider) SettingsFile(repoRoot string) string {
	return filepath.Join(repoRoot, settingsFile)
}

// InstallHooks adds Chainloop hooks to .cursor/hooks.json.
func (p *Provider) InstallHooks(repoRoot string) error {
	return p.installHookEvents(repoRoot, cursorHookEvents)
}

// InstallHooksForTraceRun installs every hook except SessionEnd; trace
// run drives end-of-session itself.
func (p *Provider) InstallHooksForTraceRun(repoRoot string) error {
	events := make([]cursorHookEvent, 0, len(cursorHookEvents))
	for _, h := range cursorHookEvents {
		if h.event == eventSessionEnd {
			continue
		}
		events = append(events, h)
	}

	return p.installHookEvents(repoRoot, events)
}

func (p *Provider) installHookEvents(repoRoot string, events []cursorHookEvent) error {
	settingsPath := filepath.Join(repoRoot, settingsFile)

	settings, err := readJSONFile(settingsPath)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = make(map[string]any)
	}

	changed := false
	for _, h := range events {
		if addHookEntry(hooks, h) {
			changed = true
		}
	}

	if version, _ := settings["version"].(float64); int(version) != hooksSchemaVersion {
		settings["version"] = hooksSchemaVersion
		changed = true
	}

	if !changed {
		return nil
	}

	settings["hooks"] = hooks

	return writeJSONFile(settingsPath, settings)
}

// UninstallHooks removes Chainloop hooks from .cursor/hooks.json.
// Leaves user-authored hooks intact.
func (p *Provider) UninstallHooks(repoRoot string) error {
	settingsPath := filepath.Join(repoRoot, settingsFile)

	settings, err := readJSONFile(settingsPath)
	if err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		return nil
	}

	changed := false
	for _, h := range cursorHookEvents {
		if removeHookEntry(hooks, h.event) {
			changed = true
		}
	}

	if !changed {
		return nil
	}

	if len(hooks) == 0 {
		delete(settings, "hooks")
		// version is meaningless without any hooks; drop it too
		delete(settings, "version")
	} else {
		settings["hooks"] = hooks
	}

	if len(settings) == 0 {
		err := os.Remove(settingsPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	return writeJSONFile(settingsPath, settings)
}

// HooksInstalled reports whether .cursor/hooks.json holds any Chainloop hook.
// User-authored hooks are ignored.
func (p *Provider) HooksInstalled(repoRoot string) (bool, error) {
	settings, err := readJSONFile(filepath.Join(repoRoot, settingsFile))
	if err != nil {
		return false, err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	for _, h := range cursorHookEvents {
		entries, _ := hooks[h.event].([]any)
		if slices.ContainsFunc(entries, entryContainsChainloopHook) {
			return true, nil
		}
	}

	return false, nil
}

// cursorHookInput is the wire-level structure of a Cursor hook JSON payload.
// Only the fields we consume are modelled; unknown fields are ignored.
type cursorHookInput struct {
	ConversationID string           `json:"conversation_id"`
	SessionID      string           `json:"session_id"`
	HookEventName  string           `json:"hook_event_name"`
	CursorVersion  string           `json:"cursor_version"`
	Model          string           `json:"model"`
	WorkspaceRoots []string         `json:"workspace_roots"`
	FilePath       string           `json:"file_path"`
	Edits          []cursorHookEdit `json:"edits"`
	// Cwd, ToolName, ToolUseID and ToolInput are set on preToolUse and
	// postToolUse only.
	Cwd       string `json:"cwd"`
	ToolName  string `json:"tool_name"`
	ToolUseID string `json:"tool_use_id"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type cursorHookEdit struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// maxHookPayloadBytes caps hook payload reads to defend against runaway or
// malformed payloads (afterFileEdit edits can embed arbitrary file content).
const maxHookPayloadBytes = 16 * 1024 * 1024

// ReadHookInput parses a Cursor hook JSON payload from the given reader.
// ConversationID is preferred as the session identifier; session_id is used
// as a fallback for forward-compatibility.
func (p *Provider) ReadHookInput(r io.Reader) (*trace.HookInput, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxHookPayloadBytes))
	if err != nil {
		return nil, err
	}

	var raw cursorHookInput
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	sessionID := raw.ConversationID
	if sessionID == "" {
		sessionID = raw.SessionID
	}

	input := &trace.HookInput{
		SessionID:     sessionID,
		HookEventName: raw.HookEventName,
		ToolName:      raw.ToolName,
		ToolUseID:     raw.ToolUseID,
		FilePath:      raw.FilePath,
		Command:       raw.ToolInput.Command,
		Cwd:           raw.Cwd,
		AgentVersion:  raw.CursorVersion,
		Model:         raw.Model,
	}

	// Cursor files transcripts under the workspace it was opened on, which
	// is not the shell's cwd when a command runs in a subdirectory. A
	// multi-root workspace has no single answer; the first root is the one
	// Cursor lists first and the best guess available.
	if len(raw.WorkspaceRoots) > 0 {
		input.Cwd = raw.WorkspaceRoots[0]
	}

	if raw.HookEventName == eventAfterFileEdit {
		input.ToolName = syntheticEditToolName
	}

	if len(raw.Edits) > 0 {
		input.Edits = make([]trace.HookEdit, len(raw.Edits))
		for i, e := range raw.Edits {
			input.Edits[i] = trace.HookEdit{OldString: e.OldString, NewString: e.NewString}
		}
	}

	return input, nil
}

func addHookEntry(hooks map[string]any, h cursorHookEvent) bool {
	entries, _ := hooks[h.event].([]any)

	for i, entry := range entries {
		if !entryContainsChainloopHook(entry) {
			continue
		}
		m, _ := entry.(map[string]any)
		existingCmd, _ := m["command"].(string)
		existingMatcher, _ := m["matcher"].(string)
		if existingCmd == trace.GuardHookCommand(h.command) && existingMatcher == h.matcher {
			return false
		}
		entries[i] = buildHookEntry(h)
		hooks[h.event] = entries

		return true
	}

	hooks[h.event] = append(entries, buildHookEntry(h))

	return true
}

func removeHookEntry(hooks map[string]any, event string) bool {
	entries, _ := hooks[event].([]any)
	if len(entries) == 0 {
		return false
	}

	var kept []any
	for _, entry := range entries {
		if !entryContainsChainloopHook(entry) {
			kept = append(kept, entry)
		}
	}

	if len(kept) == len(entries) {
		return false
	}

	if len(kept) == 0 {
		delete(hooks, event)
	} else {
		hooks[event] = kept
	}

	return true
}

func buildHookEntry(h cursorHookEvent) map[string]any {
	entry := map[string]any{
		"command": trace.GuardHookCommand(h.command),
		"timeout": defaultHookTimeoutSeconds,
	}
	if h.matcher != "" {
		entry["matcher"] = h.matcher
	}

	return entry
}

func entryContainsChainloopHook(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}

	cmd, _ := m["command"].(string)

	return strings.Contains(cmd, hookMarker)
}

func readJSONFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return make(map[string]any), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}

func writeJSONFile(path string, data map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	content = append(content, '\n')

	return os.WriteFile(path, content, 0600)
}
