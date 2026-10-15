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

package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", dir)
	require.NoError(t, cmd.Run())
}

// runTraceInit executes `trace init` with the given args in dir, with no control
// plane reachable. It returns the RunE error.
func runTraceInit(t *testing.T, dir string, args ...string) error {
	t.Helper()
	t.Chdir(dir)

	cmd := newTraceInitCmd()
	cmd.Flags().String("org", "", "organization name")
	cmd.SetArgs(args)

	return cmd.Execute()
}

// TestTraceInitLocal covers R-008/R-011: `trace init --local` sets the
// repository up for local storage with no control plane, writing traceStorage:
// LOCAL and no control-plane identity.
func TestTraceInitLocal(t *testing.T) {
	t.Run("writes LOCAL storage and no identity, installs hooks", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)

		require.NoError(t, runTraceInit(t, repoDir, "--local", "--claude"))

		data, err := os.ReadFile(filepath.Join(repoDir, chainloopYMLName))
		require.NoError(t, err)
		got := string(data)
		assert.Contains(t, got, "traceStorage: LOCAL")
		assert.NotContains(t, got, "organization:")
		assert.NotContains(t, got, "workflowName:")
		assert.NotContains(t, got, "projectName:")

		// The harness hooks were installed, so a session gets recorded.
		_, err = os.Stat(filepath.Join(repoDir, claudeSettings))
		assert.NoError(t, err)
	})

	t.Run("--trace-path implies local and is written", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)

		require.NoError(t, runTraceInit(t, repoDir, "--trace-path", "evidence/sessions", "--claude"))

		data, err := os.ReadFile(filepath.Join(repoDir, chainloopYMLName))
		require.NoError(t, err)
		got := string(data)
		assert.Contains(t, got, "traceStorage: LOCAL")
		assert.Contains(t, got, "tracePath: evidence/sessions")
	})

	t.Run("--require-trace is allowed and written as traceRequired", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)

		require.NoError(t, runTraceInit(t, repoDir, "--local", "--require-trace", "--claude"))

		data, err := os.ReadFile(filepath.Join(repoDir, chainloopYMLName))
		require.NoError(t, err)
		got := string(data)
		assert.Contains(t, got, "traceStorage: LOCAL")
		assert.Contains(t, got, "traceRequired: true")
	})

	t.Run("control-plane flags are rejected with --local", func(t *testing.T) {
		for _, flag := range [][]string{
			{"--contract", "my-contract"},
			{"--workflow", "my-workflow"},
			{"--project", "my-project"}, //nolint:goconst // test fixture value
		} {
			repoDir := t.TempDir()
			gitInit(t, repoDir)
			args := append([]string{"--local", "--claude"}, flag...)
			err := runTraceInit(t, repoDir, args...)
			assert.ErrorContains(t, err, "cannot use", "flag %v", flag)
		}
	})
}

// TestTraceInitInvalidStorage covers the guard that a control-plane init over an
// invalid traceStorage fails before installing hooks, so init never reports
// success for a repository whose every pre-push would then error, while an
// explicit --local run overrides the bad value.
func TestTraceInitInvalidStorage(t *testing.T) {
	t.Run("control-plane init over an invalid traceStorage errors before install", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, chainloopYMLName),
			[]byte("traceStorage: bogus\n"), 0600))

		err := runTraceInit(t, repoDir, "--claude")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "traceStorage")

		// Nothing was written: the harness settings were not installed.
		_, statErr := os.Stat(filepath.Join(repoDir, claudeSettings))
		assert.True(t, os.IsNotExist(statErr), "hooks must not be installed over a broken config")
	})

	t.Run("--local overrides an invalid traceStorage", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, chainloopYMLName),
			[]byte("traceStorage: bogus\n"), 0600))

		require.NoError(t, runTraceInit(t, repoDir, "--local", "--claude"))

		data, err := os.ReadFile(filepath.Join(repoDir, chainloopYMLName))
		require.NoError(t, err)
		assert.Contains(t, string(data), "traceStorage: LOCAL")
	})
}

func TestWriteTraceStorageSwitchedNote(t *testing.T) {
	out := &bytes.Buffer{}
	writeTraceStorageSwitchedNote(out, ".chainloop.yml")

	got := out.String()
	assert.Contains(t, got, "was in local mode")
	assert.Contains(t, got, "control")
	assert.Contains(t, got, ".chainloop.yml")
}

func TestWriteTraceInitLocalSummary(t *testing.T) {
	out := &bytes.Buffer{}
	writeTraceInitLocalSummary(out, t.TempDir(), []string{providerClaudeCode, "cursor"}) //nolint:goconst // provider wire name, kept literal like the other provider tests

	got := out.String()
	assert.Contains(t, got, "local mode")
	assert.Contains(t, got, "no control plane")
	assert.Contains(t, got, "claude-code, cursor")
	assert.NotContains(t, got, "organization")
}

func TestWriteTraceLocalNextSteps(t *testing.T) {
	repoRoot := t.TempDir()
	out := &bytes.Buffer{}
	writeTraceLocalNextSteps(out, repoRoot, repoRoot, providers.ByNames([]string{providerClaudeCode}))

	got := out.String()
	assert.Contains(t, got, chainloopYMLName)
	assert.Contains(t, got, claudeSettings)
	assert.Contains(t, got, "writes the session evidence to disk")
	assert.NotContains(t, got, "the push tells you where")
	assert.Contains(t, got, traceDocsURL)
	// In a clone, the first traced session installs the git hooks, so the
	// steps must not tell a teammate to run init again.
	assert.NotContains(t, got, "chainloop trace init")
	assert.NotContains(t, got, "not cloned")
}

func TestTraceStorageToPersist(t *testing.T) {
	testCases := []struct {
		name         string
		local        bool
		existingMode string
		wantValue    string
		wantSave     bool
	}{
		{
			name:      "local init writes LOCAL",
			local:     true,
			wantValue: config.TraceStorageLocal,
			wantSave:  true,
		},
		{
			name:         "local init writes LOCAL even over an existing control plane",
			local:        true,
			existingMode: config.TraceStorageControlPlane,
			wantValue:    config.TraceStorageLocal,
			wantSave:     true,
		},
		{
			// A control-plane re-init must clear a stale LOCAL so the pre-push
			// hook stops storing on disk.
			name:         "control-plane init over LOCAL clears it to CONTROL_PLANE",
			local:        false,
			existingMode: config.TraceStorageLocal,
			wantValue:    config.TraceStorageControlPlane,
			wantSave:     true,
		},
		{
			name:         "fresh control-plane init writes nothing",
			local:        false,
			existingMode: config.TraceStorageControlPlane,
			wantSave:     false,
		},
		{
			name:     "control-plane init with no existing mode writes nothing",
			local:    false,
			wantSave: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			value, save := traceStorageToPersist(tc.local, tc.existingMode)
			assert.Equal(t, tc.wantSave, save)
			assert.Equal(t, tc.wantValue, value)
		})
	}
}
