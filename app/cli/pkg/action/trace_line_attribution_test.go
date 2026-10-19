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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below follow the cases of spec issue-3639 through the real git
// hooks: an agent edit goes through recordFileEdit, as the agent hook does,
// and each commit through the commit-msg and post-commit handlers.

const lineSession = "sess-lines"

// lineRepo is a repository with trace state, driven like a user and an agent
// would drive it.
type lineRepo struct {
	t     *testing.T
	dir   string
	store *state.Store
}

func newLineRepo(t *testing.T) *lineRepo {
	t.Helper()
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	return &lineRepo{t: t, dir: dir, store: store}
}

// numbered returns n lines named prefix1..prefixN, newline terminated.
func numbered(prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = prefix + strings.Repeat("x", i+1)
	}

	return out
}

func content(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

// agentWrite writes a file as an agent edit of sessionID.
func (r *lineRepo) agentWrite(sessionID, file, text string) {
	r.t.Helper()
	path := filepath.Join(r.dir, file)
	before, err := os.ReadFile(path)
	if err != nil {
		before = nil
	}
	require.NoError(r.t, os.WriteFile(path, []byte(text), 0600))
	recordFileEdit(r.store, sessionID, file, before, []byte(text), zerolog.Nop())
}

// humanWrite writes a file outside any agent.
func (r *lineRepo) humanWrite(file, text string) {
	r.t.Helper()
	require.NoError(r.t, os.WriteFile(filepath.Join(r.dir, file), []byte(text), 0600))
}

// commit stages files and commits them through the trace hooks. It returns
// the commit message, trailer included.
func (r *lineRepo) commit(subject string, files ...string) string {
	r.t.Helper()
	msg := filepath.Join(r.t.TempDir(), "msg")
	require.NoError(r.t, os.WriteFile(msg, []byte(subject+"\n"), 0600))
	runGit(r.t, r.dir, append([]string{"add"}, files...)...)
	require.NoError(r.t, handleCommitMsg(msg, zerolog.Nop()))
	runGit(r.t, r.dir, "commit", "-F", msg)
	require.NoError(r.t, handlePostCommit(r.t.Context(), zerolog.Nop()))

	got, err := os.ReadFile(msg)
	require.NoError(r.t, err)

	return string(got)
}

// pushed is what a push sends for one session: its code changes and its
// commit list.
type pushed struct {
	changes *aicodingsession.CodeChanges
	commits []string
}

// push runs the push pipeline of the git hook on the recorded commits, and
// marks them tracked as a successful push does. It returns what each session
// sends.
func (r *lineRepo) push() map[string]pushed {
	r.t.Helper()
	records, err := r.store.LoadAllCommitRecords()
	require.NoError(r.t, err)

	client := tracegit.NewGitClient()
	ledgers := loadLedgers(r.store, zerolog.Nop())
	affected, failed := scanReplacements(client, r.store, r.dir, records, ledgers, zerolog.Nop())
	require.Empty(r.t, failed)

	var aiCommits []*state.CommitRecord
	untracked := 0
	for _, c := range records {
		if len(c.AllSessionIDs()) > 0 {
			aiCommits = append(aiCommits, c)
			if !c.Tracked {
				untracked++
			}
		}
	}

	out := make(map[string]pushed)
	if untracked > 0 || len(affected) > 0 {
		cache := newLineCache(client, r.dir, zerolog.Nop())
		for sid, commits := range sessionCommitGroups(aiCommits, nil, false) {
			var shas []string
			for _, c := range commits {
				shas = append(shas, c.SHA)
			}
			cc := sessionCodeChanges(client, r.dir, sid, commits, ledgers, cache, func(string) bool { return false }, zerolog.Nop())
			require.NotNil(r.t, cc)
			out[sid] = pushed{changes: cc, commits: shas}
		}
	}

	for _, c := range records {
		if !c.Tracked {
			c.Tracked = true
			require.NoError(r.t, r.store.SaveCommitRecord(c))
		}
	}

	return out
}

// sessionChanges pushes and returns the code changes that a session sends.
func (r *lineRepo) sessionChanges(sessionID string) *aicodingsession.CodeChanges {
	r.t.Helper()
	got, ok := r.push()[sessionID]
	require.True(r.t, ok, "the push sends session %s", sessionID)

	return got.changes
}

type lineSplit struct{ aiAdded, aiRemoved, humanAdded, humanRemoved int }

func fileSplitOf(t *testing.T, cc *aicodingsession.CodeChanges, path string) lineSplit {
	t.Helper()
	for _, f := range cc.Files {
		if f.Path == path {
			return lineSplit{f.AILinesAdded, f.AILinesRemoved, f.HumanLinesAdded, f.HumanLinesRemoved}
		}
	}
	require.Failf(t, "file not in the code changes", "%s", path)

	return lineSplit{}
}

func totalsOf(cc *aicodingsession.CodeChanges) lineSplit {
	return lineSplit{cc.AILinesAdded, cc.AILinesRemoved, cc.HumanLinesAdded, cc.HumanLinesRemoved}
}

// Case 1 of the spec: the AI writes the lines, a human changes some of them,
// and one commit holds the result (R-001).
func TestLineAttribution_HumanChangeBeforeCommit(t *testing.T) {
	r := newLineRepo(t)

	lines := numbered("ai", 10)
	r.agentWrite(lineSession, "a.go", content(lines))
	lines[2], lines[7] = "human edit one", "human edit two"
	r.humanWrite("a.go", content(lines))

	assert.Contains(t, r.commit("feat: a", "a.go"), "Chainloop-Trace-Sessions: "+lineSession)

	cc := r.sessionChanges(lineSession)
	want := lineSplit{aiAdded: 8, humanAdded: 2}
	assert.Equal(t, want, fileSplitOf(t, cc, "a.go"))
	assert.Equal(t, want, totalsOf(cc))

	assert.NotContains(t, r.store.LoadAILineAttribution(lineSession).Pending, "a.go",
		"a fully committed file leaves nothing pending")
}

// R-002: a human change between two agent edits stays human, also when the
// next agent edit keeps it.
func TestLineAttribution_HumanChangeBetweenAgentEdits(t *testing.T) {
	r := newLineRepo(t)

	r.agentWrite(lineSession, "a.go", "one\ntwo\nthree\n")
	r.humanWrite("a.go", "one\nTWO by hand\nthree\n")
	r.agentWrite(lineSession, "a.go", "one\nTWO by hand\nthree\nfour\n")

	open := r.store.LoadAILineAttribution(lineSession).Open["a.go"]
	assert.Equal(t, 3, open.Added.Len(), "the replaced line closed before the second edit")

	r.commit("feat: a", "a.go")
	assert.Equal(t, lineSplit{aiAdded: 3, humanAdded: 1}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
}

// Case 3 of the spec: a later commit of the same session replaces AI lines by
// hand. Each line counts once (R-003), and the removed AI lines are human
// removals (R-005).
func TestLineAttribution_Case3SameSessionLaterCommit(t *testing.T) {
	r := newLineRepo(t)

	lines := numbered("ai", 5)
	r.agentWrite(lineSession, "a.go", content(lines))
	r.commit("feat: a", "a.go")

	lines[1] = "human"
	r.humanWrite("a.go", content(lines))
	r.agentWrite(lineSession, "b.go", "package b\n")
	assert.Contains(t, r.commit("feat: b", "a.go", "b.go"), "Chainloop-Trace-Sessions: "+lineSession)

	assert.Equal(t, lineSplit{aiAdded: 4, humanAdded: 1, humanRemoved: 1}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
}

// Cases 2 and 5 of the spec: a human commit in no session replaces AI lines
// after the session's push. The next push sends the session again (R-004),
// and the commit joins it with the human lines of its replacing hunks only
// (R-010).
func TestLineAttribution_Case2HumanCommitJoinsSession(t *testing.T) {
	r := newLineRepo(t)

	lines := numbered("ai", 5)
	r.agentWrite(lineSession, "a.go", content(lines))
	r.commit("feat: a", "a.go")
	first := r.push()
	require.Contains(t, first, lineSession)
	assert.Equal(t, lineSplit{aiAdded: 5}, fileSplitOf(t, first[lineSession].changes, "a.go"))

	// The human replaces one AI line and also edits an unrelated file.
	lines[1] = "human"
	r.humanWrite("a.go", content(lines))
	r.humanWrite("other.go", "package other\n")
	assert.NotContains(t, r.commit("fix: by hand", "a.go", "other.go"), "Chainloop-Trace-Sessions")
	human := headSHA(t, r.dir)

	second := r.push()
	require.Contains(t, second, lineSession, "the session is sent again")
	got := second[lineSession]
	assert.Equal(t, lineSplit{aiAdded: 4, humanAdded: 1}, fileSplitOf(t, got.changes, "a.go"))
	assert.Contains(t, got.commits, human, "the commit list holds the joined commit")
	for _, f := range got.changes.Files {
		assert.NotEqual(t, "other.go", f.Path, "the unrelated file of the joined commit is not the session's")
	}

	// A third push with nothing new sends nothing.
	assert.Empty(t, r.push())
}

// A human commit that replaces the lines of an earlier human commit joins the
// sessions that the earlier commit joined, in whatever order the records are
// read.
func TestLineAttribution_ChainedHumanCommitsJoin(t *testing.T) {
	r := newLineRepo(t)

	lines := numbered("ai", 5)
	r.agentWrite(lineSession, "a.go", content(lines))
	r.commit("feat: a", "a.go")
	r.push()

	lines[1] = "human one"
	r.humanWrite("a.go", content(lines))
	r.commit("fix: one", "a.go")
	first := headSHA(t, r.dir)

	lines[1] = "human two"
	r.humanWrite("a.go", content(lines))
	r.commit("fix: two", "a.go")
	second := headSHA(t, r.dir)

	got := r.push()
	require.Contains(t, got, lineSession)
	assert.Equal(t, lineSplit{aiAdded: 4, humanAdded: 1}, fileSplitOf(t, got[lineSession].changes, "a.go"))
	assert.Contains(t, got[lineSession].commits, first)
	assert.Contains(t, got[lineSession].commits, second, "the second commit joins through the first one")
}

// failingLinesClient fails to read the lines of one commit.
type failingLinesClient struct {
	tracegit.Client
	sha string
}

func (c failingLinesClient) CommitLines(repoRoot, sha string) ([]tracegit.FileLines, error) {
	if sha == c.sha {
		return nil, errors.New("read failed")
	}

	return c.Client.CommitLines(repoRoot, sha)
}

// A commit that the scan cannot read is reported, so that the push keeps it
// untracked and reads it again.
func TestScanReplacements_ReportsFailedCommits(t *testing.T) {
	r := newLineRepo(t)

	r.agentWrite(lineSession, "a.go", content(numbered("ai", 3)))
	r.commit("feat: a", "a.go")
	r.humanWrite("a.go", "changed\n")
	r.commit("fix: by hand", "a.go")
	human := headSHA(t, r.dir)

	records, err := r.store.LoadAllCommitRecords()
	require.NoError(t, err)
	client := failingLinesClient{Client: tracegit.NewGitClient(), sha: human}
	_, failed := scanReplacements(client, r.store, r.dir, records, loadLedgers(r.store, zerolog.Nop()), zerolog.Nop())

	assert.Equal(t, map[string]bool{human: true}, failed)
}

// Case 4 of the spec: a commit of another session replaces lines of the first
// session. The first session is sent again with what is still there (R-004),
// and the commit does not join it.
func TestLineAttribution_Case4OtherSessionReplaces(t *testing.T) {
	r := newLineRepo(t)
	const other = "sess-other"

	lines := numbered("ai", 5)
	r.agentWrite(lineSession, "a.go", content(lines))
	r.commit("feat: a", "a.go")
	r.push()

	lines[1] = "human in b"
	r.humanWrite("a.go", content(lines))
	r.agentWrite(other, "b.go", "package b\n")
	assert.Contains(t, r.commit("feat: b", "a.go", "b.go"), "Chainloop-Trace-Sessions: "+other)
	commitB := headSHA(t, r.dir)

	got := r.push()
	require.Contains(t, got, lineSession, "the replaced session is sent again")
	assert.Equal(t, lineSplit{aiAdded: 4}, fileSplitOf(t, got[lineSession].changes, "a.go"))
	assert.NotContains(t, got[lineSession].commits, commitB, "a commit of another session does not join")

	require.Contains(t, got, other)
	assert.Equal(t, lineSplit{humanAdded: 1, humanRemoved: 1}, fileSplitOf(t, got[other].changes, "a.go"))
}

func rangesOf(t *testing.T, cc *aicodingsession.CodeChanges, path string) []aicodingsession.LineRange {
	t.Helper()
	for _, f := range cc.Files {
		if f.Path == path {
			return f.LineRanges
		}
	}
	require.Failf(t, "file not in the code changes", "%s", path)

	return nil
}

func aiRange(start, end int) aicodingsession.LineRange {
	return aicodingsession.LineRange{Start: start, End: end, Attribution: trace.AttributionAI}
}

func humanRange(start, end int) aicodingsession.LineRange {
	return aicodingsession.LineRange{Start: start, End: end, Attribution: trace.AttributionHuman}
}

// Spec issue-3639 line ranges R-001 and R-004: the AI writes 100 lines, and a
// later commit replaces 20 of them by hand. Each push sends ranges at the
// positions of the pushed commit, with their owner.
func TestLineAttribution_RangesPointIntoPushedCommit(t *testing.T) {
	testCases := []struct {
		name string
		// session is the session of the later commit, empty for none.
		session string
	}{
		{name: "later commit in no session (cases 2 and 5)"},
		{name: "later commit in the same session (case 3)", session: lineSession},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := newLineRepo(t)

			lines := numbered("ai", 100)
			r.agentWrite(lineSession, "a.go", content(lines))
			r.commit("feat: a", "a.go")
			first := r.push()
			require.Contains(t, first, lineSession)
			assert.Equal(t, []aicodingsession.LineRange{aiRange(1, 100)}, rangesOf(t, first[lineSession].changes, "a.go"))

			for i := 40; i < 60; i++ {
				lines[i] = "human " + lines[i]
			}
			r.humanWrite("a.go", content(lines))
			files := []string{"a.go"}
			if tc.session != "" {
				r.agentWrite(tc.session, "b.go", "package b\n")
				files = append(files, "b.go")
			}
			r.commit("fix: by hand", files...)

			second := r.push()
			require.Contains(t, second, lineSession, "the session is sent again")
			cc := second[lineSession].changes
			split := fileSplitOf(t, cc, "a.go")
			assert.Equal(t, 80, split.aiAdded)
			assert.Equal(t, 20, split.humanAdded)
			assert.Equal(t, []aicodingsession.LineRange{aiRange(1, 40), humanRange(41, 60), aiRange(61, 100)}, rangesOf(t, cc, "a.go"))
		})
	}
}

// Spec issue-3639 line ranges R-002: in case 4, each session gets ranges only
// for its own lines.
func TestLineAttribution_RangesOfTwoSessions(t *testing.T) {
	r := newLineRepo(t)
	const other = "sess-other"

	lines := numbered("ai", 5)
	r.agentWrite(lineSession, "a.go", content(lines))
	r.commit("feat: a", "a.go")
	r.push()

	lines[1] = "human in b"
	r.humanWrite("a.go", content(lines))
	r.agentWrite(other, "b.go", "package b\n")
	r.commit("feat: b", "a.go", "b.go")

	got := r.push()
	require.Contains(t, got, lineSession)
	require.Contains(t, got, other)
	assert.Equal(t, []aicodingsession.LineRange{aiRange(1, 1), aiRange(3, 5)}, rangesOf(t, got[lineSession].changes, "a.go"))
	assert.Equal(t, []aicodingsession.LineRange{humanRange(2, 2)}, rangesOf(t, got[other].changes, "a.go"))
	assert.Equal(t, []aicodingsession.LineRange{aiRange(1, 1)}, rangesOf(t, got[other].changes, "b.go"))
}

// R-005: lines that the agent removed are AI removals.
func TestLineAttribution_AgentRemovalIsAI(t *testing.T) {
	r := newLineRepo(t)

	r.humanWrite("a.go", "keep\ndrop one\ndrop two\n")
	r.commit("chore: base", "a.go")

	r.agentWrite(lineSession, "a.go", "keep\n")
	r.commit("feat: trim", "a.go")

	assert.Equal(t, lineSplit{aiRemoved: 2}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
}

// R-006: a commit with only part of the AI lines of a file keeps the rest
// pending, and the next commit with them is credited to the session.
func TestLineAttribution_PartialCommitOfAFile(t *testing.T) {
	r := newLineRepo(t)

	lines := numbered("ai", 4)
	r.agentWrite(lineSession, "a.go", content(lines))

	// Stage only the first two lines, as `git add -p` would.
	r.humanWrite("a.go", content(lines[:2]))
	runGit(t, r.dir, "add", "a.go")
	r.humanWrite("a.go", content(lines))
	msg := filepath.Join(t.TempDir(), "msg")
	require.NoError(t, os.WriteFile(msg, []byte("feat: first half\n"), 0600))
	require.NoError(t, handleCommitMsg(msg, zerolog.Nop()))
	runGit(t, r.dir, "commit", "-F", msg)
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	attr := r.store.LoadAILineAttribution(lineSession)
	require.Contains(t, attr.Pending, "a.go", "the unstaged AI lines keep the file pending")
	assert.Equal(t, 2, attr.Open["a.go"].Added.Len())

	assert.Contains(t, r.commit("feat: second half", "a.go"), "Chainloop-Trace-Sessions: "+lineSession)
	assert.Equal(t, lineSplit{aiAdded: 4}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
}

// A file whose AI lines a human replaced before the commit is settled by the
// commit, so a later human commit of the file is not credited to the session.
func TestLineAttribution_ReplacedLinesDoNotCreditLaterCommits(t *testing.T) {
	r := newLineRepo(t)

	r.agentWrite(lineSession, "a.go", "one\ntwo\n")
	r.agentWrite(lineSession, "b.go", "package b\n")
	// The human rewrites a.go fully and commits b.go only.
	r.humanWrite("a.go", "uno\ndos\n")
	r.commit("feat: b", "b.go")

	assert.Contains(t, r.store.LoadAILineAttribution(lineSession).Pending, "a.go",
		"a.go is not committed yet, so post-commit does not look at it")

	msg := r.commit("chore: a by hand", "a.go")
	assert.Contains(t, msg, "Chainloop-Trace-Sessions: "+lineSession,
		"the pending file still matches at the file level when it is committed")
	assert.Equal(t, lineSplit{humanAdded: 2}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
	assert.NotContains(t, r.store.LoadAILineAttribution(lineSession).Pending, "a.go")

	r.humanWrite("a.go", "uno\ndos\ntres\n")
	assert.NotContains(t, r.commit("chore: more", "a.go"), "Chainloop-Trace-Sessions")
}

// A cherry-pick, or a rebase, replays a commit with a new SHA. The lines
// still match the ledger by content, so the AI split survives.
func TestLineAttribution_SurvivesCherryPick(t *testing.T) {
	r := newLineRepo(t)

	runGit(t, r.dir, "checkout", "-b", "feature")
	r.agentWrite(lineSession, "a.go", "one\ntwo\nthree\n")
	r.humanWrite("a.go", "one\ntwo\nthree\nby hand\n")
	r.commit("feat: a", "a.go")
	picked := headSHA(t, r.dir)

	// main moves on, so the pick gets a new parent and a new SHA.
	runGit(t, r.dir, "checkout", "main")
	r.humanWrite("upstream.txt", "upstream\n")
	r.commit("chore: upstream", "upstream.txt")
	runGit(t, r.dir, "checkout", "-b", "rebased")
	runGit(t, r.dir, "cherry-pick", picked)
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))
	require.NotEqual(t, picked, headSHA(t, r.dir))

	// Drop the feature branch and its record, as the post-push GC does.
	runGit(t, r.dir, "branch", "-D", "feature")
	live, err := tracegit.NewGitClient().LocalReachableSHAs(r.dir)
	require.NoError(t, err)
	require.NoError(t, r.store.GCOrphans(live))

	assert.Equal(t, lineSplit{aiAdded: 3, humanAdded: 1}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
}

// shellWrite runs a "shell command" of the session: the pre hook captures the
// worktree, run changes it, and the post hook records what changed.
func (r *lineRepo) shellWrite(sessionID string, run func()) {
	r.t.Helper()
	key := state.ShellCallKey{SessionID: sessionID, ToolUseID: "toolu"}
	captureWorktreeSnapshot(r.store, r.dir, key, zerolog.Nop())
	run()

	sigs, err := r.store.LoadShellPreSignature(key)
	require.NoError(r.t, err)
	heads, err := r.store.LoadShellPreHeads(key)
	require.NoError(r.t, err)
	base := heads[r.dir]
	require.NotEmpty(r.t, base, "the pre hook saves the HEAD commit")
	recordRootChanges(r.store, r.dir, base, sigs[r.dir], sessionID, zerolog.Nop())
	r.store.DeleteShellPreSignature(key)
}

// A shell command is diffed against HEAD, so its diff holds again the
// uncommitted lines of earlier edits. They are not recorded twice.
func TestLineAttribution_ShellDoesNotRecordOpenLinesAgain(t *testing.T) {
	r := newLineRepo(t)

	r.shellWrite(lineSession, func() { r.humanWrite("a.go", "one\ntwo\n") })
	r.shellWrite(lineSession, func() { r.humanWrite("a.go", "one\ntwo\nthree\n") })

	attr := r.store.LoadAILineAttribution(lineSession)
	assert.Equal(t, 3, attr.Written["a.go"].Len())
	assert.Equal(t, 3, attr.Open["a.go"].Added.Len())
	assert.Equal(t, 3, attr.GeneratedAdded)

	r.commit("feat: a", "a.go")
	assert.Equal(t, lineSplit{aiAdded: 3}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
}

// A shell command that writes a file and commits it diffs against the HEAD
// from before the command, so its lines are still recorded.
func TestLineAttribution_ShellCommandThatCommits(t *testing.T) {
	r := newLineRepo(t)

	r.shellWrite(lineSession, func() {
		r.humanWrite("a.go", "one\ntwo\n")
		runGit(t, r.dir, "add", "a.go")
		runGit(t, r.dir, "commit", "-m", "feat: a")
	})

	attr := r.store.LoadAILineAttribution(lineSession)
	assert.Equal(t, 2, attr.Written["a.go"].Len())
}

// R-011: the ledger keeps the lines that every agent edit added and removed,
// also the ones that never reach a commit.
func TestLineAttribution_GeneratedLines(t *testing.T) {
	r := newLineRepo(t)

	r.agentWrite(lineSession, "a.go", "one\ntwo\nthree\n")
	r.agentWrite(lineSession, "a.go", "one\n")
	r.agentWrite(lineSession, "scratch.go", "tmp\n")
	require.NoError(t, os.Remove(filepath.Join(r.dir, "scratch.go")))

	attr := r.store.LoadAILineAttribution(lineSession)
	assert.Equal(t, 4, attr.GeneratedAdded)
	assert.Equal(t, 2, attr.GeneratedRemoved)
}
