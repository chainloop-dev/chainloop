package action

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/attribution"
	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvidenceName(t *testing.T) {
	t.Run("truncates long session ID", func(t *testing.T) {
		name := evidenceName("abcdefghijklmnop")
		assert.Equal(t, "ai-coding-session-abcdef", name)
	})

	t.Run("handles short session ID", func(t *testing.T) {
		name := evidenceName("abc")
		assert.Equal(t, "ai-coding-session-abc", name)
	})
}

func TestMatchSessionsToFiles(t *testing.T) {
	attrs := []*state.AILineAttribution{
		{SessionID: "session-a", Files: map[string][]aicodingsession.LineRange{"src/foo.go": {{Start: 1, End: 10}}}},
		{SessionID: "session-b", Files: map[string][]aicodingsession.LineRange{"src/bar.go": {{Start: 1, End: 5}}}},
		{SessionID: "session-c", Files: map[string][]aicodingsession.LineRange{"src/baz.go": {{Start: 1, End: 3}}}},
	}

	t.Run("matches sessions that touched staged files", func(t *testing.T) {
		ids := matchSessionsToFiles(attrs, []string{"src/foo.go", "src/bar.go"})
		assert.Equal(t, []string{"session-a", "session-b"}, ids)
	})

	t.Run("no match returns nil", func(t *testing.T) {
		ids := matchSessionsToFiles(attrs, []string{"src/other.go"})
		assert.Nil(t, ids)
	})

	t.Run("empty staged files returns nil", func(t *testing.T) {
		ids := matchSessionsToFiles(attrs, nil)
		assert.Nil(t, ids)
	})

	t.Run("empty attrs returns nil", func(t *testing.T) {
		ids := matchSessionsToFiles(nil, []string{"src/foo.go"})
		assert.Nil(t, ids)
	})

	t.Run("result is sorted", func(t *testing.T) {
		ids := matchSessionsToFiles(attrs, []string{"src/baz.go", "src/foo.go"})
		assert.Equal(t, []string{"session-a", "session-c"}, ids)
	})
}

func TestAppendTrailer(t *testing.T) {
	writeMsg := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "COMMIT_EDITMSG")
		require.NoError(t, os.WriteFile(path, []byte(content), 0600))

		return path
	}

	t.Run("appends trailer after blank line", func(t *testing.T) {
		path := writeMsg(t, "fix: resolve login bug\n")
		require.NoError(t, appendTrailer(path, []string{"id-1", "id-2"}))

		got, _ := os.ReadFile(path)
		assert.Equal(t, "fix: resolve login bug\n\nChainloop-Trace-Sessions: id-1, id-2\n", string(got))
	})

	t.Run("handles multi-line messages", func(t *testing.T) {
		path := writeMsg(t, "feat: add auth\n\nThis adds OAuth2 support.\n")
		require.NoError(t, appendTrailer(path, []string{"s1"}))

		got, _ := os.ReadFile(path)
		assert.Contains(t, string(got), "This adds OAuth2 support.\n\nChainloop-Trace-Sessions: s1\n")
	})

	t.Run("skips if trailer already present", func(t *testing.T) {
		original := "fix: bug\n\nChainloop-Trace-Sessions: existing-id\n"
		path := writeMsg(t, original)
		require.NoError(t, appendTrailer(path, []string{"new-id"}))

		got, _ := os.ReadFile(path)
		assert.Equal(t, original, string(got))
	})

	t.Run("single session ID", func(t *testing.T) {
		path := writeMsg(t, "chore: cleanup\n")
		require.NoError(t, appendTrailer(path, []string{"only-one"}))

		got, _ := os.ReadFile(path)
		assert.Contains(t, string(got), "Chainloop-Trace-Sessions: only-one\n")
	})

	t.Run("strips trailing newlines before adding blank line", func(t *testing.T) {
		path := writeMsg(t, "fix: something\n\n\n\n")
		require.NoError(t, appendTrailer(path, []string{"s1"}))

		got, _ := os.ReadFile(path)
		assert.Equal(t, "fix: something\n\nChainloop-Trace-Sessions: s1\n", string(got))
	})
}

