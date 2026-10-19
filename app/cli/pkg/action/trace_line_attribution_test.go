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
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
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

// sessionChanges runs the push pipeline for one session over the commits that
// name it, and returns its code changes.
func (r *lineRepo) sessionChanges(sessionID string) *aicodingsession.CodeChanges {
	r.t.Helper()
	records, err := r.store.LoadAllCommitRecords()
	require.NoError(r.t, err)
	commits := sessionCommitGroups(records, nil, false)[sessionID]

	shas := make([]string, 0, len(commits))
	for _, c := range commits {
		shas = append(shas, c.SHA)
	}

	client := tracegit.NewGoGitClient()
	cc, err := client.CodeChangesForCommits(r.dir, shas)
	require.NoError(r.t, err)

	attrs, err := r.store.LoadAllAILineAttributions()
	require.NoError(r.t, err)
	ledgers := make(map[string]*state.AILineAttribution, len(attrs))
	for _, a := range attrs {
		ledgers[a.SessionID] = a
	}
	attribution.Enrich(sessionID, ledgers, sessionCommitLines(client, r.dir, commits, map[string][]tracegit.FileLines{}, zerolog.Nop()), cc)

	return cc
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

// Case 3 of the spec, as far as milestone 1 goes: a later commit of the same
// session replaces AI lines by hand. The removed AI lines are human removals
// (R-005). The net numbers (R-003) come with milestone 2.
func TestLineAttribution_HumanRemovalIsHuman(t *testing.T) {
	r := newLineRepo(t)

	lines := numbered("ai", 5)
	r.agentWrite(lineSession, "a.go", content(lines))
	r.commit("feat: a", "a.go")

	lines[1] = "human"
	r.humanWrite("a.go", content(lines))
	r.agentWrite(lineSession, "b.go", "package b\n")
	assert.Contains(t, r.commit("feat: b", "a.go", "b.go"), "Chainloop-Trace-Sessions: "+lineSession)

	assert.Equal(t, lineSplit{aiAdded: 5, humanAdded: 1, humanRemoved: 1}, fileSplitOf(t, r.sessionChanges(lineSession), "a.go"))
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
	live, err := tracegit.NewGoGitClient().LocalReachableSHAs(r.dir)
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
	base := r.store.LoadShellPreHeads(key)[r.dir]
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
