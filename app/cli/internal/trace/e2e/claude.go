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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// claudeTools are the tools that a scenario session may use with no
// permission prompt. The session runs in a throwaway repository, and the list
// keeps the session from asking, which a headless run cannot answer.
var claudeTools = []string{"Read", "Write", "Edit", "Glob", "Grep", "Bash", "Skill", "Task", "Agent"}

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
		flagModel, req.Model,
		"--output-format", "stream-json",
		"--verbose",
		"--include-hook-events",
		"--setting-sources", "project,local",
		"--allowedTools", strings.Join(claudeTools, ","),
	}

	// A pasted image goes in a stream-json user message on stdin, the same
	// content block that an interactive paste makes.
	var stdin []byte
	if len(req.Images) > 0 {
		msg, err := claudeUserMessage(req.Prompt, req.Images)
		if err != nil {
			return nil, err
		}
		stdin = msg
		// The prompt is in the message, so -p takes no argument.
		args = append([]string{"-p", "--input-format", "stream-json"}, args[2:]...)
	}

	name := c.bin()
	if len(req.Wrapper) > 0 {
		name = req.Wrapper[0]
		args = append(append(append([]string{}, req.Wrapper[1:]...), c.bin()), args...)
	}

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the harness binary under test, with the prompt of a scenario // nosemgrep
	cmd.Dir = req.Dir
	cmd.Env = req.Env
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
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

// claudeContent is one content block of a stream-json user message.
type claudeContent struct {
	Type   string             `json:"type"`
	Text   string             `json:"text,omitempty"`
	Source *claudeImageSource `json:"source,omitempty"`
}

type claudeImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// claudeUserMessage builds the stream-json user message of a prompt with
// pasted PNG images.
func claudeUserMessage(prompt string, images [][]byte) ([]byte, error) {
	content := make([]claudeContent, 0, len(images)+1)
	for _, img := range images {
		content = append(content, claudeContent{Type: "image", Source: &claudeImageSource{
			Type: "base64", MediaType: "image/png", Data: base64.StdEncoding.EncodeToString(img),
		}})
	}
	content = append(content, claudeContent{Type: "text", Text: prompt})

	msg := struct {
		Type    string `json:"type"`
		Message struct {
			Role    string          `json:"role"`
			Content []claudeContent `json:"content"`
		} `json:"message"`
	}{Type: "user"}
	msg.Message.Role = "user"
	msg.Message.Content = content

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encode claude user message: %w", err)
	}
	return append(data, '\n'), nil
}

// claudeSettingsPath is the project settings file where `trace init` installs
// the Claude Code hooks.
const claudeSettingsPath = ".claude/settings.json"

// HookConfig implements Harness.
func (c *Claude) HookConfig() HookConfig {
	return HookConfig{
		Path:            claudeSettingsPath,
		ChainloopMarker: "chainloop trace hook claude",
		UserConfig: `{
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "echo e2e-user-hook"}]}
    ]
  }
}
`,
		UserMarker: userHookMarker,
	}
}

// DisableSessionStart implements Harness. It deletes the SessionStart entry
// from the hooks of the project settings and keeps the other entries as
// they are.
func (c *Claude) DisableSessionStart(repo string) error {
	path := filepath.Join(repo, claudeSettingsPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("decode %s: %w", claudeSettingsPath, err)
	}
	var hooks map[string]json.RawMessage
	if err := json.Unmarshal(settings["hooks"], &hooks); err != nil {
		return fmt.Errorf("decode the hooks of %s: %w", claudeSettingsPath, err)
	}
	if _, ok := hooks["SessionStart"]; !ok {
		return fmt.Errorf("%s has no SessionStart hook", claudeSettingsPath)
	}
	delete(hooks, "SessionStart")

	if settings["hooks"], err = json.Marshal(hooks); err != nil {
		return err
	}
	if data, err = json.MarshalIndent(settings, "", "  "); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// SkillPath implements Harness.
func (c *Claude) SkillPath(name string) string {
	return ".claude/skills/" + name + "/SKILL.md"
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