// fakeBranchClient is a stub implementation of tracegit.Client for testing
// filterCurrentBranchCommits in isolation.
type fakeBranchClient struct {
	branchSHAs []string
	branchErr  error
	mergeSHAs  map[string]bool
	mergeErr   error
}

func (f *fakeBranchClient) Context(string) (*aicodingsession.GitContext, error) {
	return &aicodingsession.GitContext{}, nil
}

func (f *fakeBranchClient) CodeChanges(string) (*aicodingsession.CodeChanges, error) {
	return &aicodingsession.CodeChanges{}, nil
}

func (f *fakeBranchClient) CodeChangesForRange(string, string, string) (*aicodingsession.CodeChanges, error) {
	return &aicodingsession.CodeChanges{}, nil
}

func (f *fakeBranchClient) CodeChangesForCommits(string, []string) (*aicodingsession.CodeChanges, error) {
	return &aicodingsession.CodeChanges{}, nil
}

func (f *fakeBranchClient) CommitHeadInfo(string) (string, string, error) { return "", "", nil }
func (f *fakeBranchClient) GeneratedMatcher(string) func(string) bool {
	return func(string) bool { return false }
}
func (f *fakeBranchClient) StagedFiles(string) ([]string, error) { return nil, nil }
func (f *fakeBranchClient) SnapshotWorktree(string) (map[string]string, error) {
	return nil, nil
}
func (f *fakeBranchClient) IsMergeCommit(_, sha string) (bool, error) {
	return f.mergeSHAs[sha], f.mergeErr
}

func (f *fakeBranchClient) BranchCommitSHAs(string) ([]string, error) {
	return f.branchSHAs, f.branchErr
}

func (f *fakeBranchClient) LocalReachableSHAs(string) (map[string]bool, error) {
	return nil, nil
}

func TestFilterCurrentBranchCommits(t *testing.T) {
	rec := func(sha string) *state.CommitRecord { return &state.CommitRecord{SHA: sha} }

	t.Run("drops orphans not in branch", func(t *testing.T) {
		client := &fakeBranchClient{branchSHAs: []string{"new1", "new2"}}
		got := filterCurrentBranchCommits(client, "/repo",
			[]*state.CommitRecord{rec("old1"), rec("new1"), rec("new2"), rec("old2")},
			zerolog.Nop(),
		)

		require.Len(t, got, 2)
		assert.Equal(t, "new1", got[0].SHA)
		assert.Equal(t, "new2", got[1].SHA)
	})

	t.Run("drops merge commits even when in branch", func(t *testing.T) {
		client := &fakeBranchClient{
			branchSHAs: []string{"a", "merge", "b"},
			mergeSHAs:  map[string]bool{"merge": true},
		}
		got := filterCurrentBranchCommits(client, "/repo",
			[]*state.CommitRecord{rec("a"), rec("merge"), rec("b")},
			zerolog.Nop(),
		)

		require.Len(t, got, 2)
		assert.Equal(t, "a", got[0].SHA)
		assert.Equal(t, "b", got[1].SHA)
	})

	t.Run("returns input unchanged on branch enumeration error", func(t *testing.T) {
		client := &fakeBranchClient{branchErr: assert.AnError}
		input := []*state.CommitRecord{rec("a"), rec("b")}
		got := filterCurrentBranchCommits(client, "/repo", input, zerolog.Nop())
		assert.Equal(t, input, got)
	})
}

