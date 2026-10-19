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

package action

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below follow R-007 of spec issue-3639: the AI/human split
// survives a rename, an amend, a rebase, and a squash in the same clone.

// rewrite runs a git command that rewrites commits, and then the hooks that
// git runs for it: commit-msg and post-commit for the new HEAD commit, and
// post-rewrite with the old commits that it replaced.
func (r *lineRepo) rewrite(subject string, olds []string, args ...string) {
	r.t.Helper()
	msg := filepath.Join(r.t.TempDir(), "msg")
	require.NoError(r.t, os.WriteFile(msg, []byte(subject+"\n"), 0600))
	require.NoError(r.t, handleCommitMsg(msg, zerolog.Nop()))
	runGit(r.t, r.dir, append(args, "-F", msg)...)
	require.NoError(r.t, handlePostCommit(r.t.Context(), zerolog.Nop()))

	head := headSHA(r.t, r.dir)
	var pairs strings.Builder
	for _, old := range olds {
		pairs.WriteString(old + " " + head + "\n")
	}
	require.NoError(r.t, handlePostRewrite(strings.NewReader(pairs.String()), zerolog.Nop()))
}

// An amend that replaces the message drops the trailer. The split stays, also
// for a human line that the amend adds.
func TestLineAttribution_SurvivesAmend(t *testing.T) {
	r := newLineRepo(t)

	r.agentWrite(lineSession, "a.go", "one\ntwo\nthree\n")
	r.commit("feat: a", "a.go")
	old := headSHA(t, r.dir)

	r.humanWrite("a.go", "one\ntwo\nthree\nby hand\n")
	runGit(t, r.dir, "add", "a.go")
	r.rewrite("feat: a, reworded", []string{old}, "commit", "--amend")

	got := r.push()
	require.Contains(t, got, lineSession)
	assert.Equal(t, lineSplit{aiAdded: 3, humanAdded: 1}, fileSplitOf(t, got[lineSession].changes, "a.go"))
	assert.Equal(t, []string{headSHA(t, r.dir)}, got[lineSession].commits, "the old commit is gone")
}

// A rebase onto a branch that moved on replays the commits of the session.
func TestLineAttribution_SurvivesRebase(t *testing.T) {
	r := newLineRepo(t)

	runGit(t, r.dir, "checkout", "-b", "feature")
	r.agentWrite(lineSession, "a.go", "one\ntwo\nthree\n")
	r.humanWrite("a.go", "one\ntwo\nthree\nby hand\n")
	r.commit("feat: a", "a.go")
	old := headSHA(t, r.dir)

	runGit(t, r.dir, "checkout", "main")
	r.humanWrite("upstream.txt", "upstream\n")
	r.commit("chore: upstream", "upstream.txt")
	runGit(t, r.dir, "checkout", "feature")
	runGit(t, r.dir, "rebase", "main")
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))
	require.NoError(t, handlePostRewrite(strings.NewReader(old+" "+headSHA(t, r.dir)+"\n"), zerolog.Nop()))

	assert.Equal(t, lineSplit{aiAdded: 3, humanAdded: 1}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
}

// A squash of two commits of the session keeps the split of both.
func TestLineAttribution_SurvivesSquash(t *testing.T) {
	r := newLineRepo(t)

	r.agentWrite(lineSession, "a.go", "one\ntwo\nthree\n")
	r.commit("feat: a", "a.go")
	first := headSHA(t, r.dir)

	r.agentWrite(lineSession, "b.go", "four\nfive\n")
	r.humanWrite("a.go", "one\nTWO\nthree\n")
	r.commit("feat: b", "a.go", "b.go")
	second := headSHA(t, r.dir)

	runGit(t, r.dir, "reset", "--soft", "HEAD~2")
	r.rewrite("feat: a and b", []string{first, second}, "commit")

	got := r.push()
	require.Contains(t, got, lineSession)
	assert.Equal(t, lineSplit{aiAdded: 2, humanAdded: 1}, fileSplitOf(t, got[lineSession].changes, "a.go"))
	assert.Equal(t, lineSplit{aiAdded: 2}, fileSplitOf(t, got[lineSession].changes, "b.go"))
}

