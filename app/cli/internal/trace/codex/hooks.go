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

package codex

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
	hookMarker   = "chainloop trace hook codex"
	settingsFile = ".codex/hooks.json"
	// Skip tracing on Windows when the CLI is missing, before any Go handler
	// can run. The POSIX command uses trace.GuardHookCommand, like the other
	// providers.
	windowsHookPrefix = `cmd.exe /d /s /c "where chainloop >nul 2>nul & if errorlevel 1 (exit /b 0) else (`

	// Codex hook event names.
	eventSessionStart     = "SessionStart"
	eventUserPromptSubmit = "UserPromptSubmit"
	eventPreToolUse       = "PreToolUse"
	eventPostToolUse      = "PostToolUse"
	eventSessionEnd       = "SessionEnd"
)

// fileWritingTools is the single source of truth for Codex hook tool names that modify files.
var fileWritingTools = []string{"apply_patch"}

// commandTools are Codex hook tool names that run shell commands. They can create
// or modify arbitrary files, so their changes are captured via a before/after
// working-tree snapshot rather than a per-file snapshot.
var commandTools = []string{"Bash"}

// hookToolMatcher is the pipe-separated matcher for the pre/post-tool-use hooks.
// It covers both file-writing and command tools; the handler branches on which
// kind fired.
var hookToolMatcher = "^(" + strings.Join(append(append([]string{}, fileWritingTools...), commandTools...), "|") + ")$"

type hookEvent struct {
	event         string
	command       string
	matcher       string // optional: tool name matcher
	statusMessage string
}

var hookEvents = []hookEvent{
	{eventSessionStart, "chainloop trace hook codex session-start", "", "Chainloop Trace: start session"},
	{eventUserPromptSubmit, "chainloop trace hook codex user-prompt-submit", "", "Chainloop Trace: capture prompt context"},
	{eventPreToolUse, "chainloop trace hook codex pre-tool-use", hookToolMatcher, "Chainloop Trace: snapshot files before changes"},
	{eventPostToolUse, "chainloop trace hook codex post-tool-use", hookToolMatcher, "Chainloop Trace: record tool changes"},
	{eventSessionEnd, "chainloop trace hook codex session-end", "", "Chainloop Trace: finish session"},
}

// SettingsFile returns the absolute path to .codex/hooks.json for the given repo.
func (p *Provider) SettingsFile(repoRoot string) string {
	return filepath.Join(repoRoot, settingsFile)
}

// InstallHooks adds Chainloop hooks to .codex/hooks.json.
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
	if len(events) < len(hookEvents) {
		changed = removeHookEntry(hooks, eventSessionEnd)
	}
	for _, h := range events {
		if addHookEntry(hooks, h.event, h.command, h.matcher, h.statusMessage) {
			changed = true
		}
	}

	if !changed {
		return nil
	}

	settings["hooks"] = hooks

	return writeJSONFile(settingsPath, settings)
}

// UninstallHooks removes Chainloop hooks from .codex/hooks.json.
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

// HooksInstalled reports whether .codex/hooks.json holds any Chainloop
// hook. User-authored hooks are ignored.
func (p *Provider) HooksInstalled(repoRoot string) (bool, error) {
	settings, err := readJSONFile(filepath.Join(repoRoot, settingsFile))
	if err != nil {
		return false, err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	for _, h := range hookEvents {
		entries, _ := hooks[h.event].([]any)
		if slices.ContainsFunc(entries, entryContainsChainloopHook) {
			return true, nil
		}
	}

	return false, nil
}

// maxHookPayloadBytes caps hook payload reads to defend against runaway or
// malformed payloads.
const maxHookPayloadBytes = 16 * 1024 * 1024

// ReadHookInput reads and parses the Codex hook JSON from the given reader.
func (p *Provider) ReadHookInput(r io.Reader) (*trace.HookInput, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxHookPayloadBytes+1))
	if err != nil {
		return nil, err
	}

	if len(data) > maxHookPayloadBytes {
		return nil, errors.New("codex hook payload exceeds size limit")
	}
	var raw struct {
		trace.HookInput
		Model     string `json:"model"`
		ToolInput struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	input := raw.HookInput
	input.Model = raw.Model
	if p.IsCommandTool(input.ToolName) {
		input.Command = raw.ToolInput.Command
	}
	if p.IsFileWritingTool(input.ToolName) {
		input.FilePaths = patchPaths(raw.ToolInput.Command, input.Cwd)
	}
	return &input, nil
}

