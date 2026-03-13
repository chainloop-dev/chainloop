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

func TestBuildEvidence(t *testing.T) {
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

	evidence, err := BuildEvidence(rootDir, []string{"CLAUDE.md", ".claude/settings.json"}, gitCtx)
	require.NoError(t, err)

	// Verify envelope
	assert.Equal(t, EvidenceID, evidence.ID)
	assert.Equal(t, EvidenceSchemaURL, evidence.Schema)

	// Verify data
	assert.Equal(t, "v1alpha", evidence.Data.SchemaVersion)
	assert.Equal(t, "claude", evidence.Data.Agent.Name)
	assert.NotEmpty(t, evidence.Data.CapturedAt)
	assert.NotEmpty(t, evidence.Data.ConfigHash)

	// Verify git context
	require.NotNil(t, evidence.Data.GitContext)
	assert.Equal(t, "https://github.com/org/repo", evidence.Data.GitContext.Repository)
	assert.Equal(t, "abc123", evidence.Data.GitContext.CommitSHA)

	// Verify config files
	require.Len(t, evidence.Data.ConfigFiles, 2)

	cf1 := evidence.Data.ConfigFiles[0]
	assert.Equal(t, "CLAUDE.md", cf1.Path)
	assert.Equal(t, int64(len(file1Content)), cf1.Size)
	hash1 := sha256.Sum256(file1Content)
	assert.Equal(t, hex.EncodeToString(hash1[:]), cf1.SHA256)
	assert.Equal(t, base64.StdEncoding.EncodeToString(file1Content), cf1.Base64Content)

	cf2 := evidence.Data.ConfigFiles[1]
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
	assert.Equal(t, hex.EncodeToString(combined[:]), evidence.Data.ConfigHash)
}

func TestBuildEvidenceWithoutGitContext(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))

	evidence, err := BuildEvidence(rootDir, []string{"CLAUDE.md"}, nil)
	require.NoError(t, err)

	assert.Nil(t, evidence.Data.GitContext)
	assert.Len(t, evidence.Data.ConfigFiles, 1)
}

func TestBuildEvidenceJSONFormat(t *testing.T) {
	rootDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "CLAUDE.md"), []byte("content"), 0o600))

	evidence, err := BuildEvidence(rootDir, []string{"CLAUDE.md"}, nil)
	require.NoError(t, err)

	// Verify it marshals to valid JSON with the correct envelope field
	jsonData, err := json.Marshal(evidence)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, json.Unmarshal(jsonData, &raw))

	assert.Equal(t, EvidenceID, raw["chainloop.material.evidence.id"])
	assert.Equal(t, EvidenceSchemaURL, raw["schema"])
	assert.NotNil(t, raw["data"])
}

func TestBuildEvidenceRejectsSymlinks(t *testing.T) {
	rootDir := t.TempDir()

	// Create a real file and a symlink to it
	realFile := filepath.Join(rootDir, "real.txt")
	require.NoError(t, os.WriteFile(realFile, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(realFile, filepath.Join(rootDir, "CLAUDE.md")))

	_, err := BuildEvidence(rootDir, []string{"CLAUDE.md"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlinks are not supported")
}
