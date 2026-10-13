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
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTraceInitLocal runs `trace init --local` end to end. It needs no control
// plane, which is the point: the command resolves no identity and creates no
// workflow, so the whole run stays on the machine.
func TestTraceInitLocal(t *testing.T) {
	repoRoot := t.TempDir()
	gitDir := filepath.Join(repoRoot, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))
	t.Chdir(repoRoot)

	cmd := traceInitTestCmd(t, "--local", "--claude")
	require.NoError(t, cmd.Execute())

	store := state.NewGitStore(gitDir)
	assert.True(t, store.IsLocal(), "the clone is marked as local")
	assert.True(t, store.IsTraceInitialized(), "the clone is initialized")

	// The harness hooks are installed, so sessions are recorded.
	_, err := os.Stat(filepath.Join(repoRoot, claudeSettings))
	assert.NoError(t, err, "the Claude hooks are installed")

	// Local mode is per-clone, recorded in the trace state, never in the shared
	// .chainloop.yml: a teammate who clones the repository and runs a normal
	// init must not inherit a turned-off attestation.
	_, err = os.Stat(filepath.Join(repoRoot, chainloopYMLName))
	assert.True(t, os.IsNotExist(err), "local mode writes nothing to the shared config")
}

// TestWriteTraceInitStateTogglesLocal checks that re-initializing without
// --local clears the per-clone local marker, so a clone switched back to the
// backend path starts pushing attestations again.
func TestWriteTraceInitStateTogglesLocal(t *testing.T) {
	repoRoot := t.TempDir()
	gitDir := filepath.Join(repoRoot, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o755))

	cfg := &traceInitConfig{}
	store := state.NewGitStore(gitDir)

	require.NoError(t, writeTraceInitState(cfg, repoRoot, gitDir, true))
	assert.True(t, store.IsLocal(), "a local init marks the clone local")

	require.NoError(t, writeTraceInitState(cfg, repoRoot, gitDir, false))
	assert.False(t, store.IsLocal(), "a normal init clears the stale local marker")
}

func TestTraceInitLocalRejectsContradictoryFlags(t *testing.T) {
	testCases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "contract",
			args: []string{"--local", "--contract", "my-contract"},
			want: "--contract cannot be used with --local",
		},
		{
			name: "require-trace",
			args: []string{"--local", "--require-trace"},
			want: "--require-trace cannot be used with --local",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755))
			t.Chdir(repoRoot)

			cmd := traceInitTestCmd(t, tc.args...)
			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}
