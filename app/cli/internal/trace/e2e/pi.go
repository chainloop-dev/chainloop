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
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// piExtensionPath is the extension file where `trace init` installs the Pi
// hooks (spec issue-3520, D-002).
const piExtensionPath = ".pi/extensions/chainloop-trace.ts"

const (
	piSessionStartHandler          = `pi.on("session_start", async (event, ctx) => {`
	piSessionStartFallback         = `const response = starts.get(id) ?? await start(ctx, event.type)`
	piDisabledSessionStartHandler  = `pi.on("e2e.session_start", async (event, ctx) => {`
	piDisabledSessionStartFallback = `const response = starts.get(id)`
	piInstructionMarker            = "chainloop-trace-instruction:"
	piReminderType                 = "chainloop-trace-spec-reminder"
	piReminderTextMarker           = "Capture only in these cases"
	piSessionStartEvent            = "SessionStart"
	piPromptSubmitEvent            = "UserPromptSubmit"
)

var piVersionRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// Pi is the harness adapter of the Pi coding agent.
type Pi struct {
	// Bin is the pi executable. Empty means "pi" from PATH.
	Bin string
}

func (p *Pi) bin() string {
	if p.Bin != "" {
		return p.Bin
	}
	return "pi"
}

// Column implements Harness.
func (p *Pi) Column() string { return "Pi" }

// TraceFlag implements Harness.
func (p *Pi) TraceFlag() string { return "--pi" }

// Version implements Harness. `pi --version` prints "1.3.0".
func (p *Pi) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, p.bin(), "--version").Output() //nolint:gosec // the harness binary under test // nosemgrep
	if err != nil {
		return "", fmt.Errorf("pi --version: %w", err)
	}
	v := piVersionRe.FindString(string(out))
	if v == "" {
		return "", fmt.Errorf("pi --version: no version in %q", strings.TrimSpace(string(out)))
	}
	return v, nil
}

// Run implements Harness. It runs `pi -p` with the JSON output.
//
// The configuration folder of Pi and its session folder are next to the
// workspace, so the extensions, the skills and the settings of the user who
// runs the suite do not load, and the sessions of the suite do not mix with
// the sessions of the user. --approve trusts the project files of the
// workspace, which hold the Chainloop extension.
//
// The Chainloop extension gives its instruction and reminder to the model as
// hidden context, which the run output does not show. Until the suite reads
// them from the session file, the hook messages are not reported.
func (p *Pi) Run(ctx context.Context, req RunRequest) (*RunOutput, error) {
	base := filepath.Dir(req.Dir)
	sessionDir := filepath.Join(base, "pi-sessions")
	args := []string{
		"-p", "--mode", "json", "--approve",
		flagModel, req.Model,
		"--session-dir", sessionDir,
	}
	prompt := req.Prompt
	if req.Scenario == scenarioSubagent {
		extension, err := p.prepareSubagent(ctx, base)
		if err != nil {
			return nil, err
		}
		args = append(args, "--extension", extension)
		prompt = piSubagentPrompt
	}

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
			args = append(args, "@"+path)
		}
	}
	args = append(args, prompt)

	name := p.bin()
	if len(req.Wrapper) > 0 {
		name = req.Wrapper[0]
		args = append(append(append([]string{}, req.Wrapper[1:]...), p.bin()), args...)
	}

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the harness binary under test, with the prompt of a scenario // nosemgrep
	cmd.Dir = req.Dir
	cmd.Env = append(slices.Clone(req.Env),
		"PI_CODING_AGENT_DIR="+filepath.Join(base, "pi-agent"),
		"PI_SKIP_VERSION_CHECK=1",
		"PI_TELEMETRY=0",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	raw, err := cmd.Output()
	if err != nil {
		return &RunOutput{Raw: raw}, fmt.Errorf("pi -p: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	hooks, err := piHookMessages(sessionDir)
	if err != nil {
		return &RunOutput{Raw: raw}, err
	}

	return &RunOutput{Raw: raw, Hooks: hooks, HooksReported: true}, nil
}

// HookConfig implements Harness. The Chainloop extension is a file of its
// own, so the extension of the user is in another file.
func (p *Pi) HookConfig() HookConfig {
	return HookConfig{
		Path:            piExtensionPath,
		ChainloopMarker: "chainloop",
		UserPath:        ".pi/extensions/e2e-user.ts",
		UserConfig: `// e2e-user-hook
export default function () {}
`,
		UserMarker: userHookMarker,
	}
}

// DisableSessionStart implements Harness. It disables both paths that call the
// session-start hook: the Pi lifecycle event and the before-agent fallback.
func (p *Pi) DisableSessionStart(repo string) error {
	path := filepath.Join(repo, piExtensionPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	handlerCount := bytes.Count(data, []byte(piSessionStartHandler))
	fallbackCount := bytes.Count(data, []byte(piSessionStartFallback))
	if handlerCount != 1 || fallbackCount != 1 {
		return fmt.Errorf("%s has %d session-start handlers and %d fallbacks, want 1 each", piExtensionPath, handlerCount, fallbackCount)
	}
	data = bytes.Replace(data, []byte(piSessionStartHandler), []byte(piDisabledSessionStartHandler), 1)
	data = bytes.Replace(data, []byte(piSessionStartFallback), []byte(piDisabledSessionStartFallback), 1)
	return os.WriteFile(path, data, 0o600) //nolint:gosec // the extension file of a throwaway workspace
}

// SkillPath implements Harness. Pi finds project skills in .agents/skills.
func (p *Pi) SkillPath(name string) string {
	return ".agents/skills/" + name + "/SKILL.md"
}

func piHookMessages(sessionDir string) ([]HookMessage, error) {
	paths, err := filepath.Glob(filepath.Join(sessionDir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("find Pi session: %w", err)
	}
	if len(paths) != 1 {
		return nil, fmt.Errorf("find Pi session: got %d JSONL files, want 1", len(paths))
	}

	f, err := os.Open(paths[0])
	if err != nil {
		return nil, fmt.Errorf("open Pi session: %w", err)
	}
	defer f.Close()

	var out []HookMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)
	for scanner.Scan() {
		var entry struct {
			Type       string `json:"type"`
			CustomType string `json:"customType"`
			Content    string `json:"content"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return nil, fmt.Errorf("decode Pi session entry: %w", err)
		}
		if entry.Type != "custom_message" || entry.Content == "" {
			continue
		}
		switch {
		case strings.HasPrefix(entry.CustomType, piInstructionMarker):
			out = append(out, HookMessage{Event: piSessionStartEvent, ModelContext: entry.Content})
			if strings.Contains(entry.Content, piReminderTextMarker) {
				out = append(out, HookMessage{Event: piPromptSubmitEvent, ModelContext: entry.Content})
			}
		case entry.CustomType == piReminderType:
			out = append(out, HookMessage{Event: piPromptSubmitEvent, ModelContext: entry.Content})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Pi session: %w", err)
	}
	return out, nil
}
