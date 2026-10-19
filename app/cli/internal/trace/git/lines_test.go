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
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	write("a.txt", "one\ntwo\nthree\n")
	write("gone.txt", "bye\n")
	write("bin.dat", "\x00\x01\x02")
	run("add", ".")
	run("commit", "-m", "root")
	root := run("rev-parse", "HEAD")

	write("a.txt", "one\nTWO\nthree\nfour")
	write("new.txt", "hello\n")
	write("with space.txt", "spaced\n")
	write("bin.dat", "\x00\x03")
	require.NoError(t, os.Remove(filepath.Join(dir, "gone.txt")))
	run("add", "-A")
	run("commit", "-m", "second")
	second := run("rev-parse", "HEAD")

	clients := map[string]Client{"gogit": NewGoGitClient(), "exec": NewExecClient()}
	for name, c := range clients {
		t.Run(name, func(t *testing.T) {
			t.Run("root commit adds every line", func(t *testing.T) {
				got, err := c.CommitLines(dir, root)
				require.NoError(t, err)
				sortFileLines(got)
				assert.Equal(t, []FileLines{
					{Path: "a.txt", Added: []string{"one", "two", "three"}},
					{Path: "gone.txt", Added: []string{"bye"}},
				}, got, "binary files are left out")
			})

			t.Run("commit lines against the first parent", func(t *testing.T) {
				got, err := c.CommitLines(dir, second)
				require.NoError(t, err)
				sortFileLines(got)
				assert.Equal(t, []FileLines{
					{Path: "a.txt", Added: []string{"TWO", "four"}, Removed: []string{"two"}},
					{Path: "gone.txt", Removed: []string{"bye"}},
					{Path: "new.txt", Added: []string{"hello"}},
					{Path: "with space.txt", Added: []string{"spaced"}},
				}, got)
			})

			t.Run("file content at a commit", func(t *testing.T) {
				content, err := c.FileAt(dir, second, "a.txt")
				require.NoError(t, err)
				assert.Equal(t, "one\nTWO\nthree\nfour", string(content))

				content, err = c.FileAt(dir, second, "gone.txt")
				require.NoError(t, err)
				assert.Nil(t, content, "a file the commit does not hold has no content")

				content, err = c.FileAt(dir, "HEAD", "new.txt")
				require.NoError(t, err)
				assert.Equal(t, "hello\n", string(content), "a revision name resolves")

				_, err = c.FileAt(dir, "deadbeef00000000000000000000000000000000", "a.txt")
				assert.Error(t, err, "an unknown commit is an error, not an absent file")
			})

			t.Run("an unknown commit is an error", func(t *testing.T) {
				_, err := c.CommitLines(dir, "deadbeef00000000000000000000000000000000")
				assert.Error(t, err)
			})
		})
	}
}

func sortFileLines(fl []FileLines) {
	sort.Slice(fl, func(i, j int) bool { return fl[i].Path < fl[j].Path })
}
