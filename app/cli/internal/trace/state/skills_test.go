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

package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillCopyDir(t *testing.T) {
	store := NewGitStore(filepath.Join(t.TempDir(), ".git"))

	a := store.SkillCopyDir("sess-a", "/home/me/.claude/skills/asd-ste100")
	assert.Equal(t, a, store.SkillCopyDir("sess-a", "/home/me/.claude/skills/asd-ste100/"), "the path is cleaned first")
	assert.NotEqual(t, a, store.SkillCopyDir("sess-a", "/home/me/.claude/skills/graphify"))
	assert.NotEqual(t, a, store.SkillCopyDir("sess-b", "/home/me/.claude/skills/asd-ste100"))
	assert.False(t, strings.Contains(a, "asd-ste100"), "the folder path is not in the state")
}

func TestSkillScanOffsets(t *testing.T) {
	store := NewGitStore(filepath.Join(t.TempDir(), ".git"))

	got, err := store.SkillScanOffsets("sess-a")
	require.NoError(t, err)
	assert.Empty(t, got, "a session that no hook scanned has no offsets")

	require.NoError(t, store.SetSkillScanOffsets("sess-a", map[string]int64{"main": 120, "agent-a1.jsonl": 40}))

	got, err = store.SkillScanOffsets("sess-a")
	require.NoError(t, err)
	assert.Equal(t, map[string]int64{"main": 120, "agent-a1.jsonl": 40}, got)

	other, err := store.SkillScanOffsets("sess-b")
	require.NoError(t, err)
	assert.Empty(t, other)
}

// TestGCOrphansDropsSkillCopies covers the skill copies of a session. They
// stay after the session end, for a push that comes later, and go with the
// session records.
func TestGCOrphansDropsSkillCopies(t *testing.T) {
	store := NewGitStore(filepath.Join(t.TempDir(), ".git"))
	require.NoError(t, store.InitTraceDir())

	const (
		live      = "skills-live"
		orphan    = "skills-orphan"
		committed = "2026-10-07T00:00:00Z"
	)

	require.NoError(t, store.SaveCommitRecord(&CommitRecord{SHA: "sha-skills-live", SessionIDs: []string{live}, Timestamp: committed}))
	require.NoError(t, store.SaveCommitRecord(&CommitRecord{SHA: "sha-skills-orphan", SessionIDs: []string{orphan}, Timestamp: committed}))
	for _, id := range []string{live, orphan} {
		require.NoError(t, store.SaveSessionRecord(&SessionRecord{SessionID: id, Active: true, StartedAt: time.Now().UTC().Format(time.RFC3339)}))
		copyDir := store.SkillCopyDir(id, "/skills/x")
		require.NoError(t, os.MkdirAll(copyDir, 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(copyDir, "info.json"), []byte("{}"), 0o600))
	}

	require.NoError(t, store.GCOrphans(map[string]bool{"sha-skills-live": true}))

	assert.FileExists(t, filepath.Join(store.SkillCopyDir(live, "/skills/x"), "info.json"))
	assert.NoDirExists(t, store.SkillCopyDir(orphan, "/skills/x"))
}
