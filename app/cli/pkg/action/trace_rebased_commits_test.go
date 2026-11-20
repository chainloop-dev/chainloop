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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	knownSession   = "11111111-1111-4111-8111-111111111111"
	unknownSession = "22222222-2222-4222-8222-222222222222"
	humanSubject   = "human commit"
)

// rebasedBranch is a feature branch whose commits were rebased without the
// hooks, as the GitHub "Update branch" button does.
type rebasedBranch struct {
	dir   string
	store *state.Store
	// oldSHAs are the SHAs that post-commit recorded before the rebase.
	oldSHAs []string
	// shaBySubject maps the subject of each commit to its SHA after the rebase.
	shaBySubject map[string]string
}

// setupRebasedBranch builds a branch with two recorded commits of
// knownSession, then rebases it onto a new main without running any hook. It
// also adds, before the rebase, commits with a trailer that post-commit never
// recorded: one for an unknown session, one for both sessions, and a merge.
func setupRebasedBranch(t *testing.T) *rebasedBranch {
	t.Helper()

	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: knownSession, StartedAt: state.NowTimestamp(),
	}))
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")

	commit := func(file, subject, trailer string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(subject+"\n"), 0600))
		runGit(t, dir, "add", file)
		msg := subject
		if trailer != "" {
			msg += "\n\n" + state.TrailerKey + ": " + trailer + "\n"
		}
		runGit(t, dir, "commit", "-m", msg)
	}

	runGit(t, dir, "checkout", "-b", "feature")
	oldSHAs := make([]string, 0, 2)
	for _, f := range []string{"a", "b"} {
		commit(f+".txt", "ai "+f, knownSession)
		require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))
		oldSHAs = append(oldSHAs, headSHA(t, dir))
	}
	commit("c.txt", "unknown session", unknownSession)
	commit("d.txt", "mixed sessions", knownSession+", "+unknownSession)
	commit("e.txt", humanSubject, "")

	// Upstream moves, then the branch is rebased with no hook.
	runGit(t, dir, "checkout", "main")
	commit("upstream.txt", "upstream", "")
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	runGit(t, dir, "checkout", "feature")
	runGit(t, dir, "-c", "core.hooksPath=/dev/null", "rebase", "main")

	return &rebasedBranch{dir: dir, store: store, oldSHAs: oldSHAs, shaBySubject: branchSubjects(t, dir)}
}

// branchSubjects returns the SHA of each commit on HEAD that is not on main,
// by its subject.
func branchSubjects(t *testing.T, dir string) map[string]string {
	t.Helper()
	cmd := exec.Command("git", "log", "--format=%H %s", "main..HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)

	got := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		sha, subject, _ := strings.Cut(line, " ")
		got[subject] = sha
	}

	return got
}

func recordsBySHA(records []*state.CommitRecord) map[string]*state.CommitRecord {
	out := make(map[string]*state.CommitRecord, len(records))
	for _, r := range records {
		out[r.SHA] = r
	}

	return out
}

