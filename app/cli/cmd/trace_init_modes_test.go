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
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/config"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// TestTraceInitOffline covers R-008/R-011: `trace init --offline` sets the
// repository up for the offline mode with no control plane, writing
// traceModes: [offline] and no control-plane identity.
func TestTraceInitOffline(t *testing.T) {
	t.Run("writes the offline mode and no identity, installs hooks", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)

		require.NoError(t, runTraceInit(t, repoDir, "--offline", "--claude"))

		modes, err := config.LoadTraceModesFromYML(repoDir)
		require.NoError(t, err)
		assert.Equal(t, config.TraceModes{Offline: true}, modes)

		data, err := os.ReadFile(filepath.Join(repoDir, chainloopYMLName))
		require.NoError(t, err)
		got := string(data)
		assert.NotContains(t, got, "organization:")
		assert.NotContains(t, got, "workflowName:")
		assert.NotContains(t, got, "projectName:")

		// The harness hooks were installed, so a session gets recorded.
		_, err = os.Stat(filepath.Join(repoDir, claudeSettings))
		assert.NoError(t, err)
	})

	t.Run("--trace-path is written with --offline", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)

		require.NoError(t, runTraceInit(t, repoDir, "--offline", "--trace-path", "evidence/sessions", "--claude"))

		assert.Equal(t, "evidence/sessions", config.LoadTracePathFromYML(repoDir))
	})

	t.Run("--require-trace is allowed and written as traceRequired", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)

		require.NoError(t, runTraceInit(t, repoDir, "--offline", "--require-trace", "--claude"))

		assert.True(t, config.LoadTraceRequiredFromYML(repoDir))
	})

	t.Run("control-plane flags are rejected without --online", func(t *testing.T) {
		for _, flag := range [][]string{
			{"--contract", "my-contract"},
			{"--workflow", "my-workflow"},
			{"--project", "my-project"}, //nolint:goconst // test fixture value
		} {
			repoDir := t.TempDir()
			gitInit(t, repoDir)
			args := append([]string{"--offline", "--claude"}, flag...)
			err := runTraceInit(t, repoDir, args...)
			assert.ErrorContains(t, err, "cannot be used without --online", "flag %v", flag)
		}
	})

	t.Run("a second run with no mode flag keeps the offline mode", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)

		require.NoError(t, runTraceInit(t, repoDir, "--offline", "--claude"))
		// No control plane is reachable here, so a run that fell back to the
		// online default would fail.
		require.NoError(t, runTraceInit(t, repoDir, "--claude"))

		modes, err := config.LoadTraceModesFromYML(repoDir)
		require.NoError(t, err)
		assert.Equal(t, config.TraceModes{Offline: true}, modes)
	})
}

// TestTraceInitInvalidModes covers the guard that an init over an invalid
// traceModes fails before installing hooks, so init never reports success for
// a repository whose every pre-push would then error, while an explicit mode
// flag overrides the bad value.
func TestTraceInitInvalidModes(t *testing.T) {
	t.Run("init with no mode flag over an invalid traceModes errors before install", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, chainloopYMLName),
			[]byte("traceModes: [bogus]\n"), 0600))

		err := runTraceInit(t, repoDir, "--claude")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "traceModes")

		// Nothing was written: the harness settings were not installed.
		_, statErr := os.Stat(filepath.Join(repoDir, claudeSettings))
		assert.True(t, os.IsNotExist(statErr), "hooks must not be installed over a broken config")
	})

	t.Run("--offline overrides an invalid traceModes", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, chainloopYMLName),
			[]byte("traceModes: [bogus]\n"), 0600))

		require.NoError(t, runTraceInit(t, repoDir, "--offline", "--claude"))

		modes, err := config.LoadTraceModesFromYML(repoDir)
		require.NoError(t, err)
		assert.Equal(t, config.TraceModes{Offline: true}, modes)
	})
}

func TestResolveTraceModes(t *testing.T) {
	online := config.TraceModes{Online: true}
	offline := config.TraceModes{Offline: true}
	both := config.TraceModes{Online: true, Offline: true}
	errBroken := errors.New("broken traceModes")

	testCases := []struct {
		name        string
		flags       traceModeFlags
		existing    config.TraceModes
		existingErr error
		want        config.TraceModes
		wantSave    bool
		wantErr     bool
	}{
		{name: "no flag on a fresh repository is online and writes nothing", existing: online, want: online},
		{name: "no flag keeps an existing offline mode", existing: offline, want: offline},
		{name: "no flag keeps existing both modes", existing: both, want: both},
		{name: "no flag over a broken value is an error", existingErr: errBroken, wantErr: true},
		{name: "--offline alone is offline only", flags: traceModeFlags{offline: true, offlineSet: true}, existing: online, want: offline, wantSave: true},
		{name: "--offline again writes nothing", flags: traceModeFlags{offline: true, offlineSet: true}, existing: offline, want: offline},
		{name: "--online --offline is both", flags: traceModeFlags{online: true, onlineSet: true, offline: true, offlineSet: true}, existing: online, want: both, wantSave: true},
		{name: "--online over offline switches to online", flags: traceModeFlags{online: true, onlineSet: true}, existing: offline, want: online, wantSave: true},
		{name: "--online on a fresh repository writes nothing", flags: traceModeFlags{online: true, onlineSet: true}, existing: online, want: online},
		{name: "a flag overrides a broken value", flags: traceModeFlags{offline: true, offlineSet: true}, existingErr: errBroken, want: offline, wantSave: true},
		{name: "every flag false is an error", flags: traceModeFlags{onlineSet: true}, existing: online, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, save, err := resolveTraceModes(tc.flags, tc.existing, tc.existingErr)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.wantSave, save)
		})
	}
}

