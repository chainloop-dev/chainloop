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

// piExtensionPath is the extension file where `trace init` installs the Pi
// hooks (spec issue-3520, D-002).
const piExtensionPath = ".pi/extensions/chainloop-trace.ts"

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

// Version implements Harness. `pi --version` prints "0.79.6".
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
	args := []string{
		"-p", "--mode", "json", "--approve",
		flagModel, req.Model,
		"--session-dir", filepath.Join(base, "pi-sessions"),
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
	args = append(args, req.Prompt)

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

	return &RunOutput{Raw: raw}, nil
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

// errPiSessionStart is the error of DisableSessionStart until the suite
// knows the shape of the Chainloop extension.
var errPiSessionStart = errors.New("pi: the suite cannot remove the session-start hook of the Chainloop extension yet")

// DisableSessionStart implements Harness. The Chainloop extension of Pi does
// not exist yet, so there is no handler to change.
func (p *Pi) DisableSessionStart(string) error {
	return errPiSessionStart
}

// SkillPath implements Harness. Pi finds project skills in .agents/skills.
func (p *Pi) SkillPath(name string) string {
	return ".agents/skills/" + name + "/SKILL.md"
}
