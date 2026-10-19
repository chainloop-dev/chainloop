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
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	lineOne  = "one"
	lineTwo  = "two"
	lineFour = "four"

	fileA        = "a.txt"
	fileB        = "b.txt"
	lineUpperTwo = "TWO"
)

// testClients returns both implementations of the client, by name.
func testClients() map[string]Client {
	return map[string]Client{"gogit": NewGoGitClient(), "exec": NewExecClient()}
}

func TestCommitLinesAndFileAt(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string { return gitOutput(dir, args...) }
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	write(fileA, "one\ntwo\nthree\n")
	write("gone.txt", "bye\n")
	write("bin.dat", "\x00\x01\x02")
	run("add", ".")
	run("commit", "-m", "root")
	root := run("rev-parse", "HEAD")

	write(fileA, "one\nTWO\nthree\nfour")
	write("new.txt", "hello\n")
	write("with space.txt", "spaced\n")
	write("bin.dat", "\x00\x03")
	require.NoError(t, os.Remove(filepath.Join(dir, "gone.txt")))
	run("add", "-A")
	run("commit", "-m", "second")
	second := run("rev-parse", "HEAD")

	clients := testClients()
	for name, c := range clients {
		t.Run(name, func(t *testing.T) {
			t.Run("root commit adds every line", func(t *testing.T) {
				got, err := c.CommitLines(dir, root)
				require.NoError(t, err)
				sortFileLines(got)
				rootA, rootGone := []string{lineOne, lineTwo, "three"}, []string{"bye"}
				assert.Equal(t, []FileLines{
					{Path: fileA, Added: rootA, Hunks: []Hunk{{OldStart: 1, Added: rootA}}},
					{Path: "gone.txt", Added: rootGone, Hunks: []Hunk{{OldStart: 1, Added: rootGone}}},
				}, got, "binary files are left out")
			})

			t.Run("commit lines against the first parent", func(t *testing.T) {
				got, err := c.CommitLines(dir, second)
				require.NoError(t, err)
				sortFileLines(got)
				two, upperTwo, bye := []string{lineTwo}, []string{lineUpperTwo}, []string{"bye"}
				assert.Equal(t, []FileLines{
					{Path: fileA, Added: []string{upperTwo[0], lineFour}, Removed: two, Hunks: []Hunk{
						{OldStart: 2, Removed: two, Added: upperTwo},
						{OldStart: 4, Added: []string{lineFour}},
					}},
					{Path: "gone.txt", Removed: bye, Hunks: []Hunk{{OldStart: 1, Removed: bye}}},
					{Path: "new.txt", Added: []string{"hello"}, Hunks: []Hunk{{OldStart: 1, Added: []string{"hello"}}}},
					{Path: "with space.txt", Added: []string{"spaced"}, Hunks: []Hunk{{OldStart: 1, Added: []string{"spaced"}}}},
				}, got)
			})

			t.Run("file content at a commit", func(t *testing.T) {
				content, err := c.FileAt(dir, second, fileA)
				require.NoError(t, err)
				assert.Equal(t, "one\nTWO\nthree\nfour", string(content))

				content, err = c.FileAt(dir, second, "gone.txt")
				require.NoError(t, err)
				assert.Nil(t, content, "a file the commit does not hold has no content")

				content, err = c.FileAt(dir, "HEAD", "new.txt")
				require.NoError(t, err)
				assert.Equal(t, "hello\n", string(content), "a revision name resolves")

				_, err = c.FileAt(dir, "deadbeef00000000000000000000000000000000", fileA)
				assert.Error(t, err, "an unknown commit is an error, not an absent file")
			})

			t.Run("an unknown commit is an error", func(t *testing.T) {
				_, err := c.CommitLines(dir, "deadbeef00000000000000000000000000000000")
				assert.Error(t, err)
			})

			t.Run("blame names the last writer of each line", func(t *testing.T) {
				got, err := c.Blame(dir, second, fileA)
				require.NoError(t, err)
				assert.Equal(t, []BlameLine{
					{SHA: root, Text: lineOne},
					{SHA: second, Text: lineUpperTwo},
					{SHA: root, Text: "three"},
					{SHA: second, Text: lineFour},
				}, got)

				got, err = c.Blame(dir, second+"^", fileA)
				require.NoError(t, err)
				assert.Len(t, got, 3, "a revision name resolves")

				got, err = c.Blame(dir, second, "gone.txt")
				require.NoError(t, err)
				assert.Nil(t, got, "a file the commit does not hold has no lines")
			})
		})
	}
}