func TestWriteTraceInitSummaryModes(t *testing.T) {
	t.Run("offline only names no control plane and how to connect one later", func(t *testing.T) {
		out := &bytes.Buffer{}
		writeTraceInitSummary(out, traceInitSummary{
			modes:        config.TraceModes{Offline: true},
			sessionsPath: ".git/chainloop-trace/local-sessions",
			harnesses:    []string{providerClaudeCode, "cursor"}, //nolint:goconst // provider wire name, kept literal like the other provider tests
		})

		got := out.String()
		assert.Contains(t, got, "offline (no control plane)")
		assert.Contains(t, got, ".git/chainloop-trace/local-sessions")
		assert.Contains(t, got, "claude-code, cursor")
		assert.NotContains(t, got, "organization")
		assert.Contains(t, got, "chainloop trace init --online --offline")
	})

	t.Run("both modes name the control plane and the disk", func(t *testing.T) {
		out := &bytes.Buffer{}
		writeTraceInitSummary(out, traceInitSummary{
			modes:        config.TraceModes{Online: true, Offline: true},
			organization: "acme",
			project:      "backend-api",
			workflow:     defaultTraceWorkflow,
			sessionsPath: ".git/chainloop-trace/local-sessions",
			harnesses:    []string{providerClaudeCode},
		})

		got := out.String()
		assert.Contains(t, got, "online, offline")
		assert.Contains(t, got, "acme")
		assert.Contains(t, got, "backend-api")
		assert.Contains(t, got, ".git/chainloop-trace/local-sessions")
		assert.NotContains(t, got, "--online --offline", "a repository with the online mode needs no hint to connect")
	})
}

func TestWriteTraceNextStepsOffline(t *testing.T) {
	repoRoot := t.TempDir()
	out := &bytes.Buffer{}
	writeTraceNextSteps(out, config.TraceModes{Offline: true}, repoRoot, repoRoot, providers.ByNames([]string{providerClaudeCode}))

	got := out.String()
	assert.Contains(t, got, chainloopYMLName)
	assert.Contains(t, got, claudeSettings)
	assert.Contains(t, got, "written to disk")
	assert.Contains(t, got, traceDocsURL)
	// The first session in a clone installs the git hooks, so a teammate does
	// not run init again.
	assert.NotContains(t, got, "chainloop trace init")
}

func TestWriteTraceModesChangedNote(t *testing.T) {
	out := &bytes.Buffer{}
	writeTraceModesChangedNote(out, config.TraceModes{Offline: true}, config.TraceModes{Online: true, Offline: true}, ".chainloop.yml")

	got := out.String()
	assert.Contains(t, got, "from offline to online, offline")
	assert.Contains(t, got, ".chainloop.yml")
}

// TestWithOfflineHint covers the hint an online init adds when the control
// plane rejects the credentials: the offline mode needs none.
func TestWithOfflineHint(t *testing.T) {
	t.Run("an authentication error suggests the offline mode", func(t *testing.T) {
		authErr := fmt.Errorf("resolve organization: %w",
			status.Error(codes.Unauthenticated, jwtMiddleware.ErrMissingJwtToken.Message))

		got := withOfflineHint(authErr)
		require.Error(t, got)
		assert.Contains(t, got.Error(), `please run "chainloop auth login"`)
		assert.Contains(t, got.Error(), "chainloop trace init --offline")
	})

	t.Run("an unreachable control plane suggests the offline mode", func(t *testing.T) {
		got := withOfflineHint(status.Error(codes.Unavailable, "connection refused"))
		require.Error(t, got)
		assert.Contains(t, got.Error(), "connection refused")
		assert.Contains(t, got.Error(), "chainloop trace init --offline")
	})

	t.Run("any other error is unchanged", func(t *testing.T) {
		other := errors.New("boom")
		assert.Equal(t, other, withOfflineHint(other))
		assert.NoError(t, withOfflineHint(nil))
	})
}
