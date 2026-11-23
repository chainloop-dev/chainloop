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

package trace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsUserLevelRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("COPILOT_HOME", "")

	t.Run("home directory itself", func(t *testing.T) {
		isUserLevel, err := IsUserLevelRoot(home)
		require.NoError(t, err)
		assert.True(t, isUserLevel)
	})

	t.Run("home directory through a symlink", func(t *testing.T) {
		link := filepath.Join(t.TempDir(), "home-link")
		require.NoError(t, os.Symlink(home, link))
		isUserLevel, err := IsUserLevelRoot(link)
		require.NoError(t, err)
		assert.True(t, isUserLevel, "a symlinked home directory is still the home directory")
	})

	t.Run("project below home", func(t *testing.T) {
		project := filepath.Join(home, "work", "myrepo")
		require.NoError(t, os.MkdirAll(project, 0o755))
		isUserLevel, err := IsUserLevelRoot(project)
		require.NoError(t, err)
		assert.False(t, isUserLevel)
	})

	t.Run("unrelated directory", func(t *testing.T) {
		isUserLevel, err := IsUserLevelRoot(t.TempDir())
		require.NoError(t, err)
		assert.False(t, isUserLevel)
	})

	t.Run("agent home folders", func(t *testing.T) {
		codexHome := filepath.Join(t.TempDir(), "codex-home")
		copilotHome := filepath.Join(t.TempDir(), "copilot-home")
		t.Setenv("CODEX_HOME", codexHome)
		t.Setenv("COPILOT_HOME", copilotHome)

		for _, dir := range []string{codexHome, copilotHome} {
			isUserLevel, err := IsUserLevelRoot(dir)
			require.NoError(t, err)
			assert.True(t, isUserLevel, "%s is an agent user-level folder", dir)
		}
	})

	t.Run("agent home defaults under home", func(t *testing.T) {
		for _, dir := range []string{filepath.Join(home, ".codex"), filepath.Join(home, ".copilot")} {
			isUserLevel, err := IsUserLevelRoot(dir)
			require.NoError(t, err)
			assert.True(t, isUserLevel, "%s is an agent user-level folder", dir)
		}
	})
}
