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

package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTempGitRepo creates a temp dir and runs git init in it.
func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	initGitRepo(t, dir)
	return dir
}

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", dir)
	require.NoError(t, cmd.Run())
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

		// Run from inside the child dir so git rev-parse finds our repo
		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(child))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		assert.Equal(t, "parent-project", LoadProjectFromYML(child))
	})

	t.Run("does not walk above git repo root", func(t *testing.T) {
		parent := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(parent, ".chainloop.yml"), []byte("projectName: outside-repo\n"), 0600))

		repoDir := filepath.Join(parent, "repo")
		require.NoError(t, os.Mkdir(repoDir, 0755))
		initGitRepo(t, repoDir)

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(repoDir))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		assert.Empty(t, LoadProjectFromYML(repoDir))
	})

	t.Run("returns empty when no file found", func(t *testing.T) {
		dir := t.TempDir()
		assert.Empty(t, LoadProjectFromYML(dir))
	})

	t.Run("skips file without projectName", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("otherField: value\n"), 0600))
		assert.Empty(t, LoadProjectFromYML(dir))
	})
}

func TestSaveProjectToYML(t *testing.T) {
	t.Run("creates new file", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, SaveProjectToYML(dir, "new-project"))

		got := LoadProjectFromYML(dir)
		assert.Equal(t, "new-project", got)
	})

	t.Run("preserves existing fields", func(t *testing.T) {
		dir := t.TempDir()
		existing := "projectName: old-project\nprojectVersion: v1.0.0\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte(existing), 0600))

		require.NoError(t, SaveProjectToYML(dir, "updated-project"))

		data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "projectName: updated-project")
		assert.Contains(t, string(data), "projectVersion: v1.0.0")
	})

	t.Run("updates existing projectName", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: old\n"), 0600))

		require.NoError(t, SaveProjectToYML(dir, "new"))
		assert.Equal(t, "new", LoadProjectFromYML(dir))
	})

	t.Run("respects existing .chainloop.yaml extension", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yaml"), []byte("projectName: yaml-ext\nprojectVersion: v2.0.0\n"), 0600))

		require.NoError(t, SaveProjectToYML(dir, "updated"))

		// Should have updated the .yaml file, not created a new .yml file
		_, err := os.Stat(filepath.Join(dir, ".chainloop.yml"))
		assert.True(t, os.IsNotExist(err), "should not create .yml when .yaml exists")

		data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "projectName: updated")
		assert.Contains(t, string(data), "projectVersion: v2.0.0")
	})

	t.Run("returns error on malformed yaml", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte(":\ninvalid: [yaml\n"), 0600))

		err := SaveProjectToYML(dir, "proj")
		assert.Error(t, err)
	})
}

