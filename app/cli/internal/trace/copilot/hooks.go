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

package copilot

import (
	"bytes"
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
	hookMarker = "chainloop trace hook copilot"
	// settingsFile is the hook file that Trace owns. Copilot CLI and the
	// Copilot agent in VS Code read every JSON file in .github/hooks, so
	// Trace writes its own file instead of merging into a file of the user.
	settingsFile = ".github/hooks/chainloop-trace.json"

	// Copilot hook event names, in the camelCase form of the hook file.
	eventSessionStart = "sessionStart"
	eventPreToolUse   = "preToolUse"
	eventPostToolUse  = "postToolUse"
	eventSessionEnd   = "sessionEnd"

	// maxHookPayloadBytes caps hook payload reads.
	maxHookPayloadBytes = 16 * 1024 * 1024
)

// fileWritingTools is the single source of truth for Copilot tools that write
// one file with a known path.
//
// create and edit are the file tools of Claude models. apply_patch is the
// file tool of GPT models: its argument is the patch text, which can change
// several files.
var fileWritingTools = []string{"create", "edit", toolApplyPatch}

// toolApplyPatch is the Copilot tool that applies a patch.
const toolApplyPatch = "apply_patch"

// commandTools are the Copilot tools that run a shell command. Their changes
// are captured with a before and after working-tree snapshot.
var commandTools = []string{"bash", "powershell"}

// hookEvents lists the installed hooks. There is no agentStop hook: it fires
// at the end of each turn, and the session-end handler would end the session
// after the first turn. The push copies the transcript again anyway. There is
// no userPromptSubmitted hook either: Copilot drops its output.
var hookEvents = []struct {
	event, command string
}{
	{eventSessionStart, hookMarker + " session-start"},
	{eventPreToolUse, hookMarker + " pre-tool-use"},
	{eventPostToolUse, hookMarker + " post-tool-use"},
	{eventSessionEnd, hookMarker + " session-end"},
}

// hookFile is the format of a Copilot hook file.
type hookFile struct {
	Version int                    `json:"version"`
	Hooks   map[string][]hookEntry `json:"hooks"`
}

type hookEntry struct {
	Type       string `json:"type"`
	Bash       string `json:"bash,omitempty"`
	PowerShell string `json:"powershell,omitempty"`
	Command    string `json:"command,omitempty"`
}

// guardPrefix is the part of a guarded command that comes before the wrapped
// command. trace.GuardHookCommand closes it with a single quote.
var guardPrefix = strings.TrimSuffix(trace.GuardHookCommand(""), "'")

// isChainloopEntry reports whether a hook entry runs a Chainloop Trace hook,
// with or without the missing-CLI guard.
func isChainloopEntry(e hookEntry) bool {
	command := e.Bash
	if command == "" {
		command = e.Command
	}
	if strings.HasPrefix(command, guardPrefix) && strings.HasSuffix(command, "'") {
		command = strings.TrimSuffix(strings.TrimPrefix(command, guardPrefix), "'")
	}
	return strings.HasPrefix(command, hookMarker+" ")
}

// SettingsFile returns the absolute path of the hook file that Trace owns.
func (p *Provider) SettingsFile(repoRoot string) string {
	return filepath.Join(repoRoot, settingsFile)
}

// InstallHooks writes the Trace hook file.
func (p *Provider) InstallHooks(repoRoot string) error {
	return p.writeHookFile(repoRoot, true, false)
}

// InstallHooksForTraceRun writes the hook file without session end. trace run
// ends the session itself, and it restores the previous file after the run,
// so it can replace a file that Trace does not own.
func (p *Provider) InstallHooksForTraceRun(repoRoot string) error {
	return p.writeHookFile(repoRoot, false, true)
}

func renderHookFile(includeSessionEnd bool) []byte {
	f := hookFile{Version: 1, Hooks: map[string][]hookEntry{}}
	for _, h := range hookEvents {
		if h.event == eventSessionEnd && !includeSessionEnd {
			continue
		}
		f.Hooks[h.event] = []hookEntry{{Type: "command", Bash: trace.GuardHookCommand(h.command)}}
	}
	content, _ := json.MarshalIndent(f, "", "  ")
	return append(content, '\n')
}

