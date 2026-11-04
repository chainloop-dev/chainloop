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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reflogRepo(t *testing.T, initArgs ...string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}
	dir := t.TempDir()
	reflogGit(t, dir, append([]string{"init", "-q", "-b", "main"}, initArgs...)...)
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

// reftableSupported reports whether the git binary can create a repository
// with the reftable ref storage (git 2.45 and later).
func reftableSupported(t *testing.T) bool {
	t.Helper()
	cmd := exec.Command("git", "init", "-q", "--ref-format=reftable", t.TempDir()) //nolint:gosec // a fixed git command on a test temp dir

	return cmd.Run() == nil
}

func TestHeadReflogSince(t *testing.T) {
	tests := []struct {
		name     string
		initArgs []string
		reftable bool
	}{
		{name: "files ref storage"},
		{name: "reftable ref storage", initArgs: []string{"--ref-format=reftable"}, reftable: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.reftable && !reftableSupported(t) {
				t.Skip("git does not support reftable")
			}
			dir := reflogRepo(t, tc.initArgs...)

			// No commit yet: the reflog is empty.
			start, err := HeadReflogPos(dir)
			require.NoError(t, err)
			assert.Zero(t, start)

			first := reflogCommit(t, dir, "a.txt", "a\n", "first")
			size, err := HeadReflogPos(dir)
			require.NoError(t, err)
			require.NotZero(t, size)

			entries, err := HeadReflogSince(dir, start)
			require.NoError(t, err)
			require.Len(t, entries, 1)
			assert.Equal(t, first, entries[0].New)
			assert.Equal(t, "commit (initial): first", entries[0].Message)
			assert.True(t, IsZeroOID(entries[0].Old), "the first commit replaces no commit: %q", entries[0].Old)

			second := reflogCommit(t, dir, "a.txt", "b\n", "second")
			reflogGit(t, dir, "checkout", "-q", "-b", "other", first)

			entries, err = HeadReflogSince(dir, size)
			require.NoError(t, err)
			require.Len(t, entries, 2, "only the entries written after the size was taken")
			assert.Equal(t, ReflogEntry{Old: first, New: second, Message: "commit: second"}, entries[0])
			assert.Equal(t, second, entries[1].Old)
			assert.Equal(t, first, entries[1].New)
			assert.True(t, strings.HasPrefix(entries[1].Message, "checkout: moving from main to other"), entries[1].Message)

			// Nothing written since the current position.
			now, err := HeadReflogPos(dir)
			require.NoError(t, err)
			entries, err = HeadReflogSince(dir, now)
			require.NoError(t, err)
			assert.Empty(t, entries)

			// More entries than one read of the reflog returns.
			const many = 40
			for i := range many {
				reflogGit(t, dir, "commit", "-q", "--allow-empty", "-m", fmt.Sprintf("empty %d", i))
			}
			entries, err = HeadReflogSince(dir, now)
			require.NoError(t, err)
			require.Len(t, entries, many)
			assert.Equal(t, first, entries[0].Old)
			assert.Equal(t, "commit: empty 0", entries[0].Message)
			for i := 1; i < many; i++ {
				assert.Equal(t, entries[i-1].New, entries[i].Old, "entry %d starts where the one before it ended", i)
			}
			assert.Equal(t, reflogGit(t, dir, "rev-parse", "HEAD"), entries[many-1].New)

			// A reflog that no longer holds the position was rewritten.
			_, err = HeadReflogSince(dir, ReflogPos{Offset: 1 << 40, Top: "gone"})
			assert.ErrorIs(t, err, ErrReflogRewritten)
		})
	}
}

// A linked worktree has its own HEAD reflog.
func TestHeadReflogSince_LinkedWorktree(t *testing.T) {
	for _, reftable := range []bool{false, true} {
		t.Run(map[bool]string{false: "files", true: "reftable"}[reftable], func(t *testing.T) {
			var initArgs []string
			if reftable {
				if !reftableSupported(t) {
					t.Skip("git does not support reftable")
				}
				initArgs = []string{"--ref-format=reftable"}
			}
			testLinkedWorktreeReflog(t, reflogRepo(t, initArgs...))
		})
	}
}

func TestCommitPaths(t *testing.T) {
	dir := reflogRepo(t)
	first := reflogCommit(t, dir, "a.txt", "a\n", "first")
	reflogCommit(t, dir, "b.txt", "b\n", "second")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	third := reflogCommit(t, dir, filepath.Join("sub", "c d.txt"), "c\n", "third")

	tests := []struct {
		name     string
		from, to string
		want     map[string]bool
	}{
		{name: "a range of commits", from: first, to: third, want: map[string]bool{"b.txt": true, "sub/c d.txt": true}},
		{name: "the first commit of a branch", from: strings.Repeat("0", len(first)), to: first, want: map[string]bool{"a.txt": true}},
		{name: "no change", from: third, to: third, want: map[string]bool{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CommitPaths(dir, tc.from, tc.to)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	_, err := CommitPaths(dir, first, "not-a-commit")
	assert.Error(t, err)
}

func TestBlobReader(t *testing.T) {
	dir := reflogRepo(t)
	first := reflogCommit(t, dir, "a.txt", "one\ntwo\n", "first")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	second := reflogCommit(t, dir, filepath.Join("sub", "b c.txt"), "", "second")

	r, err := NewBlobReader(dir)
	require.NoError(t, err)
	defer func() { assert.NoError(t, r.Close()) }()

	tests := []struct {
		name      string
		rev, path string
		want      []byte
	}{
		{name: "a file", rev: first, path: "a.txt", want: []byte("one\ntwo\n")},
		{name: "an empty file with a space in its path", rev: second, path: "sub/b c.txt", want: []byte{}},
		{name: "a path the commit does not hold", rev: first, path: "sub/b c.txt"},
		{name: "a directory", rev: second, path: "sub"},
		{name: "a commit that does not resolve", rev: strings.Repeat("0", len(first)), path: "a.txt"},
		{name: "a symbolic revision", rev: "HEAD", path: "a.txt", want: []byte("one\ntwo\n")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.FileAt(tc.rev, tc.path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	_, err = r.FileAt(first, "a\nb")
	assert.Error(t, err)
	got, err := r.FileAt(first, "a.txt")
	require.NoError(t, err, "a rejected name does not break later reads")
	assert.Equal(t, []byte("one\ntwo\n"), got)
}

func testLinkedWorktreeReflog(t *testing.T, dir string) {
	t.Helper()
	reflogCommit(t, dir, "a.txt", "a\n", "first")

	wt := filepath.Join(t.TempDir(), "wt")
	reflogGit(t, dir, "worktree", "add", "-q", "-b", "wt", wt)
	mainSize, err := HeadReflogPos(dir)
	require.NoError(t, err)
	wtSize, err := HeadReflogPos(wt)
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