func TestRequireTrace(t *testing.T) {
	t.Run("defaults to false when field missing", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\n"), 0600))
		assert.False(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("defaults to false when no config file", func(t *testing.T) {
		dir := t.TempDir()
		assert.False(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("returns false when explicitly set", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\nrequireTrace: false\n"), 0600))
		assert.False(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("reads from file without projectName", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("requireTrace: false\n"), 0600))
		assert.False(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("returns true when explicitly set", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\nrequireTrace: true\n"), 0600))
		assert.True(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("save and load round-trip", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\n"), 0600))

		require.NoError(t, SaveRequireTraceToYML(dir, false))
		assert.False(t, LoadRequireTraceFromYML(dir))

		require.NoError(t, SaveRequireTraceToYML(dir, true))
		assert.True(t, LoadRequireTraceFromYML(dir))
	})

	t.Run("preserves other fields", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: my-proj\nprojectVersion: v1.0.0\n"), 0600))

		require.NoError(t, SaveRequireTraceToYML(dir, false))

		data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "projectName: my-proj")
		assert.Contains(t, string(data), "projectVersion: v1.0.0")
		assert.Contains(t, string(data), "requireTrace: false")
	})
}

func TestWorkflow(t *testing.T) {
	t.Run("defaults to empty when field missing", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\n"), 0600))
		assert.Empty(t, LoadWorkflowFromYML(dir))
	})

	t.Run("defaults to empty when no config file", func(t *testing.T) {
		dir := t.TempDir()
		assert.Empty(t, LoadWorkflowFromYML(dir))
	})

	t.Run("reads from file without projectName", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("workflowName: my-flow\n"), 0600))
		assert.Equal(t, "my-flow", LoadWorkflowFromYML(dir))
	})

	t.Run("save and load round-trip", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\n"), 0600))

		require.NoError(t, SaveWorkflowToYML(dir, "my-flow"))
		assert.Equal(t, "my-flow", LoadWorkflowFromYML(dir))

		require.NoError(t, SaveWorkflowToYML(dir, "another"))
		assert.Equal(t, "another", LoadWorkflowFromYML(dir))
	})

	t.Run("preserves other fields", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: my-proj\nprojectVersion: v1.0.0\nrequireTrace: true\n"), 0600))

		require.NoError(t, SaveWorkflowToYML(dir, "my-flow"))

		data, err := os.ReadFile(filepath.Join(dir, ".chainloop.yml"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "projectName: my-proj")
		assert.Contains(t, string(data), "projectVersion: v1.0.0")
		assert.Contains(t, string(data), "requireTrace: true")
		assert.Contains(t, string(data), "workflowName: my-flow")
	})
}

func TestFindChainloopYML(t *testing.T) {
	t.Run("reads both fields from .chainloop.yml", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: my-project\nprojectVersion: v1.2.3\n"), 0600))
		cfg := FindChainloopYML(dir)
		require.NotNil(t, cfg)
		assert.Equal(t, "my-project", cfg.ProjectName)
		assert.Equal(t, "v1.2.3", cfg.ProjectVersion)
	})

	t.Run("returns nil when no file found", func(t *testing.T) {
		dir := t.TempDir()
		assert.Nil(t, FindChainloopYML(dir))
	})

	t.Run("returns empty version when not set", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("projectName: my-project\n"), 0600))
		cfg := FindChainloopYML(dir)
		require.NotNil(t, cfg)
		assert.Equal(t, "my-project", cfg.ProjectName)
		assert.Empty(t, cfg.ProjectVersion)
	})

	t.Run("walks up to git repo root", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		child := filepath.Join(repoDir, "subdir")
		require.NoError(t, os.Mkdir(child, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".chainloop.yml"), []byte("projectName: p\nprojectVersion: v2.0.0\n"), 0600))

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(child))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		cfg := FindChainloopYML(child)
		require.NotNil(t, cfg)
		assert.Equal(t, "p", cfg.ProjectName)
		assert.Equal(t, "v2.0.0", cfg.ProjectVersion)
	})

	t.Run("reads requireTrace field", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\nrequireTrace: false\n"), 0600))
		cfg := FindChainloopYML(dir)
		require.NotNil(t, cfg)
		require.NotNil(t, cfg.RequireTrace)
		assert.False(t, *cfg.RequireTrace)
	})

	t.Run("requireTrace nil when not set", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"),
			[]byte("projectName: p\n"), 0600))
		cfg := FindChainloopYML(dir)
		require.NotNil(t, cfg)
		assert.Nil(t, cfg.RequireTrace)
	})

	t.Run("skips file without projectName and walks up", func(t *testing.T) {
		repoDir := initTempGitRepo(t)
		child := filepath.Join(repoDir, "subdir")
		require.NoError(t, os.Mkdir(child, 0755))
		// Child has a .chainloop.yml without projectName
		require.NoError(t, os.WriteFile(filepath.Join(child, ".chainloop.yml"), []byte("otherField: value\n"), 0600))
		// Parent has the real config
		require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".chainloop.yml"), []byte("projectName: parent\nprojectVersion: v3.0.0\n"), 0600))

		origDir, _ := os.Getwd()
		require.NoError(t, os.Chdir(child))
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		cfg := FindChainloopYML(child)
		require.NotNil(t, cfg)
		assert.Equal(t, "parent", cfg.ProjectName)
		assert.Equal(t, "v3.0.0", cfg.ProjectVersion)
	})
}
