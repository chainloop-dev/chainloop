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

func TestTraceExportCommand(t *testing.T) {
	t.Run("needs exactly one directory argument", func(t *testing.T) {
		cmd := newTraceExportCmd()
		cmd.SetArgs(nil)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		assert.Error(t, cmd.Execute())
	})

	t.Run("does not require the control plane", func(t *testing.T) {
		// The export talks to no control plane, so the command opts out of the
		// authenticated action-options setup the root command does.
		cmd := newTraceExportCmd()
		assert.Equal(t, trueString, cmd.Annotations[skipActionOptsInit])
	})

	t.Run("exports nothing when no session is recorded", func(t *testing.T) {
		repoRoot := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755))
		t.Chdir(repoRoot)
		out := t.TempDir()

		cmd := newTraceExportCmd()
		cmd.SetArgs([]string{out})
		require.NoError(t, cmd.Execute())
	})
}