func TestRunTracePushEmpty(t *testing.T) {
	t.Run("AllowEmpty=false returns nil when there are no AI commits", func(t *testing.T) {
		_, gitDir := initGitRepo(t)
		store := state.NewGitStore(gitDir)
		require.NoError(t, store.InitTraceDir())

		err := RunTracePush(t.Context(), zerolog.Nop(), RunTracePushOpts{})
		require.NoError(t, err, "no commits + AllowEmpty=false must short-circuit cleanly")
	})

	t.Run("AllowEmpty=true returns nil when there are no sessions either", func(t *testing.T) {
		_, gitDir := initGitRepo(t)
		store := state.NewGitStore(gitDir)
		require.NoError(t, store.InitTraceDir())

		err := RunTracePush(t.Context(), zerolog.Nop(), RunTracePushOpts{AllowEmpty: true})
		require.NoError(t, err, "no commits + no sessions must short-circuit cleanly even under AllowEmpty")
	})
}

// TestRunTracePush_SkipsWhenAllTracked verifies that the pre-push hook
// short-circuits when every AI-assisted commit on the branch has already
// been marked tracked by a previous successful push. This is the core
// dedup behaviour for back-to-back `git push` (and `git push --tags`)
// invocations that introduce no new commits.
func TestRunTracePush_SkipsWhenAllTracked(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	// Use the SHA of the initial commit so filterCurrentBranchCommits keeps
	// the record (it drops SHAs not reachable from HEAD).
	sha := headSHA(t, dir)
	rec := &state.CommitRecord{
		SHA:        sha,
		Message:    "initial",
		SessionIDs: []string{"sess-1"},
		Timestamp:  state.NowTimestamp(),
		Tracked:    true,
	}
	require.NoError(t, store.SaveCommitRecord(rec))

	// AllowEmpty=false matches the pre-push hook caller. With every AI
	// commit already tracked, RunTracePush must return nil without
	// attempting to contact the executor.
	err := RunTracePush(t.Context(), zerolog.Nop(), RunTracePushOpts{})
	require.NoError(t, err, "all tracked AI commits must short-circuit cleanly")

	// Record should still be on disk and still marked tracked (we skipped
	// before any wipe could run).
	got := mustLoadRecord(t, store, sha)
	assert.True(t, got.Tracked, "tracked flag must survive the short-circuit")
}

// TestCommitRecord_TrackedRoundTrip verifies that the new Tracked field
// round-trips through JSON and that legacy records written before this
// change (no `tracked` key on disk) decode as Tracked=false.
func TestCommitRecord_TrackedRoundTrip(t *testing.T) {
	gitDir := t.TempDir()
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	t.Run("tracked=true round-trips", func(t *testing.T) {
		rec := &state.CommitRecord{
			SHA:        "abc",
			Message:    "m",
			SessionIDs: []string{"sess-1"},
			Timestamp:  state.NowTimestamp(),
			Tracked:    true,
		}
		require.NoError(t, store.SaveCommitRecord(rec))

		all, err := store.LoadAllCommitRecords()
		require.NoError(t, err)
		require.Len(t, all, 1)
		assert.True(t, all[0].Tracked)
	})

	t.Run("legacy record without tracked key decodes as false", func(t *testing.T) {
		legacyDir := t.TempDir()
		require.NoError(t, state.NewGitStore(legacyDir).InitTraceDir())

		raw := `{"sha":"def","message":"legacy","session_ids":["sess-1"],"timestamp":"2026-01-01T00:00:00Z"}`
		path := filepath.Join(legacyDir, "chainloop-trace", "commits", "def.json")
		require.NoError(t, os.WriteFile(path, []byte(raw), 0600))

		all, err := state.NewGitStore(legacyDir).LoadAllCommitRecords()
		require.NoError(t, err)
		require.Len(t, all, 1)
		assert.False(t, all[0].Tracked, "legacy records must default to untracked")
	})
}