func TestCommitLinesRenames(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string { return gitOutput(dir, args...) }
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
	}

	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")
	run("config", "diff.renames", "false")

	body := "one\ntwo\nthree\nfour\nfive\nsix\n"
	write(fileA, body)
	write(fileB, body+"seven\n")
	// Git quotes a path with a quote in the headers of a diff.
	write("q\"ab.txt", "quote one\nquote two\nquote three\n")
	run("add", ".")
	run("commit", "-m", "root")
	root := run("rev-parse", "HEAD")

	run("mv", fileA, "moved.txt")
	run("mv", fileB, "changed.txt")
	run("mv", "q\"ab.txt", "q\"moved.txt")
	write("changed.txt", strings.Replace(body+"seven\n", lineTwo, lineUpperTwo, 1))
	run("add", "-A")
	run("commit", "-m", "rename")
	renamed := run("rev-parse", "HEAD")

	run("mv", "moved.txt", "again.txt")
	run("commit", "-m", "rename again")

	clients := testClients()
	for name, c := range clients {
		t.Run(name, func(t *testing.T) {
			t.Run("a renamed file keeps its unchanged lines", func(t *testing.T) {
				got, err := c.CommitLines(dir, renamed)
				require.NoError(t, err)
				sortFileLines(got)
				two, upperTwo := []string{lineTwo}, []string{lineUpperTwo}
				assert.Equal(t, []FileLines{
					{Path: "changed.txt", OldPath: fileB, Added: upperTwo, Removed: two, Hunks: []Hunk{{OldStart: 2, Removed: two, Added: upperTwo}}},
					{Path: "moved.txt", OldPath: fileA},
					{Path: "q\"moved.txt", OldPath: "q\"ab.txt"},
				}, got)
			})

			t.Run("renames between two commits", func(t *testing.T) {
				got, err := c.Renames(dir, root, "HEAD")
				require.NoError(t, err)
				assert.Equal(t, map[string]string{fileA: "again.txt", fileB: "changed.txt", "q\"ab.txt": "q\"moved.txt"}, got)

				got, err = c.Renames(dir, "HEAD", "HEAD")
				require.NoError(t, err)
				assert.Empty(t, got)

				_, err = c.Renames(dir, "deadbeef00000000000000000000000000000000", "HEAD")
				assert.Error(t, err)
			})
		})
	}
}

// Both clients find a rename that keeps at least half of the file, as git
// does by default.
func TestRenamesKeepHalf(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string { return gitOutput(dir, args...) }
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	var kept, changed strings.Builder
	for i := range 20 {
		line := fmt.Sprintf("line number %02d\n", i)
		kept.WriteString(line)
		if i < 9 {
			line = fmt.Sprintf("LINE CHANGE %02d\n", i)
		}
		changed.WriteString(line)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileA), []byte(kept.String()), 0600))
	run("add", ".")
	run("commit", "-m", "root")
	root := run("rev-parse", "HEAD")

	run("mv", fileA, fileB)
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileB), []byte(changed.String()), 0600))
	run("add", "-A")
	run("commit", "-m", "rename")

	clients := testClients()
	for name, c := range clients {
		t.Run(name, func(t *testing.T) {
			got, err := c.Renames(dir, root, "HEAD")
			require.NoError(t, err)
			assert.Equal(t, map[string]string{fileA: fileB}, got)
		})
	}
}

func sortFileLines(fl []FileLines) {
	sort.Slice(fl, func(i, j int) bool { return fl[i].Path < fl[j].Path })
}

func TestNewGitClient(t *testing.T) {
	testCases := []struct {
		name    string
		lookErr error
		want    Client
	}{
		{name: "git on the PATH", want: &ExecClient{}},
		{name: "no git binary", lookErr: exec.ErrNotFound, want: &GoGitClient{}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			orig := lookPath
			t.Cleanup(func() { lookPath = orig })
			lookPath = func(string) (string, error) { return "/usr/bin/git", tc.lookErr }

			assert.IsType(t, tc.want, NewGitClient())
		})
	}
}

func TestGoGitBlameFallback(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) string { return gitOutput(dir, args...) }
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(filepath.Join(dir, fileA), []byte("one\ntwo\n"), 0600))
	run("add", ".")
	run("commit", "-m", "root")
	root := run("rev-parse", "HEAD")

	got, err := goGitBlame(dir, "HEAD", fileA)
	require.NoError(t, err)
	assert.Equal(t, []BlameLine{{SHA: root, Text: lineOne}, {SHA: root, Text: lineTwo}}, got)
}
