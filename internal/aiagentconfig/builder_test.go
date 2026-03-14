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

package aiagentconfig

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	rootDir := t.TempDir()

	// Create test files
	file1Content := []byte("# Project Rules\nAlways use Go.")
	file2Content := []byte(`{"allow": ["read"]}`)

	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), file1Content, 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".claude"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, ".claude", "settings.json"), file2Content, 0o600))

	gitCtx := &GitContext{
		Repository: "https://github.com/org/repo",
		CommitSHA:  "abc123",
	}

	data, err := Build(rootDir, []string{"CLAUDE.md", ".claude/settings.json"}, "claude", gitCtx)
	require.NoError(t, err)

	assert.Equal(t, "0.1", data.SchemaVersion)
	assert.Equal(t, "claude", data.Agent.Name)
	assert.NotEmpty(t, data.CapturedAt)
	assert.NotEmpty(t, data.ConfigHash)

	// Verify git context
	require.NotNil(t, data.GitContext)
	assert.Equal(t, "https://github.com/org/repo", data.GitContext.Repository)
	assert.Equal(t, "abc123", data.GitContext.CommitSHA)

	// Verify config files
	require.Len(t, data.ConfigFiles, 2)

	cf1 := data.ConfigFiles[0]
	assert.Equal(t, "CLAUDE.md", cf1.Path)
	assert.Equal(t, int64(len(file1Content)), cf1.Size)
	hash1 := sha256.Sum256(file1Content)
	assert.Equal(t, hex.EncodeToString(hash1[:]), cf1.SHA256)
	assert.Equal(t, base64.StdEncoding.EncodeToString(file1Content), cf1.Content)

	cf2 := data.ConfigFiles[1]
	assert.Equal(t, ".claude/settings.json", cf2.Path)
	hash2 := sha256.Sum256(file2Content)
	assert.Equal(t, hex.EncodeToString(hash2[:]), cf2.SHA256)

	// Verify config hash is deterministic (includes path:hash for rename detection)
	hashes := []string{
		fmt.Sprintf("CLAUDE.md:%s", hex.EncodeToString(hash1[:])),
		fmt.Sprintf(".claude/settings.json:%s", hex.EncodeToString(hash2[:])),
	}
	sort.Strings(hashes)
	combined := sha256.Sum256([]byte(strings.Join(hashes, "")))
	assert.Equal(t, hex.EncodeToString(combined[:]), data.ConfigHash)
}

func TestBuildWithCursorAgent(t *testing.T) {
	rootDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".cursor", "rules"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, ".cursor", "rules", "coding.md"), []byte("rules"), 0o600))

	data, err := Build(rootDir, []string{".cursor/rules/coding.md"}, "cursor", nil)
	require.NoError(t, err)

	assert.Equal(t, "cursor", data.Agent.Name)
	require.Len(t, data.ConfigFiles, 1)
	assert.Equal(t, ".cursor/rules/coding.md", data.ConfigFiles[0].Path)
}

func TestBuildWithoutGitContext(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))

	data, err := Build(rootDir, []string{"CLAUDE.md"}, "claude", nil)
	require.NoError(t, err)

	assert.Nil(t, data.GitContext)
	assert.Len(t, data.ConfigFiles, 1)
}

func TestBuildJSONFormat(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))

	data, err := Build(rootDir, []string{"CLAUDE.md"}, "claude", nil)
	require.NoError(t, err)

	// Verify it marshals to valid JSON with top-level fields (no envelope)
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(jsonData, &raw))

	assert.NotNil(t, raw["schema_version"])
	assert.NotNil(t, raw["agent"])
	assert.NotNil(t, raw["config_hash"])
	assert.NotNil(t, raw["captured_at"])
	assert.NotNil(t, raw["config_files"])
	// Ensure no envelope fields
	assert.Nil(t, raw["chainloop.material.evidence.id"])
	assert.Nil(t, raw["data"])
}

func TestBuildRejectsSymlinksEscapingRoot(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create a file outside rootDir and a symlink pointing to it
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(outsideDir, "secret.txt"), filepath.Join(rootDir, "CLAUDE.md")))

	_, err := Build(rootDir, []string{"CLAUDE.md"}, "claude", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path escapes root directory via symlink")
}

func TestBuildRejectsSymlinkedParentDir(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create a .claude directory outside rootDir with a config file
	outsideClaude := filepath.Join(outsideDir, "claude-data")
	require.NoError(t, os.MkdirAll(outsideClaude, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outsideClaude, "settings.json"), []byte(`{"secret": true}`), 0o600))

	// Symlink .claude -> outside directory
	require.NoError(t, os.Symlink(outsideClaude, filepath.Join(rootDir, ".claude")))

	_, err := Build(rootDir, []string{".claude/settings.json"}, "claude", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path escapes root directory via symlink")
}

func TestBuildEmptyFileList(t *testing.T) {
	rootDir := t.TempDir()

	data, err := Build(rootDir, []string{}, "claude", nil)
	require.NoError(t, err)
	assert.Empty(t, data.ConfigFiles)
	assert.NotEmpty(t, data.ConfigHash)
	assert.NotEmpty(t, data.CapturedAt)
}

func TestBuildNilFileList(t *testing.T) {
	rootDir := t.TempDir()

	data, err := Build(rootDir, nil, "claude", nil)
	require.NoError(t, err)
	assert.Empty(t, data.ConfigFiles)
}

func TestBuildAllowsRegularFilesInRoot(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))

	data, err := Build(rootDir, []string{"CLAUDE.md"}, "claude", nil)
	require.NoError(t, err)
	assert.Len(t, data.ConfigFiles, 1)
}