func TestRecordRebasedCommits(t *testing.T) {
	b := setupRebasedBranch(t)

	records, err := b.store.LoadAllCommitRecords()
	require.NoError(t, err)
	sessions, err := b.store.LoadAllSessionRecords()
	require.NoError(t, err)

	got := recordsBySHA(recordRebasedCommits(tracegit.NewGitClient(), b.store, b.dir, records, sessions, zerolog.Nop()))

	testCases := []struct {
		subject      string
		wantRecorded bool
		wantSessions []string
	}{
		{subject: "ai a", wantRecorded: true, wantSessions: []string{knownSession}},
		{subject: "ai b", wantRecorded: true, wantSessions: []string{knownSession}},
		{subject: "unknown session", wantRecorded: false},
		{subject: "mixed sessions", wantRecorded: true, wantSessions: []string{knownSession}},
		{subject: humanSubject, wantRecorded: false},
	}
	for _, tc := range testCases {
		t.Run(tc.subject, func(t *testing.T) {
			sha := b.shaBySubject[tc.subject]
			require.NotEmpty(t, sha)

			rec, ok := got[sha]
			assert.Equal(t, tc.wantRecorded, ok)
			if !tc.wantRecorded {
				return
			}
			assert.Equal(t, tc.wantSessions, rec.SessionIDs)
			assert.False(t, rec.Tracked)
			assert.Equal(t, tc.wantSessions, mustLoadRecord(t, b.store, sha).SessionIDs, "record must be saved")
		})
	}

	t.Run("keeps the existing records", func(t *testing.T) {
		for _, sha := range b.oldSHAs {
			assert.Contains(t, got, sha)
		}
	})

	t.Run("does not overwrite an existing record", func(t *testing.T) {
		sha := b.shaBySubject["ai a"]
		existing := &state.CommitRecord{SHA: sha, Message: "kept", SessionIDs: []string{knownSession}, Tracked: true, Timestamp: state.NowTimestamp()}
		require.NoError(t, b.store.SaveCommitRecord(existing))

		all, err := b.store.LoadAllCommitRecords()
		require.NoError(t, err)
		again := recordsBySHA(recordRebasedCommits(tracegit.NewGitClient(), b.store, b.dir, all, sessions, zerolog.Nop()))
		assert.True(t, again[sha].Tracked)
		assert.Equal(t, "kept", mustLoadRecord(t, b.store, sha).Message)
	})
}

func TestRecordRebasedCommits_SkipsMergeCommits(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{SessionID: knownSession, StartedAt: state.NowTimestamp()}))
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")

	runGit(t, dir, "checkout", "-b", "side")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "side.txt"), []byte("side\n"), 0600))
	runGit(t, dir, "add", "side.txt")
	runGit(t, dir, "commit", "-m", "side")
	runGit(t, dir, "checkout", "-b", "feature", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), []byte("f\n"), 0600))
	runGit(t, dir, "add", "f.txt")
	runGit(t, dir, "commit", "-m", "feature")
	runGit(t, dir, "merge", "--no-ff", "side", "-m", "merge\n\n"+state.TrailerKey+": "+knownSession+"\n")
	mergeSHA := headSHA(t, dir)

	got := recordsBySHA(recordRebasedCommits(tracegit.NewGitClient(), store, dir, nil, map[string]*state.SessionRecord{knownSession: {SessionID: knownSession}}, zerolog.Nop()))
	assert.NotContains(t, got, mergeSHA)
}

// TestGatherPushSessions_AfterRebaseOutsideClone covers the acceptance
// criteria of issue 3838: after a rebase that ran without the hooks, the next
// push sends the session with the rebased SHAs, with or without a new commit.
func TestGatherPushSessions_AfterRebaseOutsideClone(t *testing.T) {
	testCases := []struct {
		name      string
		newCommit bool
	}{
		{name: "push with no new commit"},
		{name: "push with one new commit", newCommit: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := setupRebasedBranch(t)
			want := []string{b.shaBySubject["ai a"], b.shaBySubject["ai b"], b.shaBySubject["mixed sessions"]}

			if tc.newCommit {
				require.NoError(t, os.WriteFile(filepath.Join(b.dir, "new.txt"), []byte("new\n"), 0600))
				runGit(t, b.dir, "add", "new.txt")
				runGit(t, b.dir, "commit", "-m", "new\n\n"+state.TrailerKey+": "+knownSession+"\n")
				require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))
				want = append(want, headSHA(t, b.dir))
			}

			_, commits, err := gatherPushSessions(t.Context(), b.store, b.dir, "", false, zerolog.Nop())
			require.NoError(t, err)

			var got []string
			for _, c := range commits {
				if len(c.SessionIDs) > 0 {
					got = append(got, c.SHA)
				}
			}
			assert.ElementsMatch(t, want, got)
			for _, sha := range b.oldSHAs {
				assert.NotContains(t, got, sha, "old SHAs are no longer on the branch")
			}
		})
	}
}
