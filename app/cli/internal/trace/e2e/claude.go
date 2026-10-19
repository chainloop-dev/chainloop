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

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// claudeTools are the tools that a scenario session may use with no
// permission prompt. The session runs in a throwaway repository, and the list
// keeps the session from asking, which a headless run cannot answer.
var claudeTools = []string{"Read", "Write", "Edit", "Glob", "Grep", "Bash", "Skill"}

// Claude is the harness adapter of Claude Code.
type Claude struct {
	// Bin is the claude executable. Empty means "claude" from PATH.
	Bin string
}

func (c *Claude) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "claude"
}

// Column implements Harness.
func (c *Claude) Column() string { return HarnessClaudeCode }

// TraceFlag implements Harness.
func (c *Claude) TraceFlag() string { return "--claude" }

// Version implements Harness. `claude --version` prints "2.1.0 (Claude Code)".
func (c *Claude) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, c.bin(), "--version").Output() //nolint:gosec // the harness binary under test
	if err != nil {
		return "", fmt.Errorf("claude --version: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", fmt.Errorf("claude --version: empty output")
	}
	return fields[0], nil
}

// Run implements Harness. It runs `claude -p` with the stream-json output and
// the hook events, so the output shows the hook messages that the user sees.
//
// Only the project and local settings are loaded. The settings and the plugins
// of the user who runs the suite would add their own hooks and change the
// session.
func (c *Claude) Run(ctx context.Context, req RunRequest) (*RunOutput, error) {
	args := []string{
		"-p", req.Prompt,
		"--model", req.Model,
		"--output-format", "stream-json",
		"--verbose",
		"--include-hook-events",
		"--setting-sources", "project,local",
		"--allowedTools", strings.Join(claudeTools, ","),
	}

	cmd := exec.CommandContext(ctx, c.bin(), args...) //nolint:gosec // the harness binary under test, with the prompt of a scenario
	cmd.Dir = req.Dir
	cmd.Env = req.Env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	raw, err := cmd.Output()
	if err != nil {
		return &RunOutput{Raw: raw}, fmt.Errorf("claude -p: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	hooks, err := parseClaudeHookMessages(raw)
	if err != nil {
		return &RunOutput{Raw: raw}, err
	}

	return &RunOutput{Raw: raw, Hooks: hooks, HooksReported: true}, nil
}

// claudeStreamEvent is the part of a stream-json line that the suite reads. A
// hook event has type "system" and subtype "hook_response". Its stdout is
// the response that the hook printed.
type claudeStreamEvent struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	HookEvent string `json:"hook_event"`
	Stdout    string `json:"stdout"`
}

// claudeHookResponse is the response that a Trace hook prints for Claude Code.
type claudeHookResponse struct {
	SystemMessage      string `json:"systemMessage"`
	HookSpecificOutput struct {
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

// parseClaudeHookMessages reads the hook responses from the stream-json
// output. A hook that printed no JSON, for example a tool hook, gives no
// message.
func parseClaudeHookMessages(raw []byte) ([]HookMessage, error) {
	var out []HookMessage

	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var ev claudeStreamEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("decode claude stream event: %w", err)
		}
		if ev.Type != "system" || ev.Subtype != "hook_response" || strings.TrimSpace(ev.Stdout) == "" {
			continue
		}

		var resp claudeHookResponse
		if err := json.Unmarshal([]byte(ev.Stdout), &resp); err != nil {
			// Not a JSON response: the hook printed plain text, which Claude
			// Code does not show to the user.
			continue
		}
		out = append(out, HookMessage{
			Event:        ev.HookEvent,
			UserMessage:  resp.SystemMessage,
			ModelContext: resp.HookSpecificOutput.AdditionalContext,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read claude output: %w", err)
	}

	return out, nil
}