func (p *Provider) writeHookFile(repoRoot string, includeSessionEnd, allowUnowned bool) error {
	path := p.SettingsFile(repoRoot)
	content := renderHookFile(includeSessionEnd)

	existing, owned, err := readOwnedHookFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil && !allowUnowned:
		return fmt.Errorf("install Copilot hooks: %w", err)
	case err == nil && bytes.Equal(existing, content):
		return nil
	case err == nil && !owned && !allowUnowned:
		return fmt.Errorf("install Copilot hooks: %s holds hooks that Chainloop did not write", settingsFile)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("install Copilot hooks: create directory: %w", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("install Copilot hooks: write file: %w", err)
	}
	return nil
}

// readOwnedHookFile reads the hook file and tells whether every hook in it
// is a Chainloop hook. A missing file returns an error that wraps
// os.ErrNotExist.
func readOwnedHookFile(path string) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%s is not a regular file", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	var f hookFile
	if err := json.Unmarshal(content, &f); err != nil {
		return content, false, fmt.Errorf("parse %s: %w", path, err)
	}
	total, ours := 0, 0
	for _, entries := range f.Hooks {
		for _, e := range entries {
			total++
			if isChainloopEntry(e) {
				ours++
			}
		}
	}
	return content, total > 0 && total == ours, nil
}

// UninstallHooks deletes the hook file, but only when every hook in it is a
// Chainloop hook. Other hook files are never touched.
func (p *Provider) UninstallHooks(repoRoot string) error {
	path := p.SettingsFile(repoRoot)
	_, owned, err := readOwnedHookFile(path)
	if errors.Is(err, os.ErrNotExist) || err != nil || !owned {
		// A file that cannot be read or holds user hooks stays as it is.
		return nil
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("uninstall Copilot hooks: %w", err)
	}
	return nil
}

// HooksInstalled reports whether the hook file holds a Chainloop hook.
func (p *Provider) HooksInstalled(repoRoot string) (bool, error) {
	content, err := os.ReadFile(p.SettingsFile(repoRoot))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var f hookFile
	if json.Unmarshal(content, &f) != nil {
		return false, nil
	}
	for _, entries := range f.Hooks {
		if slices.ContainsFunc(entries, isChainloopEntry) {
			return true, nil
		}
	}
	return false, nil
}

// hookPayload is the union of the two payload forms that the Copilot agent
// sends: camelCase keys from the hook file, and snake_case keys in the form
// that VS Code compatible hooks get.
type hookPayload struct {
	SessionID          string          `json:"sessionId"`
	SessionIDSnake     string          `json:"session_id"`
	HookEventName      string          `json:"hook_event_name"`
	HookEventNameCamel string          `json:"hookEventName"`
	Cwd                string          `json:"cwd"`
	TranscriptPath     string          `json:"transcriptPath"`
	TranscriptPathSnk  string          `json:"transcript_path"`
	ToolName           string          `json:"toolName"`
	ToolNameSnake      string          `json:"tool_name"`
	ToolArgs           json.RawMessage `json:"toolArgs"`
	ToolInput          json.RawMessage `json:"tool_input"`
	// ToolCalls is the pre-tool form of older Copilot CLI releases: one
	// hook for a batch of calls.
	ToolCalls []struct {
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	} `json:"toolCalls"`
}

// toolArgs are the arguments of the file and shell tools that Trace reads.
type toolArgs struct {
	Path    string `json:"path"`
	Command string `json:"command"`
}

// decodeToolArgs reads tool arguments that come as a JSON object, or as a
// JSON string that holds the object.
func decodeToolArgs(raw json.RawMessage) toolArgs {
	var args toolArgs
	if len(raw) == 0 {
		return args
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		raw = json.RawMessage(text)
	}
	_ = json.Unmarshal(raw, &args)
	return args
}