// patchPaths lists both sides of moves so deletions and new paths are recorded.
func patchPaths(patch, cwd string) []string {
	var paths []string
	if !filepath.IsAbs(cwd) {
		return nil
	}
	for _, line := range strings.Split(patch, "\n") {
		for _, prefix := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "} {
			path, ok := strings.CutPrefix(line, prefix)
			if !ok || path == "" {
				continue
			}
			if !filepath.IsAbs(path) {
				path = filepath.Join(cwd, path)
			}
			path = filepath.Clean(path)
			if !slices.Contains(paths, path) {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func addHookEntry(hooks map[string]any, event, command, matcher, statusMessage string) bool {
	entries, _ := hooks[event].([]any)
	for _, entry := range entries {
		group, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		handlers, _ := group["hooks"].([]any)
		existingMatcher, _ := group["matcher"].(string)
		if len(handlers) == 1 && existingMatcher == matcher {
			handler, _ := handlers[0].(map[string]any)
			if handler["command"] == trace.GuardHookCommand(command) && handler["commandWindows"] == windowsHookPrefix+command+`)"` && handler["statusMessage"] == statusMessage {
				return false
			}
		}
	}
	removeHookEntry(hooks, event)
	entries, _ = hooks[event].([]any)
	hooks[event] = append(entries, buildHookEntry(command, matcher, statusMessage))
	return true
}

func buildHookEntry(command, matcher, statusMessage string) map[string]any {
	entry := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":           "command",
				"statusMessage":  statusMessage,
				"command":        trace.GuardHookCommand(command),
				"commandWindows": windowsHookPrefix + command + `)"`,
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
	var kept []any
	changed := false
	for _, entry := range entries {
		group, ok := entry.(map[string]any)
		if !ok {
			kept = append(kept, entry)
			continue
		}
		handlers, _ := group["hooks"].([]any)
		var userHooks []any
		for _, handler := range handlers {
			if isChainloopHandler(handler) {
				changed = true
			} else {
				userHooks = append(userHooks, handler)
			}
		}
		if len(userHooks) == len(handlers) {
			kept = append(kept, entry)
		} else if len(userHooks) > 0 {
			group["hooks"] = userHooks
			kept = append(kept, group)
		}
	}
	if !changed {
		return false
	}
	if len(kept) == 0 {
		delete(hooks, event)
	} else {
		hooks[event] = kept
	}
	return true
}

// guardPrefix is the part of a guarded command that comes before the wrapped
// command; trace.GuardHookCommand closes it with a single quote.
var guardPrefix = strings.TrimSuffix(trace.GuardHookCommand(""), "'")

func isChainloopHandler(handler any) bool {
	h, ok := handler.(map[string]any)
	if !ok {
		return false
	}
	command, _ := h["command"].(string)
	if strings.HasPrefix(command, guardPrefix) && strings.HasSuffix(command, "'") {
		command = strings.TrimSuffix(strings.TrimPrefix(command, guardPrefix), "'")
	}
	return strings.HasPrefix(command, hookMarker+" ")
}

func entryContainsChainloopHook(entry any) bool {
	group, ok := entry.(map[string]any)
	if !ok {
		return false
	}
	handlers, _ := group["hooks"].([]any)
	return slices.ContainsFunc(handlers, isChainloopHandler)
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

	if hooks, ok := result["hooks"]; ok && hooks != nil {
		groups, ok := hooks.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("parse %s: hooks must be an object", path)
		}
		for event, entries := range groups {
			if _, ok := entries.([]any); !ok {
				return nil, fmt.Errorf("parse %s: %s hooks must be an array", path, event)
			}
		}
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