// initGitRepo creates a real git repo in dir and chdirs into it for the test.
// Restores cwd at teardown.
func initGitRepo(t *testing.T) (dir, gitDir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}

	// Resolve symlinks: on macOS t.TempDir() hands back /var/folders/... while
	// the repo root resolves to /private/var/folders/..., and the mismatch makes
	// the repo-relative paths under test come out as "../.." escapes.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, string(out))
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("config", "commit.gpgsign", "false")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0600))
	run("add", "init.txt")
	run("commit", "-m", "initial")

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	return dir, filepath.Join(dir, ".git")
}

// runGit runs git in dir, failing the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, string(out))
}

func TestHandlePostCommit_UsesTrailerWhenPresent(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	// Make a commit whose message contains a Chainloop-Trace-Sessions trailer
	// but with NO matching ai-lines data (so file-based derivation would
	// return nothing). The trailer is what carries attribution forward across
	// rebase/cherry-pick.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rebased.txt"), []byte("hello"), 0600))
	runGit(t, dir, "add", "rebased.txt")
	runGit(t, dir, "commit", "-m", "feat: rebased work\n\nChainloop-Trace-Sessions: orig-1, orig-2\n")

	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	commits, err := store.LoadAllCommitRecords()
	require.NoError(t, err)
	require.Len(t, commits, 1)
	assert.Equal(t, []string{"orig-1", "orig-2"}, commits[0].SessionIDs)
}

func TestHandlePostCommit_SkipsMergeCommit(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	// Create a side branch with one commit, then merge it into main with --no-ff
	// to force a merge commit.
	runGit(t, dir, "checkout", "-b", "side")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "side.txt"), []byte("side"), 0600))
	runGit(t, dir, "add", "side.txt")
	runGit(t, dir, "commit", "-m", "side commit")
	runGit(t, dir, "checkout", "main")
	runGit(t, dir, "merge", "--no-ff", "-m", "merge side", "side")

	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	commits, err := store.LoadAllCommitRecords()
	require.NoError(t, err)
	assert.Empty(t, commits, "no CommitRecord should be saved for a merge commit")
}

func TestHandleCommitMsg_SkipsWhenMergeHeadPresent(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)

	// Plant ai-lines data so the hook would otherwise append a trailer.
	require.NoError(t, store.InitTraceDir())
	require.NoError(t, store.RecordLineRanges("session-x", "init.txt", []aicodingsession.LineRange{{Start: 1, End: 1}}))

	// Plant MERGE_HEAD to simulate a merge in progress.
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte("abc\n"), 0600))

	// Stage a change matching the ai-lines file so matchSessionsToFiles would match.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "init.txt"), []byte("changed"), 0600))
	cmd := exec.Command("git", "add", "init.txt")
	cmd.Dir = dir
	require.NoError(t, cmd.Run())

	msgPath := filepath.Join(dir, "COMMIT_EDITMSG")
	original := "merge: pulling in side\n"
	require.NoError(t, os.WriteFile(msgPath, []byte(original), 0600))

	require.NoError(t, handleCommitMsg(msgPath, zerolog.Nop()))

	got, err := os.ReadFile(msgPath)
	require.NoError(t, err)
	assert.Equal(t, original, string(got), "merge commit message should not get a trailer")
}