// A human rename of a file of the session, before the push, keeps the AI
// lines of the file.
func TestLineAttribution_RenameBeforePush(t *testing.T) {
	r := newLineRepo(t)

	r.agentWrite(lineSession, "a.go", content(numbered("ai", 5)))
	r.commit("feat: a", "a.go")
	runGit(t, r.dir, "mv", "a.go", "b.go")
	r.commit("refactor: rename", "b.go")

	assert.Equal(t, lineSplit{aiAdded: 5}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
}

// A human change to a renamed file of the session, after the push, replaces
// the AI lines and joins the session, as it does without the rename (R-004,
// R-010).
func TestLineAttribution_ChangeAfterRename(t *testing.T) {
	testCases := []struct {
		name string
		// sameCommit renames and changes the file in one commit.
		sameCommit bool
		// pushRename pushes the rename before the change.
		pushRename bool
	}{
		{name: "rename and change in two commits"},
		{name: "rename and change in one commit", sameCommit: true},
		{name: "rename pushed before the change", pushRename: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := newLineRepo(t)

			lines := numbered("ai", 5)
			r.agentWrite(lineSession, "a.go", content(lines))
			r.commit("feat: a", "a.go")
			r.push()

			runGit(t, r.dir, "mv", "a.go", "b.go")
			if !tc.sameCommit {
				r.commit("refactor: rename", "b.go")
			}
			if tc.pushRename {
				assert.Empty(t, r.push(), "a rename alone changes no line")
			}
			lines[1] = "by hand"
			r.humanWrite("b.go", content(lines))
			r.commit("fix: by hand", "b.go")
			human := headSHA(t, r.dir)

			got := r.push()
			require.Contains(t, got, lineSession, "the session is sent again")
			assert.Equal(t, lineSplit{aiAdded: 4, humanAdded: 1}, fileSplitOf(t, got[lineSession].changes, "a.go"))
			assert.Contains(t, got[lineSession].commits, human)
		})
	}
}

// An agent that renames a file of a human and changes one line of it wrote
// that one line only, and removed the line that it replaced.
func TestLineAttribution_AgentRenamesHumanFile(t *testing.T) {
	r := newLineRepo(t)

	lines := numbered("h", 5)
	r.humanWrite("h.go", content(lines))
	r.commit("feat: h", "h.go")

	r.shellWrite(lineSession, func() {
		runGit(t, r.dir, "mv", "h.go", "g.go")
		lines[2] = "agent"
		r.humanWrite("g.go", content(lines))
	})
	r.commit("refactor: g", "g.go")

	assert.Equal(t, lineSplit{aiAdded: 1, aiRemoved: 1}, fileSplitOf(t, r.sessionChanges(lineSession), "g.go"))
}

// A human commit that replaces a line that an earlier joined commit added
// joins the session, also when a commit renamed the file in between.
func TestLineAttribution_ChainedJoinAfterRename(t *testing.T) {
	r := newLineRepo(t)

	lines := numbered("ai", 5)
	r.agentWrite(lineSession, "a.go", content(lines))
	r.commit("feat: a", "a.go")
	r.push()

	lines[1] = "first by hand"
	r.humanWrite("a.go", content(lines))
	r.commit("fix: first", "a.go")
	require.Contains(t, r.push(), lineSession)

	runGit(t, r.dir, "mv", "a.go", "b.go")
	r.commit("refactor: rename", "b.go")
	r.push()

	lines[1] = "second by hand"
	r.humanWrite("b.go", content(lines))
	r.commit("fix: second", "b.go")
	second := headSHA(t, r.dir)

	got := r.push()
	require.Contains(t, got, lineSession, "the session is sent again")
	assert.Equal(t, lineSplit{aiAdded: 4, humanAdded: 1}, fileSplitOf(t, got[lineSession].changes, "a.go"))
	assert.Contains(t, got[lineSession].commits, second)
}

// failingRenamesClient fails to read the renames.
type failingRenamesClient struct {
	tracegit.Client
}

func (failingRenamesClient) Renames(string, string, string) (map[string]string, error) {
	return nil, errors.New("read failed")
}

// A commit that changes a renamed file, when the renames cannot be read, is
// reported, so that the push keeps it untracked and reads it again.
func TestScanReplacements_ReportsFailedRenames(t *testing.T) {
	r := newLineRepo(t)

	r.agentWrite(lineSession, "a.go", content(numbered("ai", 3)))
	r.commit("feat: a", "a.go")
	r.push()
	runGit(t, r.dir, "mv", "a.go", "b.go")
	r.commit("refactor: rename", "b.go")
	r.push()
	r.humanWrite("b.go", "changed\n")
	r.commit("fix: by hand", "b.go")
	human := headSHA(t, r.dir)

	records, err := r.store.LoadAllCommitRecords()
	require.NoError(t, err)
	client := failingRenamesClient{Client: tracegit.NewGitClient()}
	_, failed := scanReplacements(client, r.store, r.dir, records, loadLedgers(r.store, zerolog.Nop()), zerolog.Nop())

	assert.Equal(t, map[string]bool{human: true}, failed)
}
