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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstalledHooksWithoutCLI(t *testing.T) {
	shell, err := exec.LookPath("sh")
	require.NoError(t, err)
	bin := t.TempDir()
	require.NoError(t, os.Symlink(shell, filepath.Join(bin, "sh")))
	root := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(root))
	settings, err := readJSONFile(p.SettingsFile(root))
	require.NoError(t, err)

	for _, event := range hookEvents {
		t.Run(event.event, func(t *testing.T) {
			command := installedHookCommand(t, settings, event.event)
			cmd := exec.CommandContext(t.Context(), shell, "-c", command) //nolint:gosec // execute the installed hook to verify its shell behavior
			cmd.Env = append(os.Environ(), "PATH="+bin)
			cmd.Stdin = strings.NewReader(`{"session_id":"test"}`)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, string(out))
			assert.Empty(t, out)
		})
	}
}

func TestInstalledHookPreservesIOAndExitStatus(t *testing.T) {
	shell, err := exec.LookPath("sh")
	require.NoError(t, err)
	bin := t.TempDir()
	require.NoError(t, os.Symlink(shell, filepath.Join(bin, "sh")))
	root := t.TempDir()
	p := New()
	require.NoError(t, p.InstallHooks(root))
	settings, err := readJSONFile(p.SettingsFile(root))
	require.NoError(t, err)
	command := installedHookCommand(t, settings, eventPostToolUse)
	stub := "#!/bin/sh\nIFS= read -r payload\nprintf '%s\\n%s\\n' \"$*\" \"$payload\"\nexit \"$HOOK_TEST_EXIT_CODE\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "chainloop"), []byte(stub), 0700)) //nolint:gosec // executable stub CLI in a temporary test directory

	for _, code := range []string{"0", "37"} {
		t.Run("exit_"+code, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), shell, "-c", command) //nolint:gosec // execute the installed hook against a local stub CLI
			cmd.Env = append(os.Environ(), "PATH="+bin, "HOOK_TEST_EXIT_CODE="+code)
			cmd.Stdin = strings.NewReader("{\"session_id\":\"test\"}\n")
			out, err := cmd.Output()
			if code == "0" {
				require.NoError(t, err)
			} else {
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				assert.Equal(t, 37, exitErr.ExitCode())
			}
			assert.Equal(t, "trace hook codex post-tool-use\n{\"session_id\":\"test\"}\n", string(out))
		})
	}
}

func TestInstallUpgradesUnguardedHooks(t *testing.T) {
	root := t.TempDir()
	p := New()
	path := p.SettingsFile(root)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	legacy := `{"hooks":{"PostToolUse":[{"matcher":"^(apply_patch|Bash)$","hooks":[{"type":"command","command":"chainloop trace hook codex post-tool-use"},{"type":"command","command":"echo user-hook"}]}]}}`
	require.NoError(t, os.WriteFile(path, []byte(legacy), 0600))
	require.NoError(t, p.InstallHooks(root))
	first, err := os.ReadFile(p.SettingsFile(root))
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(first), "chainloop trace hook codex post-tool-use"))
	assert.Contains(t, string(first), "command -v chainloop")
	assert.Contains(t, string(first), "echo user-hook")
	require.NoError(t, p.InstallHooks(root))
	second, err := os.ReadFile(p.SettingsFile(root))
	require.NoError(t, err)
	assert.Equal(t, first, second)
	require.NoError(t, p.UninstallHooks(root))
	remaining, err := os.ReadFile(p.SettingsFile(root))
	require.NoError(t, err)
	assert.NotContains(t, string(remaining), hookMarker)
	assert.Contains(t, string(remaining), "echo user-hook")
}

func installedHookCommand(t *testing.T, settings map[string]any, event string) string {
	t.Helper()
	hooks := settings["hooks"].(map[string]any)
	groups := hooks[event].([]any)
	require.Len(t, groups, 1)
	handlers := groups[0].(map[string]any)["hooks"].([]any)
	require.Len(t, handlers, 1)
	return handlers[0].(map[string]any)["command"].(string)
}
