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

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: "CLAUDE.md", Kind: ConfigFileKindInstruction},
		{Path: ".claude/settings.json", Kind: ConfigFileKindConfiguration},
	}, "claude", gitCtx)
	require.NoError(t, err)

	assert.Equal(t, "claude", data.Agent.Name)
	assert.NotEmpty(t, data.ConfigHash)

	// Verify git context
	require.NotNil(t, data.GitContext)
	assert.Equal(t, "https://github.com/org/repo", data.GitContext.Repository)
	assert.Equal(t, "abc123", data.GitContext.CommitSHA)

	// Verify config files
	require.Len(t, data.ConfigFiles, 2)

	cf1 := data.ConfigFiles[0]
	assert.Equal(t, "CLAUDE.md", cf1.Path)
	assert.Equal(t, ConfigFileKindInstruction, cf1.Kind)
	assert.Equal(t, int64(len(file1Content)), cf1.Size)
	hash1 := sha256.Sum256(file1Content)
	assert.Equal(t, hex.EncodeToString(hash1[:]), cf1.SHA256)
	assert.Equal(t, base64.StdEncoding.EncodeToString(file1Content), cf1.Content)

	cf2 := data.ConfigFiles[1]
	assert.Equal(t, ".claude/settings.json", cf2.Path)
	assert.Equal(t, ConfigFileKindConfiguration, cf2.Kind)
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

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: ".cursor/rules/coding.md", Kind: ConfigFileKindInstruction},
	}, "cursor", nil)
	require.NoError(t, err)

	assert.Equal(t, "cursor", data.Agent.Name)
	require.Len(t, data.ConfigFiles, 1)
	assert.Equal(t, ".cursor/rules/coding.md", data.ConfigFiles[0].Path)
	assert.Equal(t, ConfigFileKindInstruction, data.ConfigFiles[0].Kind)
}

func TestBuildWithoutGitContext(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: "CLAUDE.md", Kind: ConfigFileKindInstruction},
	}, "claude", nil)
	require.NoError(t, err)

	assert.Nil(t, data.GitContext)
	assert.Len(t, data.ConfigFiles, 1)
}

func TestBuildJSONFormat(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: "CLAUDE.md", Kind: ConfigFileKindInstruction},
	}, "claude", nil)
	require.NoError(t, err)

	// Verify it marshals to valid JSON with top-level fields (no envelope)
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(jsonData, &raw))

	assert.NotNil(t, raw["agent"])
	assert.NotNil(t, raw["config_hash"])
	assert.NotNil(t, raw["config_files"])
	// Ensure no envelope fields
	assert.Nil(t, raw["chainloop.material.evidence.id"])
	assert.Nil(t, raw["data"])

	// Verify kind is present in config_files
	files := raw["config_files"].([]any)
	require.Len(t, files, 1)
	file := files[0].(map[string]any)
	assert.Equal(t, "instruction", file["kind"])
}

func TestBuildRejectsSymlinksEscapingRoot(t *testing.T) {
	rootDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create a file outside rootDir and a symlink pointing to it
	require.NoError(t, os.WriteFile(filepath.Join(outsideDir, "secret.txt"), []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(outsideDir, "secret.txt"), filepath.Join(rootDir, "CLAUDE.md")))

	_, err := Build(rootDir, []DiscoveredFile{
		{Path: "CLAUDE.md", Kind: ConfigFileKindInstruction},
	}, "claude", nil)
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

	_, err := Build(rootDir, []DiscoveredFile{
		{Path: ".claude/settings.json", Kind: ConfigFileKindConfiguration},
	}, "claude", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path escapes root directory via symlink")
}

func TestBuildEmptyFileList(t *testing.T) {
	rootDir := t.TempDir()

	data, err := Build(rootDir, []DiscoveredFile{}, "claude", nil)
	require.NoError(t, err)
	assert.Empty(t, data.ConfigFiles)
	assert.NotEmpty(t, data.ConfigHash)
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

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: "CLAUDE.md", Kind: ConfigFileKindInstruction},
	}, "claude", nil)
	require.NoError(t, err)
	assert.Len(t, data.ConfigFiles, 1)
}

