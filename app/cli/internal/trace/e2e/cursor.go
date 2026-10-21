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
	"slices"
	"strings"
)

// cursorHooksPath is the project file where `trace init` installs the Cursor
// hooks.
const cursorHooksPath = ".cursor/hooks.json"

// cursorSessionStartEvent is the hook event of the session start in
// cursorHooksPath.
const cursorSessionStartEvent = "sessionStart"

// errCursorImages is the error of a run with pasted images. The headless
// Cursor CLI has no input for an image.
var errCursorImages = errors.New("cursor-agent: the headless CLI cannot attach a pasted image")

// Cursor is the harness adapter of the Cursor CLI (cursor-agent).
type Cursor struct {
	// Bin is the cursor-agent executable. Empty means "cursor-agent" from
	// PATH.
	Bin string
}

func (c *Cursor) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "cursor-agent"
}

// Column implements Harness.
func (c *Cursor) Column() string { return "Cursor" }

// TraceFlag implements Harness.
func (c *Cursor) TraceFlag() string { return "--cursor" }

// Version implements Harness.
func (c *Cursor) Version(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, c.bin(), "--version").Output() //nolint:gosec // the harness binary under test // nosemgrep
	if err != nil {
		return "", fmt.Errorf("cursor-agent --version: %w", err)
	}
	return parseCursorVersion(out)
}

// parseCursorVersion reads the output of `cursor-agent --version`, for example
// "2026.01.09-6d1b8f1". The version is a build tag, not a semantic version.
func parseCursorVersion(out []byte) (string, error) {
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", errors.New("cursor-agent --version: empty output")
	}
	return fields[len(fields)-1], nil
}

// Run implements Harness. It runs `cursor-agent -p` with the stream-json
// output.
//
// HOME is a folder next to the workspace, so the configuration, the user
// hooks (~/.cursor/hooks.json) and the transcripts of the user who runs the
// suite do not load or mix with the sessions of the suite. The Trace hooks
// run in the same environment, so the provider reads the transcripts from
// that folder too. The CLI then has no stored login: it reads its key from
// CURSOR_API_KEY. --force approves the tool calls and --trust trusts the
// workspace, which a headless run cannot answer.
//
// Cursor gives the session-start context to the model only, and the
// stream-json output does not show it, so the hook messages are not reported.
func (c *Cursor) Run(ctx context.Context, req RunRequest) (*RunOutput, error) {
	if len(req.Images) > 0 {
		return nil, errCursorImages
	}

	home := filepath.Join(filepath.Dir(req.Dir), "cursor-home")
	if err := writeCursorShellProfiles(home, req.Env); err != nil {
		return nil, err
	}

	args := []string{
		"-p", "--output-format", outputStreamJSON,
		"--force", "--trust",
		"--workspace", req.Dir,
		flagModel, req.Model,
		req.Prompt,
	}

	name := c.bin()
	if len(req.Wrapper) > 0 {
		name = req.Wrapper[0]
		args = append(append(append([]string{}, req.Wrapper[1:]...), c.bin()), args...)
	}

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the harness binary under test, with the prompt of a scenario // nosemgrep
	cmd.Dir = req.Dir
	cmd.Env = append(slices.Clone(req.Env), "HOME="+home)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	raw, err := cmd.Output()
	if err != nil {
		return &RunOutput{Raw: raw}, fmt.Errorf("cursor-agent -p: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if err := cursorError(raw); err != nil {
		return &RunOutput{Raw: raw}, err
	}

	return &RunOutput{Raw: raw}, nil
}

// cursorShellProfiles are the startup files of the login shells that run the
// shell tool of Cursor, relative to HOME.
var cursorShellProfiles = []string{".zprofile", ".zshrc", ".bash_profile", ".bashrc", ".profile"}

// writeCursorShellProfiles creates HOME and writes a startup file for each
// shell that sets PATH back to the PATH of env.
//
// Cursor runs its shell tool in a login shell. On macOS, /etc/zprofile runs
// path_helper, which puts the system folders first in PATH. A chainloop CLI
// in /usr/local/bin then hides the CLI under test, and the git hooks of the
// session run the wrong CLI. The startup files of HOME run after
// /etc/zprofile, so they set PATH back.
func writeCursorShellProfiles(home string, env []string) error {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}

	var path string
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			path = v
		}
	}
	if path == "" {
		return nil
	}

	// A single-quoted shell word, with each quote in the value escaped.
	profile := "export PATH='" + strings.ReplaceAll(path, "'", `'\''`) + "'\n"
	for _, name := range cursorShellProfiles {
		if err := os.WriteFile(filepath.Join(home, name), []byte(profile), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// cursorEvent is the part of a stream-json line of `cursor-agent -p` that the
// suite reads.
type cursorEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype"`
	IsError bool   `json:"is_error"`
	Result  string `json:"result"`
}

// cursorError returns the error of the result event of a run, which can come
// with exit status 0.
func cursorError(raw []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev cursorEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			return fmt.Errorf("decode cursor-agent event: %w", err)
		}
		if ev.Type == "result" && ev.IsError {
			return fmt.Errorf("cursor-agent -p: %s: %s", ev.Subtype, ev.Result)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read cursor-agent output: %w", err)
	}
	return nil
}

// HookConfig implements Harness. The hooks of the user and the Chainloop
// hooks share one file.
func (c *Cursor) HookConfig() HookConfig {
	return HookConfig{
		Path:            cursorHooksPath,
		ChainloopMarker: "chainloop trace hook cursor",
		UserConfig: `{
  "version": 1,
  "hooks": {
    "stop": [
      {"command": "echo e2e-user-hook"}
    ]
  }
}
`,
		UserMarker: userHookMarker,
	}
}

// DisableSessionStart implements Harness. It deletes the sessionStart entry
// from the hooks of the project file and keeps the other entries as they
// are.
func (c *Cursor) DisableSessionStart(repo string) error {
	path := filepath.Join(repo, cursorHooksPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("decode %s: %w", cursorHooksPath, err)
	}
	var hooks map[string]json.RawMessage
	if err := json.Unmarshal(settings["hooks"], &hooks); err != nil {
		return fmt.Errorf("decode the hooks of %s: %w", cursorHooksPath, err)
	}
	if _, ok := hooks[cursorSessionStartEvent]; !ok {
		return fmt.Errorf("%s has no %s hook", cursorHooksPath, cursorSessionStartEvent)
	}
	delete(hooks, cursorSessionStartEvent)

	if settings["hooks"], err = json.Marshal(hooks); err != nil {
		return err
	}
	if data, err = json.MarshalIndent(settings, "", "  "); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// SkillPath implements Harness. Cursor finds project skills in
// .cursor/skills.
func (c *Cursor) SkillPath(name string) string {
	return ".cursor/skills/" + name + "/SKILL.md"
}