// TestRebaseLifecycle exercises the full attribution lifecycle across a rebase
// and a merge using a real git repo. It pins the end-to-end behavior the
// manual verification covers: rebased commits inherit attribution via the
// trailer, old SHA records become orphans that the pre-push filter drops, and
// merge commits never produce records at all.
func TestRebaseLifecycle(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	// detectBaseBranchGoGit looks for refs/remotes/origin/main, so plant it.
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")

	// Plant ai-lines for sess-1 against a.txt so the file-based path also
	// works as a fallback.
	require.NoError(t, store.RecordLineRanges("sess-1", "a.txt", []aicodingsession.LineRange{{Start: 1, End: 1}}))

	// --- 1. Original commit on a feature branch with the trailer. ---
	runGit(t, dir, "checkout", "-b", "feature")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0600))
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "feat: add a\n\nChainloop-Trace-Sessions: sess-1\n")

	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	oldSHA := headSHA(t, dir)
	oldRecord := mustLoadRecord(t, store, oldSHA)
	require.Equal(t, []string{"sess-1"}, oldRecord.SessionIDs)

	// --- 2. Advance main with an unrelated commit, then cherry-pick the
	//        original commit on top — simulates rebase replay onto a moved
	//        base. The new commit has a different parent, hence a new SHA,
	//        but git preserves the message (and trailer) verbatim. ---
	runGit(t, dir, "checkout", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.txt"), []byte("main moved\n"), 0600))
	runGit(t, dir, "add", "main.txt")
	runGit(t, dir, "commit", "-m", "main: move forward")
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")

	runGit(t, dir, "checkout", "-b", "feature-rebased")
	runGit(t, dir, "cherry-pick", oldSHA)

	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	newSHA := headSHA(t, dir)
	require.NotEqual(t, oldSHA, newSHA, "cherry-pick should produce a different SHA")

	newRecord := mustLoadRecord(t, store, newSHA)
	assert.Equal(t, []string{"sess-1"}, newRecord.SessionIDs,
		"new SHA's record should inherit session IDs from the trailer")

	// Old record still on disk — orphan cleanup is the filter's job, not post-commit's.
	all, err := store.LoadAllCommitRecords()
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// --- 3. The pre-push filter should drop the orphan and keep the new SHA. ---
	client := tracegit.NewGoGitClient()
	filtered := filterCurrentBranchCommits(client, dir, all, zerolog.Nop())
	require.Len(t, filtered, 1)
	assert.Equal(t, newSHA, filtered[0].SHA, "only the in-branch SHA should survive the filter")

	// --- 4. Create and merge a side branch — merge commit must not be recorded. ---
	runGit(t, dir, "checkout", "main")
	runGit(t, dir, "checkout", "-b", "side")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.txt"), []byte("side\n"), 0600))
	runGit(t, dir, "add", "s.txt")
	runGit(t, dir, "commit", "-m", "side: change")

	runGit(t, dir, "checkout", "feature-rebased")
	runGit(t, dir, "merge", "--no-ff", "-m", "merge side", "side")

	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	mergeSHA := headSHA(t, dir)
	_, err = os.Stat(filepath.Join(gitDir, "chainloop-trace", "commits", mergeSHA+".json"))
	assert.True(t, os.IsNotExist(err), "no record should exist for the merge commit")

	// --- 5. Filter again post-merge: still only the rebased SHA in evidence. ---
	all, err = store.LoadAllCommitRecords()
	require.NoError(t, err)
	filtered = filterCurrentBranchCommits(client, dir, all, zerolog.Nop())
	require.Len(t, filtered, 1)
	assert.Equal(t, newSHA, filtered[0].SHA)
}

// headSHA returns the SHA of HEAD in dir.
func headSHA(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)

	return strings.TrimSpace(string(out))
}

// mustLoadRecord reads the CommitRecord for sha and fails the test if it doesn't exist.
func mustLoadRecord(t *testing.T, store *state.Store, sha string) *state.CommitRecord {
	t.Helper()
	all, err := store.LoadAllCommitRecords()
	require.NoError(t, err)
	for _, r := range all {
		if r.SHA == sha {
			return r
		}
	}
	require.Failf(t, "no CommitRecord found", "sha %s", sha)

	return nil
}

func TestCommitDescriptions(t *testing.T) {
	t.Run("includes first line of message", func(t *testing.T) {
		commits := []*state.CommitRecord{
			{SHA: "abc123", Message: "Implement hello world app"},
			{SHA: "def456", Message: "Add credit header"},
		}
		descs := commitDescriptions(commits)
		assert.Equal(t, []string{
			"abc123 Implement hello world app",
			"def456 Add credit header",
		}, descs)
	})

	t.Run("uses only first line for multi-line messages", func(t *testing.T) {
		commits := []*state.CommitRecord{
			{SHA: "abc123", Message: "First line\n\nMore details here"},
		}
		descs := commitDescriptions(commits)
		assert.Equal(t, []string{"abc123 First line"}, descs)
	})
}

