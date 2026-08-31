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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
)

const (
	hookMarker   = "chainloop trace hook claude"
	settingsFile = ".claude/settings.json"

	// Claude Code hook event names.
	eventSessionStart = "SessionStart"
	eventPreToolUse   = "PreToolUse"
	eventPostToolUse  = "PostToolUse"
	eventSessionEnd   = "SessionEnd"
)

// fileWritingTools is the single source of truth for Claude tool names that modify files.
var fileWritingTools = []string{"Edit", "Write", "MultiEdit"}

// commandTools are Claude tool names that run shell commands. They can create
// or modify arbitrary files, so their changes are captured via a before/after
// working-tree snapshot rather than a per-file snapshot.
var commandTools = []string{"Bash"}

// hookToolMatcher is the pipe-separated matcher for the pre/post-tool-use hooks.
// It covers both file-writing and command tools; the handler branches on which
// kind fired.
var hookToolMatcher = strings.Join(append(append([]string{}, fileWritingTools...), commandTools...), "|")

type hookEvent struct {
	event   string
	command string
	matcher string // optional: tool name matcher
}

var hookEvents = []hookEvent{
	{eventSessionStart, "chainloop trace hook claude session-start", ""},
	{eventPreToolUse, "chainloop trace hook claude pre-tool-use", hookToolMatcher},
	{eventPostToolUse, "chainloop trace hook claude post-tool-use", hookToolMatcher},
	{eventSessionEnd, "chainloop trace hook claude session-end", ""},
}

// SettingsFile returns the absolute path to .claude/settings.json for the given repo.
func (p *Provider) SettingsFile(repoRoot string) string {
	return filepath.Join(repoRoot, settingsFile)
}

// InstallHooks adds Chainloop hooks to .claude/settings.json.
func (p *Provider) InstallHooks(repoRoot string) error {
	return p.installHookEvents(repoRoot, hookEvents)
}

// InstallHooksForTraceRun installs every hook except SessionEnd; trace
// run drives end-of-session itself.
func (p *Provider) InstallHooksForTraceRun(repoRoot string) error {
	events := make([]hookEvent, 0, len(hookEvents))
	for _, h := range hookEvents {
		if h.event == eventSessionEnd {
			continue
		}
		events = append(events, h)
	}

	return p.installHookEvents(repoRoot, events)
}

func (p *Provider) installHookEvents(repoRoot string, events []hookEvent) error {
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
		if addHookEntry(hooks, h.event, h.command, h.matcher) {
			changed = true
		}
	}

	if !changed {
		return nil
	}

	settings["hooks"] = hooks

	return writeJSONFile(settingsPath, settings)
}

// UninstallHooks removes Chainloop hooks from .claude/settings.json.
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
	for _, h := range hookEvents {
		if removeHookEntry(hooks, h.event) {
			changed = true
		}
	}

	if !changed {
		return nil
	}

	// Clean up empty hooks map
	if len(hooks) == 0 {
		delete(settings, "hooks")
	}

	// Delete file if empty
	if len(settings) == 0 {
		return os.Remove(settingsPath)
	}

	return writeJSONFile(settingsPath, settings)
}

// maxHookPayloadBytes caps hook payload reads to defend against runaway or
// malformed payloads.
const maxHookPayloadBytes = 16 * 1024 * 1024

// ReadHookInput reads and parses the Claude Code hook JSON from the given reader.
func (p *Provider) ReadHookInput(r io.Reader) (*trace.HookInput, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxHookPayloadBytes))
	if err != nil {
		return nil, err
	}

	// Single unmarshal into a combined struct to avoid double-parsing.
	// CursorVersion is read alongside the standard fields so we can detect
	// Cursor-relayed events without a second pass.
	var raw struct {
		trace.HookInput
		ToolInput struct {
			FilePath string `json:"file_path"`
		} `json:"tool_input"`
		CursorVersion string `json:"cursor_version"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// When Cursor is the runtime relaying the event (both .claude/settings.json
	// and .cursor/hooks.json are installed), every payload — including the
	// preToolUse/postToolUse it forwards through Claude's hook config — carries
	// cursor_version. Native Claude Code never injects that field. Returning
	// an empty input makes the Claude handler bail out so the cursor provider's
	// afterFileEdit owns the session and recording is not duplicated.
	if raw.CursorVersion != "" {
		return &trace.HookInput{}, nil
	}

	input := raw.HookInput
	if raw.ToolInput.FilePath != "" {
		input.FilePath = raw.ToolInput.FilePath
	}

	return &input, nil
}

func addHookEntry(hooks map[string]any, event, command, matcher string) bool {
	entries, _ := hooks[event].([]any)

	// Check if a chainloop hook already exists for this event
	for i, entry := range entries {
		if !entryContainsChainloopHook(entry) {
			continue
		}
		// Chainloop entry exists — check if it needs updating (e.g., matcher changed)
		m, _ := entry.(map[string]any)
		existingMatcher, _ := m["matcher"].(string)
		if existingMatcher == matcher {
			return false // already up to date
		}
		// Replace the entry with updated version
		entries[i] = buildHookEntry(command, matcher)
		hooks[event] = entries
		return true
	}

	// No existing chainloop entry — add new one
	hooks[event] = append(entries, buildHookEntry(command, matcher))
	return true
}

func buildHookEntry(command, matcher string) map[string]any {
	entry := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	}
	if matcher != "" {
		entry["matcher"] = matcher
	}
	return entry
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

func entryContainsChainloopHook(entry any) bool {
	m, ok := entry.(map[string]any)
	if !ok {
		return false
	}

	innerHooks, _ := m["hooks"].([]any)
	for _, h := range innerHooks {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}

		cmd, _ := hm["command"].(string)
		if strings.Contains(cmd, hookMarker) {
			return true
		}
	}

	return false
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

	// A file containing `null` unmarshals successfully into a nil map, which
	// panics on assignment.
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
