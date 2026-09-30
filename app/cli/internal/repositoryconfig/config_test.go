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

package repositoryconfig

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadChainloopYML(t *testing.T) {
	t.Run("returns nearest config and path without requiring projectName", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		child := filepath.Join(repoDir, "subdir")
		require.NoError(t, os.Mkdir(child, 0755))
		path := filepath.Join(child, ".chainloop.yml")
		require.NoError(t, os.WriteFile(path, []byte("organization: my-org\nprojectVersion: v1.2.3\n"), 0600))

		cfg, gotPath, err := LoadChainloopYML(child)
		require.NoError(t, err)
		assert.Equal(t, resolveDir(path), gotPath)
		assert.Equal(t, "my-org", cfg.Organization)
		assert.Equal(t, "v1.2.3", cfg.ProjectVersion)
		assert.Empty(t, cfg.ProjectName)
	})

	t.Run("returns parse error from preferred file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".chainloop.yml")
		require.NoError(t, os.WriteFile(path, []byte(":\ninvalid: [yaml\n"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yaml"), []byte("projectName: ignored\n"), 0600))

		cfg, gotPath, err := LoadChainloopYML(dir)
		assert.Nil(t, cfg)
		assert.Equal(t, resolveDir(path), gotPath)
		assert.ErrorContains(t, err, "parse .chainloop.yml")
	})

	t.Run("returns not found error", func(t *testing.T) {
		cfg, path, err := LoadChainloopYML(t.TempDir())
		assert.Nil(t, cfg)
		assert.Empty(t, path)
		assert.ErrorIs(t, err, ErrChainloopYMLNotFound)
	})
}

func TestLoadProjectFromYML(t *testing.T) {
	t.Run("reads projectName from .chainloop.yml", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: my-project\n"), 0600))
		assert.Equal(t, "my-project", LoadProjectFromYML(dir))
	})

	t.Run("reads projectName from .chainloop.yaml", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yaml"), []byte("projectName: yaml-project\n"), 0600))
		assert.Equal(t, "yaml-project", LoadProjectFromYML(dir))
	})

	t.Run("prefers .chainloop.yml over .chainloop.yaml", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: from-yml\n"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yaml"), []byte("projectName: from-yaml\n"), 0600))
		assert.Equal(t, "from-yml", LoadProjectFromYML(dir))
	})

	t.Run("walks up to git repo root", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		child := filepath.Join(repoDir, "subdir")
		require.NoError(t, os.Mkdir(child, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".chainloop.yml"), []byte("projectName: parent-project\n"), 0600))
		assert.Equal(t, "parent-project", LoadProjectFromYML(child))
	})

	t.Run("does not walk above git repo root", func(t *testing.T) {
		parent := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(parent, ".chainloop.yml"), []byte("projectName: outside-repo\n"), 0600))

		repoDir := filepath.Join(parent, "repo")
		require.NoError(t, os.Mkdir(repoDir, 0755))
		initGitRepo(t, repoDir)

		assert.Empty(t, LoadProjectFromYML(repoDir))
	})

	t.Run("returns empty when no file found", func(t *testing.T) {
		assert.Empty(t, LoadProjectFromYML(t.TempDir()))
	})

	t.Run("returns empty for file without projectName", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("otherField: value\n"), 0600))
		assert.Empty(t, LoadProjectFromYML(dir))
	})
}

func TestFindChainloopYML(t *testing.T) {
	t.Run("reads fields from .chainloop.yml", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: my-project\nprojectVersion: v1.2.3\nrequireTrace: false\n"), 0600))
		cfg := FindChainloopYML(dir)
		require.NotNil(t, cfg)
		assert.Equal(t, "my-project", cfg.ProjectName)
		assert.Equal(t, "v1.2.3", cfg.ProjectVersion)
		require.NotNil(t, cfg.RequireTrace)
		assert.False(t, *cfg.RequireTrace)
	})

	t.Run("returns nil when no file found", func(t *testing.T) {
		assert.Nil(t, FindChainloopYML(t.TempDir()))
	})

	t.Run("leaves optional fields empty when not set", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: my-project\n"), 0600))
		cfg := FindChainloopYML(dir)
		require.NotNil(t, cfg)
		assert.Empty(t, cfg.ProjectVersion)
		assert.Nil(t, cfg.RequireTrace)
	})

	t.Run("does not skip nearest file without projectName", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		child := filepath.Join(repoDir, "subdir")
		require.NoError(t, os.Mkdir(child, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(child, ".chainloop.yml"), []byte("projectVersion: child\n"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".chainloop.yml"), []byte("projectName: parent\nprojectVersion: parent\n"), 0600))

		cfg := FindChainloopYML(child)
		require.NotNil(t, cfg)
		assert.Empty(t, cfg.ProjectName)
		assert.Equal(t, "child", cfg.ProjectVersion)
	})
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	initGitRepo(t, dir)
	return dir
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}

	cmd := exec.Command("git", "init", dir)
	require.NoError(t, cmd.Run())
}
