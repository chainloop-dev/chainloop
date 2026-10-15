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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runTraceRun executes `trace run` with the given args in dir. It returns the
// RunE error.
func runTraceRun(t *testing.T, dir string, args ...string) error {
	t.Helper()
	t.Chdir(dir)

	cmd := newTraceRunCmd()
	cmd.Flags().String("org", "", "organization name")
	cmd.SetArgs(args)

	return cmd.Execute()
}

// TestTraceRunLocalFlag covers the --local flag of trace run. The flag alone
// selects local mode: trace run does not read traceStorage from .chainloop.yml.
func TestTraceRunLocalFlag(t *testing.T) {
	t.Run("control-plane flags are rejected with --local", func(t *testing.T) {
		for _, flag := range [][]string{
			{"--org", "acme"},
			{"--project", "my-project"},
			{"--workflow", "my-workflow"},
			{"--version", "v1.0.0"},
			{"--contract", "my-contract"},
		} {
			repoDir := t.TempDir()
			gitInit(t, repoDir)
			args := append(append([]string{"--local"}, flag...), "--", "true")
			err := runTraceRun(t, repoDir, args...)
			assert.ErrorContains(t, err, "with --local", "flag %v", flag)
		}
	})

	t.Run("traceStorage LOCAL in .chainloop.yml does not select local mode", func(t *testing.T) {
		repoDir := t.TempDir()
		gitInit(t, repoDir)
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, chainloopYMLName),
			[]byte("traceStorage: LOCAL\n"), 0600))

		// Without --local, the control-plane identity is required.
		err := runTraceRun(t, repoDir, "--", "true")
		assert.ErrorContains(t, err, "--org is required")
	})
}
