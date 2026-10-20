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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// openCodePluginPath is the plugin file where `trace init` installs the
// OpenCode hooks.
const openCodePluginPath = ".opencode/plugins/chainloop-trace.ts"

// openCodeReminderMarker is a text that only the spec capture reminder holds,
// not the session-start instruction. The plugin posts both to the session as
// synthetic messages, and this text tells them apart.
const openCodeReminderMarker = "Capture only in these cases"

// openCodeSessionStartHandler is the line of the plugin that runs the
// session-start hook. DisableSessionStart replaces it.
const openCodeSessionStartHandler = `if (type === "session.created") {`

var openCodeVersionRe = regexp.MustCompile(`(\d+)\.\d+\.\d+`)

// OpenCode is the harness adapter of OpenCode. One adapter serves one major
// version, which is one column of the feature table.
type OpenCode struct {
	// Major is the major version of OpenCode that the column covers.
	Major string
	// Bin is the opencode executable. Empty means "opencode" from PATH.
	Bin string
}

func (o *OpenCode) bin() string {
	if o.Bin != "" {
		return o.Bin
	}
	return "opencode"
}

// Column implements Harness.
func (o *OpenCode) Column() string { return "OpenCode " + o.Major }

// TraceFlag implements Harness.
func (o *OpenCode) TraceFlag() string { return "--opencode" }

// Version implements Harness. `opencode --version` prints "opencode v2.0.22".
// An installed major version that is not the one of the column is an error,
// because the cells would test the wrong column.
func (o *OpenCode) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, o.bin(), "--version").Output() //nolint:gosec // the harness binary under test
	if err != nil {
		return "", fmt.Errorf("opencode --version: %w", err)
	}
	m := openCodeVersionRe.FindStringSubmatch(string(out))
	if m == nil {
		return "", fmt.Errorf("opencode --version: no version in %q", strings.TrimSpace(string(out)))
	}
	if m[1] != o.Major {
		return "", fmt.Errorf("installed opencode %s is not the major version %s of the column %q", m[0], o.Major, o.Column())
	}
	return m[0], nil
}

