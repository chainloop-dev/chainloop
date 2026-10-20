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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	// codexHooksPath is the project file where Codex reads its hooks. Trace
	// has no Codex provider yet, so no Chainloop hook is in it.
	codexHooksPath = ".codex/hooks.json"
	// codexOpenRouterPrefix selects the OpenRouter provider for a model of
	// the suite config.
	codexOpenRouterPrefix = "openrouter/"
	// codexOpenRouterProvider defines the OpenRouter provider, which reads
	// its key from OPENROUTER_API_KEY.
	codexOpenRouterProvider = `model_providers.openrouter={name="OpenRouter",base_url="https://openrouter.ai/api/v1",env_key="OPENROUTER_API_KEY"}`
)

var codexVersionRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// Codex is the harness adapter of the OpenAI Codex CLI.
type Codex struct {
	// Bin is the codex executable. Empty means "codex" from PATH.
	Bin string
}

func (c *Codex) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "codex"
}

// Column implements Harness.
func (c *Codex) Column() string { return "Codex" }

// TraceFlag implements Harness.
func (c *Codex) TraceFlag() string { return "--codex" }

// Version implements Harness.
func (c *Codex) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, c.bin(), "--version").Output() //nolint:gosec // the harness binary under test // nosemgrep
	if err != nil {
		return "", fmt.Errorf("codex --version: %w", err)
	}
	return parseCodexVersion(out)
}

// parseCodexVersion reads the output of `codex --version`, for example
// "codex-cli 0.163.0".
func parseCodexVersion(out []byte) (string, error) {
	v := codexVersionRe.FindString(string(out))
	if v == "" {
		return "", fmt.Errorf("codex --version: no version in %q", strings.TrimSpace(string(out)))
	}
	return v, nil
}

// codexModelArgs returns the arguments that select the model. A model with
// the "openrouter/" prefix runs on OpenRouter. Any other model runs on the
// default provider of Codex, which reads its key from CODEX_API_KEY.
func codexModelArgs(model string) []string {
	name, ok := strings.CutPrefix(model, codexOpenRouterPrefix)
	if !ok {
		return []string{flagModel, model}
	}
	return []string{
		flagModel, name,
		"-c", "model_provider=openrouter",
		"-c", codexOpenRouterProvider,
	}
}

// Run implements Harness. It runs `codex exec` with the JSON output.
//
// The configuration folder of Codex (CODEX_HOME) is next to the workspace, so
// the configuration, the hooks, the skills and the sessions of the user who
// runs the suite do not load or mix with the sessions of the suite. The
// sandbox and the approvals are off, because the workspace is a throwaway
// repository. The project hooks run with no persisted trust.
// --ignore-user-config is not set, because it also turns off the project
// hooks.
//
// Trace has no Codex provider yet, so no hook message is reported.
func (c *Codex) Run(ctx context.Context, req RunRequest) (*RunOutput, error) {
	base := filepath.Dir(req.Dir)
	codexHome := filepath.Join(base, "codex-home")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		return nil, err
	}

	args := []string{
		"exec", "--json",
		"--dangerously-bypass-approvals-and-sandbox",
		"--dangerously-bypass-hook-trust",
		"-C", req.Dir,
	}
	args = append(args, codexModelArgs(req.Model)...)

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
			args = append(args, "--image", path)
		}
	}
	// The prompt follows "--", so that the list of images ends before it.
	args = append(args, "--", req.Prompt)

	name := c.bin()
	if len(req.Wrapper) > 0 {
		name = req.Wrapper[0]
		args = append(append(append([]string{}, req.Wrapper[1:]...), c.bin()), args...)
	}

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the harness binary under test, with the prompt of a scenario // nosemgrep
	cmd.Dir = req.Dir
	cmd.Env = append(slices.Clone(req.Env), "CODEX_HOME="+codexHome)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	raw, err := cmd.Output()
	if err != nil {
		return &RunOutput{Raw: raw}, fmt.Errorf("codex exec: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return &RunOutput{Raw: raw}, nil
}

// HookConfig implements Harness. The hooks of the user and the Chainloop
// hooks share one file.
func (c *Codex) HookConfig() HookConfig {
	return HookConfig{
		Path:            codexHooksPath,
		ChainloopMarker: "chainloop trace hook codex",
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

// errCodexSessionStart is the error of DisableSessionStart until Trace has a
// Codex provider.
var errCodexSessionStart = errors.New("codex: the suite cannot remove the session-start hook of Trace, because Trace has no Codex provider yet")

// DisableSessionStart implements Harness. Trace installs no Codex hooks yet,
// so there is no hook to remove.
func (c *Codex) DisableSessionStart(string) error {
	return errCodexSessionStart
}

// SkillPath implements Harness. Codex finds project skills in
// .agents/skills.
func (c *Codex) SkillPath(name string) string {
	return ".agents/skills/" + name + "/SKILL.md"
}