// TestPushRebasePushPreservesAIAttribution is the end-to-end regression test
// for PFM-5878. The scenario it captures, in order:
//
//  1. AI session edits a file (ai-lines recorded for sess-1).
//  2. User commits (commit-msg appends the trailer; post-commit fires).
//  3. User pushes — runTracePush attests with AI attribution; the post-push
//     wipe + GC must preserve commits/ and ai-lines/ because they're the only
//     things tying a future rebase replay back to the original session.
//  4. origin advances on main (a different change pushed elsewhere).
//  5. Feature branch is rebased onto the new origin/main. Each replayed
//     commit's post-commit hook reads the trailer and writes a fresh
//     CommitRecord under the new SHA.
//  6. User force-pushes — runTracePush must produce a second attestation
//     where main.go is again attributed to AI (line ranges + sessionID
//     intact), not silently flipped to human.
//
// The test exercises the same code paths the real hooks invoke, but skips
// the actual attestation executor (no Chainloop control plane required).
// Instead it asserts the inputs and outputs of attribution.Enrich after
// the rebase, which is the single point where the bug manifests.
func TestPushRebasePushPreservesAIAttribution(t *testing.T) {
	dir, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())

	const sessionID = "5f3b6dd7-f6ca-4639-a65c-493a12cd2844"

	// --- 0. Establish hello.go on main first so the AI commit's diff truly
	//        shows it as a deletion (not just an absent file on a side branch). ---
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.go"), []byte("a\nb\nc\nd\ne\nf\ng\n"), 0600))
	runGit(t, dir, "add", "hello.go")
	runGit(t, dir, "commit", "-m", "preexisting hello.go")
	// detectBaseBranchGoGit looks for refs/remotes/origin/main; plant it.
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")

	// --- 1. AI-edits main.go and records line ranges (one AI line at line 1). ---
	// The agent hook would do this in production via state.RecordLineRanges.
	require.NoError(t, store.RecordLineRanges(sessionID, "main.go", []aicodingsession.LineRange{{Start: 1, End: 1}}))
	require.NoError(t, store.SaveSessionRecord(&state.SessionRecord{
		SessionID: sessionID, Provider: "claude", Active: true, StartedAt: state.NowTimestamp(),
	}))

	// --- 2. Author the AI commit on a feature branch. main.go is created
	//        with 8 lines (1 AI + 7 human); hello.go is deleted. The trailer
	//        binds the commit to sessionID across rebases. ---
	runGit(t, dir, "checkout", "-b", "feature")
	require.NoError(t, os.Remove(filepath.Join(dir, "hello.go")))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("ai-line-1\nh1\nh2\nh3\nh4\nh5\nh6\nh7\n"), 0600))
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "feat: switch to main\n\nChainloop-Trace-Sessions: "+sessionID+"\n")
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	preRebaseSHA := headSHA(t, dir)

	// Sanity: pre-rebase Enrich attributes main.go to AI.
	preChanges := buildSessionCodeChanges(t, store, dir, preRebaseSHA, sessionID)
	require.NotZero(t, preChanges.AILinesAdded, "pre-rebase: main.go must be attributed to AI")
	require.Equal(t, 1, preChanges.AILinesAdded, "pre-rebase: only the line covered by ai-lines is AI")
	requireFileAttribution(t, preChanges, "main.go", trace.AttributionAI, []string{sessionID})
	requireFileAttribution(t, preChanges, "hello.go", trace.AttributionHuman, nil)

	// --- 3. Simulate the post-push wipe + reachability GC that runs at the
	//        end of runTracePush. ai-lines/ and commits/ for sessionID must
	//        survive because the SHA is reachable from feature. ---
	require.NoError(t, store.WipeTraceDir())
	gitClient := tracegit.NewGoGitClient()
	liveSHAs, err := gitClient.LocalReachableSHAs(dir)
	require.NoError(t, err)
	require.NotEmpty(t, liveSHAs, "post-push GC must see reachable SHAs")
	require.NoError(t, store.GCOrphans(liveSHAs))

	require.NotEmpty(t, store.LoadAILineAttribution(sessionID).Files,
		"post-push: ai-lines for the live session must survive the wipe + GC")

	// --- 4. origin/main advances with an unrelated commit. ---
	runGit(t, dir, "checkout", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "unrelated.txt"), []byte("upstream change\n"), 0600))
	runGit(t, dir, "add", "unrelated.txt")
	runGit(t, dir, "commit", "-m", "main: upstream change")
	runGit(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")

	// --- 5. Rebase the feature branch onto the new main. Cherry-pick is the
	//        deterministic stand-in for `git rebase` — same effect on hooks
	//        (post-commit fires per replayed commit) and same trailer
	//        preservation. ---
	runGit(t, dir, "checkout", "-b", "feature-rebased")
	runGit(t, dir, "cherry-pick", preRebaseSHA)
	require.NoError(t, handlePostCommit(t.Context(), zerolog.Nop()))

	postRebaseSHA := headSHA(t, dir)
	require.NotEqual(t, preRebaseSHA, postRebaseSHA, "rebase must produce a new SHA")

	postRec := mustLoadRecord(t, store, postRebaseSHA)
	require.Equal(t, []string{sessionID}, postRec.SessionIDs,
		"post-rebase CommitRecord must inherit sessionID from the trailer")

	// --- 6. Push 2: the second attestation must again attribute main.go to AI. ---
	postChanges := buildSessionCodeChanges(t, store, dir, postRebaseSHA, sessionID)
	require.Equal(t, 1, postChanges.AILinesAdded,
		"PFM-5878 regression: post-rebase attribution must match pre-rebase, not flip to human")
	require.Equal(t, preChanges.LinesAdded, postChanges.LinesAdded, "diff size must be stable across rebase")
	require.Equal(t, preChanges.LinesRemoved, postChanges.LinesRemoved)
	requireFileAttribution(t, postChanges, "main.go", trace.AttributionAI, []string{sessionID})
	requireFileAttribution(t, postChanges, "hello.go", trace.AttributionHuman, nil)
}