// Run implements Harness. It runs `opencode run` with the JSON output. Each
// session gets a private server (--standalone): the shared background server
// keeps the environment and the plugins of the directory that started it.
// --auto approves the tool calls, which a headless run cannot answer.
//
// The plugin posts the instruction and the reminder of Trace to the session
// as synthetic messages, which the run output does not show. Run exports the
// session after the run and reads them from the export.
func (o *OpenCode) Run(ctx context.Context, req RunRequest) (*RunOutput, error) {
	args := []string{"run", "--standalone", "--format", "json", "--auto", flagModel, req.Model}

	// A pasted image goes in as an attached file, from outside the
	// repository, so that the agent cannot commit it.
	if len(req.Images) > 0 {
		dir, err := os.MkdirTemp("", "chainloop-trace-e2e-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(dir)
		for i, img := range req.Images {
			path := filepath.Join(dir, fmt.Sprintf("pasted-%d.png", i+1))
			if err := os.WriteFile(path, img, 0o600); err != nil {
				return nil, err
			}
			args = append(args, "--file", path)
		}
	}
	args = append(args, req.Prompt)

	name := o.bin()
	if len(req.Wrapper) > 0 {
		name = req.Wrapper[0]
		args = append(append(append([]string{}, req.Wrapper[1:]...), o.bin()), args...)
	}

	raw, err := o.command(ctx, req, name, args...)
	if err != nil {
		return &RunOutput{Raw: raw}, fmt.Errorf("opencode run: %w", err)
	}
	if err := openCodeError(raw); err != nil {
		return &RunOutput{Raw: raw}, err
	}

	sessionID, err := openCodeSessionID(raw)
	if err != nil {
		return &RunOutput{Raw: raw}, err
	}
	export, err := o.command(ctx, req, o.bin(), "session", "export", sessionID)
	if err != nil {
		return &RunOutput{Raw: raw}, fmt.Errorf("opencode session export: %w", err)
	}

	// The export follows the run events in the harness output, on one line.
	var line bytes.Buffer
	if err := json.Compact(&line, export); err != nil {
		return &RunOutput{Raw: raw}, fmt.Errorf("decode opencode export: %w", err)
	}
	raw = append(append(raw, line.Bytes()...), '\n')

	hooks, err := parseOpenCodeHookMessages(export)
	if err != nil {
		return &RunOutput{Raw: raw}, err
	}

	return &RunOutput{Raw: raw, Hooks: hooks, HooksReported: true}, nil
}

func (o *OpenCode) command(ctx context.Context, req RunRequest, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the harness binary under test, with the prompt of a scenario // nosemgrep
	cmd.Dir = req.Dir
	// OpenCode takes its project directory from PWD, not from the working
	// directory of the process. With the PWD of the suite, the agent works
	// and pushes in the checkout that runs the suite.
	cmd.Env = append(slices.Clone(req.Env), "PWD="+req.Dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// openCodeEvent is the part of a JSON event of `opencode run` that the suite
// reads.
type openCodeEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID"`
	Error     *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func openCodeEvents(raw []byte) ([]openCodeEvent, error) {
	var out []openCodeEvent
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev openCodeEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return nil, fmt.Errorf("decode opencode event: %w", err)
		}
		out = append(out, ev)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read opencode output: %w", err)
	}
	return out, nil
}

// openCodeError returns the error event of a run. `opencode run` exits with
// status 0 for some errors, for example a provider that does not accept
// the credentials.
func openCodeError(raw []byte) error {
	events, err := openCodeEvents(raw)
	if err != nil {
		return err
	}
	for _, ev := range events {
		if ev.Type == "error" && ev.Error != nil {
			return fmt.Errorf("opencode run: %s: %s", ev.Error.Type, ev.Error.Message)
		}
	}
	return nil
}

// openCodeSessionID returns the session of a run, from its first event.
func openCodeSessionID(raw []byte) (string, error) {
	events, err := openCodeEvents(raw)
	if err != nil {
		return "", err
	}
	for _, ev := range events {
		if ev.SessionID != "" {
			return ev.SessionID, nil
		}
	}
	return "", errors.New("opencode run: no session in the output")
}

// openCodeExport is the part of an OpenCode 2 session export that the suite
// reads.
type openCodeExport struct {
	Messages []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"messages"`
}

// parseOpenCodeHookMessages reads the synthetic messages of a session export.
// The plugin posts them for the model only, so each one is model context.
func parseOpenCodeHookMessages(export []byte) ([]HookMessage, error) {
	var ex openCodeExport
	if err := json.Unmarshal(export, &ex); err != nil {
		return nil, fmt.Errorf("decode opencode export: %w", err)
	}

	var out []HookMessage
	for _, m := range ex.Messages {
		if m.Type != "synthetic" || m.Text == "" {
			continue
		}
		event := "SessionStart"
		if strings.Contains(m.Text, openCodeReminderMarker) {
			event = "UserPromptSubmit"
		}
		out = append(out, HookMessage{Event: event, ModelContext: m.Text})
	}
	return out, nil
}

// HookConfig implements Harness. The Chainloop plugin is a file of its own,
// so the plugin of the user is in another file.
func (o *OpenCode) HookConfig() HookConfig {
	return HookConfig{
		Path:            openCodePluginPath,
		ChainloopMarker: `"trace", "hook", "opencode"`,
		UserPath:        ".opencode/plugins/e2e-user.ts",
		UserConfig: `// e2e-user-hook
export default {
  id: "e2e-user",
  async setup() {},
}
`,
		UserMarker: userHookMarker,
	}
}

// DisableSessionStart implements Harness. It changes the session.created
// handler of the plugin, so that the session-start hook never runs.
func (o *OpenCode) DisableSessionStart(repo string) error {
	path := filepath.Join(repo, openCodePluginPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if n := bytes.Count(data, []byte(openCodeSessionStartHandler)); n != 1 {
		return fmt.Errorf("%s has %d session.created handlers, want 1", openCodePluginPath, n)
	}
	data = bytes.Replace(data, []byte(openCodeSessionStartHandler), []byte(`if (type === "e2e.disabled") {`), 1)
	return os.WriteFile(path, data, 0o600) //nolint:gosec // the plugin file of a throwaway workspace
}

// SkillPath implements Harness.
func (o *OpenCode) SkillPath(name string) string {
	return ".opencode/skills/" + name + "/SKILL.md"
}
