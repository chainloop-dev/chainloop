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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// copilotHooksPath is the hook file that `trace init --copilot` writes.
const copilotHooksPath = ".github/hooks/chainloop-trace.json"

// copilotSessionStartEvent is the hook event of the session start in
// copilotHooksPath.
const copilotSessionStartEvent = "sessionStart"

// errCopilotImages is the error of a run with pasted images. The headless
// Copilot CLI has no input for an image.
var errCopilotImages = errors.New("copilot: the headless CLI cannot attach a pasted image")

// Copilot is the harness adapter of GitHub Copilot CLI. The Copilot agent in
// VS Code reads the same hook file, but it has no headless mode, so the
// suite runs the CLI only.
type Copilot struct {
	// Bin is the copilot executable. Empty means "copilot" from PATH.
	Bin string
}

func (c *Copilot) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "copilot"
}

// Column implements Harness.
func (c *Copilot) Column() string { return "Copilot" }

// TraceFlag implements Harness.
func (c *Copilot) TraceFlag() string { return "--copilot" }

// Version implements Harness.
func (c *Copilot) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, c.bin(), "--version").Output() //nolint:gosec // the harness binary under test // nosemgrep
	if err != nil {
		return "", fmt.Errorf("copilot --version: %w", err)
	}
	return parseCopilotVersion(out)
}

// parseCopilotVersion reads the output of `copilot --version`: the first word
// that starts with a digit, for example "1.0.82" in "GitHub Copilot CLI
// 1.0.82.".
func parseCopilotVersion(out []byte) (string, error) {
	for _, field := range strings.Fields(string(out)) {
		field = strings.TrimSuffix(strings.TrimPrefix(field, "v"), ".")
		if field != "" && field[0] >= '0' && field[0] <= '9' {
			return field, nil
		}
	}
	return "", errors.New("copilot --version: no version in the output")
}

// Run implements Harness. It runs `copilot -p` and approves all tools,
// because a headless run cannot answer a permission question.
//
// COPILOT_HOME is a folder next to the workspace, so the configuration, the
// user hooks and the sessions of the user who runs the suite do not load or
// mix with the sessions of the suite. The Trace hooks run in the same
// environment, so the provider reads the session log from that folder too.
// The CLI then has no stored login: it reads a token from COPILOT_GITHUB_TOKEN,
// GH_TOKEN or GITHUB_TOKEN.
//
// The headless output does not show the hook responses, so the hook messages
// are not reported.
func (c *Copilot) Run(ctx context.Context, req RunRequest) (*RunOutput, error) {
	if len(req.Images) > 0 {
		return nil, errCopilotImages
	}

	home := filepath.Join(filepath.Dir(req.Dir), "copilot-home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return nil, err
	}

	args := []string{
		"-p", req.Prompt,
		"--allow-all-tools",
		"--no-ask-user",
		flagModel, req.Model,
	}

	name := c.bin()
	if len(req.Wrapper) > 0 {
		name = req.Wrapper[0]
		args = append(append(append([]string{}, req.Wrapper[1:]...), c.bin()), args...)
	}

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the harness binary under test, with the prompt of a scenario // nosemgrep
	cmd.Dir = req.Dir
	cmd.Env = append(slices.Clone(req.Env), "COPILOT_HOME="+home)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	raw, err := cmd.Output()
	if err != nil {
		return &RunOutput{Raw: raw}, fmt.Errorf("copilot -p: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return &RunOutput{Raw: raw}, nil
}

// HookConfig implements Harness. Copilot reads every JSON file in
// .github/hooks, so the hook of the user is in its own file.
func (c *Copilot) HookConfig() HookConfig {
	return HookConfig{
		Path:            copilotHooksPath,
		ChainloopMarker: "chainloop trace hook copilot",
		UserPath:        ".github/hooks/e2e-user.json",
		UserConfig: `{
  "version": 1,
  "hooks": {
    "sessionEnd": [
      {"type": "command", "bash": "echo e2e-user-hook"}
    ]
  }
}
`,
		UserMarker: userHookMarker,
	}
}

// DisableSessionStart implements Harness. It deletes the sessionStart entry
// from the Trace hook file and keeps the other entries as they are.
func (c *Copilot) DisableSessionStart(repo string) error {
	path := filepath.Join(repo, copilotHooksPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("decode %s: %w", copilotHooksPath, err)
	}
	var hooks map[string]json.RawMessage
	if err := json.Unmarshal(settings["hooks"], &hooks); err != nil {
		return fmt.Errorf("decode the hooks of %s: %w", copilotHooksPath, err)
	}
	if _, ok := hooks[copilotSessionStartEvent]; !ok {
		return fmt.Errorf("%s has no %s hook", copilotHooksPath, copilotSessionStartEvent)
	}
	delete(hooks, copilotSessionStartEvent)

	if settings["hooks"], err = json.Marshal(hooks); err != nil {
		return err
	}
	if data, err = json.MarshalIndent(settings, "", "  "); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// SkillPath implements Harness. Copilot finds project skills in
// .github/skills.
func (c *Copilot) SkillPath(name string) string {
	return ".github/skills/" + name + "/SKILL.md"
}