func TestBuildExtractsMCPServers(t *testing.T) {
	rootDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, ".mcp.json"), []byte(`{
		"mcpServers": {
			"my-server": {
				"command": "npx",
				"args": ["-y", "my-package"],
				"env": {"API_KEY": "secret-value", "HOME": "/home/user"}
			}
		}
	}`), 0o600))

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: "CLAUDE.md", Kind: ConfigFileKindInstruction},
	}, "claude", nil)
	require.NoError(t, err)

	// MCP servers extracted from .mcp.json
	require.Len(t, data.MCPServers, 1)
	assert.Equal(t, "my-server", data.MCPServers[0].Name)
	assert.Equal(t, "npx", data.MCPServers[0].Command)
	assert.Equal(t, []string{"-y", "my-package"}, data.MCPServers[0].Args)
	assert.Equal(t, []string{"API_KEY", "HOME"}, data.MCPServers[0].EnvKeys)

	// .mcp.json must NOT appear in config_files
	for _, cf := range data.ConfigFiles {
		assert.NotEqual(t, ".mcp.json", cf.Path, ".mcp.json should not be in config_files")
	}
}

func TestBuildMCPServersFromSettings(t *testing.T) {
	rootDir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".claude"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, ".claude", "settings.json"), []byte(`{
		"permissions": {"allow": ["read"]},
		"mcpServers": {
			"settings-server": {"url": "https://example.com/mcp"}
		}
	}`), 0o600))

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: ".claude/settings.json", Kind: ConfigFileKindConfiguration},
	}, "claude", nil)
	require.NoError(t, err)

	require.Len(t, data.MCPServers, 1)
	assert.Equal(t, "settings-server", data.MCPServers[0].Name)
	assert.Equal(t, "https://example.com/mcp", data.MCPServers[0].URL)
}

func TestBuildMCPServersDeduplication(t *testing.T) {
	rootDir := t.TempDir()

	// .mcp.json defines "shared-server" with command "from-mcp"
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, ".mcp.json"), []byte(`{
		"mcpServers": {
			"shared-server": {"command": "from-mcp"},
			"mcp-only": {"command": "mcp-cmd"}
		}
	}`), 0o600))

	// settings.json defines "shared-server" with command "from-settings" and a unique server
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".claude"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, ".claude", "settings.json"), []byte(`{
		"mcpServers": {
			"shared-server": {"command": "from-settings"},
			"settings-only": {"url": "https://example.com"}
		}
	}`), 0o600))

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: ".claude/settings.json", Kind: ConfigFileKindConfiguration},
	}, "claude", nil)
	require.NoError(t, err)

	require.Len(t, data.MCPServers, 3)

	// Sorted by name: mcp-only, settings-only, shared-server
	assert.Equal(t, "mcp-only", data.MCPServers[0].Name)
	assert.Equal(t, "mcp-cmd", data.MCPServers[0].Command)

	assert.Equal(t, "settings-only", data.MCPServers[1].Name)
	assert.Equal(t, "https://example.com", data.MCPServers[1].URL)

	// shared-server comes from .mcp.json (first occurrence wins)
	assert.Equal(t, "shared-server", data.MCPServers[2].Name)
	assert.Equal(t, "from-mcp", data.MCPServers[2].Command)
}

func TestBuildNoMCPServersWhenNonePresent(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: "CLAUDE.md", Kind: ConfigFileKindInstruction},
	}, "claude", nil)
	require.NoError(t, err)

	assert.Nil(t, data.MCPServers)
}

func TestBuildMCPServersIgnoresInvalidJSON(t *testing.T) {
	rootDir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, ".mcp.json"), []byte(`not valid json`), 0o600))

	data, err := Build(rootDir, []DiscoveredFile{
		{Path: "CLAUDE.md", Kind: ConfigFileKindInstruction},
	}, "claude", nil)
	require.NoError(t, err)

	// Build succeeds but no MCP servers extracted
	assert.Nil(t, data.MCPServers)
	// Config files still collected (CLAUDE.md)
	require.Len(t, data.ConfigFiles, 1)
}