// ReadHookInput reads a Copilot hook payload in either form.
//
// The pre-tool payload of a batch of calls cannot pair one call with its
// post-tool payload, so the input never carries a tool call ID, also when
// the post-tool payload has one. The pre and post hooks of a shell tool then
// share the snapshot key of the session (TRC-006).
func (p *Provider) ReadHookInput(r io.Reader) (*trace.HookInput, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxHookPayloadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxHookPayloadBytes {
		return nil, errors.New("copilot hook payload exceeds size limit")
	}

	var raw hookPayload
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	input := &trace.HookInput{
		SessionID:      firstNonEmpty(raw.SessionID, raw.SessionIDSnake),
		HookEventName:  firstNonEmpty(raw.HookEventName, raw.HookEventNameCamel),
		Cwd:            raw.Cwd,
		TranscriptPath: firstNonEmpty(raw.TranscriptPath, raw.TranscriptPathSnk),
		ToolName:       firstNonEmpty(raw.ToolName, raw.ToolNameSnake),
	}

	if input.ToolName != "" {
		args := raw.ToolArgs
		if len(args) == 0 {
			args = raw.ToolInput
		}
		p.setToolTarget(input, args)
		return input, nil
	}

	if len(raw.ToolCalls) > 0 {
		p.readToolCalls(input, raw)
	}
	return input, nil
}

func (p *Provider) setToolTarget(input *trace.HookInput, args json.RawMessage) {
	switch {
	case p.IsFileWritingTool(input.ToolName):
		setPaths(input, toolPaths(input.ToolName, args, input.Cwd))
	case p.IsCommandTool(input.ToolName):
		input.Command = decodeToolArgs(args).Command
	}
}

// setPaths sets FilePath for one path, or FilePaths for several, so the
// handlers snapshot each file of a tool that changes several files.
func setPaths(input *trace.HookInput, paths []string) {
	switch len(paths) {
	case 0:
	case 1:
		input.FilePath = paths[0]
	default:
		input.FilePaths = paths
	}
}

// toolPaths returns the absolute paths that a file tool writes.
func toolPaths(tool string, args json.RawMessage, cwd string) []string {
	if tool == toolApplyPatch {
		return patchPaths(patchText(args), cwd)
	}
	if path := absPath(decodeToolArgs(args).Path, cwd); path != "" {
		return []string{path}
	}
	return nil
}

// patchText returns the patch of an apply_patch call: a JSON string, or an
// object with the patch in its input field.
func patchText(args json.RawMessage) string {
	var text string
	if json.Unmarshal(args, &text) == nil {
		return text
	}
	var object struct {
		Input string `json:"input"`
	}
	_ = json.Unmarshal(args, &object)
	return object.Input
}

// patchPaths lists the files of a patch, with both sides of a move, so that
// a deletion and a new path are recorded too. It reads the same patch format
// as the Codex provider.
func patchPaths(patch, cwd string) []string {
	var paths []string
	for _, line := range strings.Split(patch, "\n") {
		for _, prefix := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "} {
			path, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), prefix)
			if !ok {
				continue
			}
			if path = absPath(strings.TrimSpace(path), cwd); path != "" && !slices.Contains(paths, path) {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

// readToolCalls maps a batch of calls to one input. File tools win over a
// shell tool, because their per-file snapshots are exact. Each later
// post-tool hook carries one call.
func (p *Provider) readToolCalls(input *trace.HookInput, raw hookPayload) {
	var paths []string
	for _, call := range raw.ToolCalls {
		if !p.IsFileWritingTool(call.Name) {
			continue
		}
		if input.ToolName == "" {
			input.ToolName = call.Name
		}
		for _, path := range toolPaths(call.Name, call.Args, input.Cwd) {
			if !slices.Contains(paths, path) {
				paths = append(paths, path)
			}
		}
	}
	if input.ToolName != "" {
		setPaths(input, paths)
		return
	}

	for _, call := range raw.ToolCalls {
		if p.IsCommandTool(call.Name) {
			input.ToolName = call.Name
			input.Command = decodeToolArgs(call.Args).Command
			return
		}
	}
}

func absPath(path, cwd string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		if !filepath.IsAbs(cwd) {
			return ""
		}
		path = filepath.Join(cwd, path)
	}
	return filepath.Clean(path)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