// buildSessionCodeChanges runs the same diff-and-enrich pipeline runTracePush
// uses for one session: aggregate the session's commits via
// CodeChangesForCommits, then layer on AI line attribution from ai-lines/.
// Returns the resulting CodeChanges so the caller can assert on it.
func buildSessionCodeChanges(t *testing.T, store *state.Store, dir, sha, sessionID string) *aicodingsession.CodeChanges {
	t.Helper()
	client := tracegit.NewGoGitClient()
	cc, err := client.CodeChangesForCommits(dir, []string{sha})
	require.NoError(t, err)
	attr := store.LoadAILineAttribution(sessionID)
	attribution.Enrich(sessionID, attr.Files, cc)

	return cc
}

// requireFileAttribution finds path in changes.Files and asserts attribution
// + sessionIDs match. wantSessionIDs may be nil when no session owns the file.
func requireFileAttribution(t *testing.T, changes *aicodingsession.CodeChanges, path, wantAttr string, wantSessionIDs []string) {
	t.Helper()
	for _, f := range changes.Files {
		if f.Path != path {
			continue
		}
		assert.Equal(t, wantAttr, f.Attribution, "attribution for %s", path)
		assert.Equal(t, wantSessionIDs, f.SessionIDs, "session IDs for %s", path)

		return
	}
	require.Failf(t, "file not found in CodeChanges", "%s (got %v)", path, changes.Files)
}
