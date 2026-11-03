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

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reflogRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}
	dir := t.TempDir()
	reflogGit(t, dir, "init", "-q", "-b", "main")
	reflogGit(t, dir, "config", "user.email", "test@example.com")
	reflogGit(t, dir, "config", "user.name", "Test")
	reflogGit(t, dir, "config", "commit.gpgsign", "false")

	return dir
}

func reflogGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)

	return strings.TrimSpace(string(out))
}

func reflogCommit(t *testing.T, dir, file, text, msg string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(text), 0600))
	reflogGit(t, dir, "add", file)
	reflogGit(t, dir, "commit", "-q", "-m", msg)

	return reflogGit(t, dir, "rev-parse", "HEAD")
}

func TestHeadReflogSince(t *testing.T) {
	dir := reflogRepo(t)

	// No commit yet: there is no reflog, and its size is zero.
	size, err := HeadReflogSize(dir)
	require.NoError(t, err)
	assert.Zero(t, size)

	first := reflogCommit(t, dir, "a.txt", "a\n", "first")
	size, err = HeadReflogSize(dir)
	require.NoError(t, err)
	require.Positive(t, size)

	second := reflogCommit(t, dir, "a.txt", "b\n", "second")
	reflogGit(t, dir, "checkout", "-q", "-b", "other", first)

	entries, err := HeadReflogSince(dir, size)
	require.NoError(t, err)
	require.Len(t, entries, 2, "only the entries written after the size was taken")
	assert.Equal(t, ReflogEntry{Old: first, New: second, Message: "commit: second"}, entries[0])
	assert.Equal(t, second, entries[1].Old)
	assert.Equal(t, first, entries[1].New)
	assert.True(t, strings.HasPrefix(entries[1].Message, "checkout: moving from main to other"), entries[1].Message)

	// Nothing written since the current size.
	now, err := HeadReflogSize(dir)
	require.NoError(t, err)
	entries, err = HeadReflogSince(dir, now)
	require.NoError(t, err)
	assert.Empty(t, entries)

	// A reflog that is now shorter than the offset was rewritten.
	_, err = HeadReflogSince(dir, now+1)
	assert.ErrorIs(t, err, ErrReflogRewritten)
}

// A linked worktree has its own HEAD reflog.
func TestHeadReflogSince_LinkedWorktree(t *testing.T) {
	dir := reflogRepo(t)
	reflogCommit(t, dir, "a.txt", "a\n", "first")

	wt := filepath.Join(t.TempDir(), "wt")
	reflogGit(t, dir, "worktree", "add", "-q", "-b", "wt", wt)
	mainSize, err := HeadReflogSize(dir)
	require.NoError(t, err)
	wtSize, err := HeadReflogSize(wt)
	require.NoError(t, err)

	reflogCommit(t, wt, "b.txt", "b\n", "in worktree")

	entries, err := HeadReflogSince(wt, wtSize)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "commit: in worktree", entries[0].Message)

	entries, err = HeadReflogSince(dir, mainSize)
	require.NoError(t, err)
	assert.Empty(t, entries, "the main checkout's HEAD did not move")
}
