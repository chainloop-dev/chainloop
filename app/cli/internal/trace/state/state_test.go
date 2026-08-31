package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogFilePath(t *testing.T) {
	assert.Equal(t, filepath.Join("/fake/.git", "chainloop-trace", "log.txt"), NewGitStore("/fake/.git").LogFilePath())
}

func TestTraceInitialized(t *testing.T) {
	store := NewGitStore(t.TempDir())

	t.Run("not initialized by default", func(t *testing.T) {
		assert.False(t, store.IsTraceInitialized())
	})

	t.Run("mark and check initialized", func(t *testing.T) {
		require.NoError(t, store.MarkTraceInitialized())
		assert.True(t, store.IsTraceInitialized())
	})

	t.Run("remove initialized", func(t *testing.T) {
		require.NoError(t, store.MarkTraceInitialized())
		require.NoError(t, store.RemoveTraceInitialized())
		assert.False(t, store.IsTraceInitialized())
	})

	t.Run("remove nonexistent is noop", func(t *testing.T) {
		empty := NewGitStore(t.TempDir())
		require.NoError(t, os.MkdirAll(empty.traceDirPath(), 0755))
		assert.NoError(t, empty.RemoveTraceInitialized())
	})
}

func TestInitTraceDir(t *testing.T) {
	store := NewGitStore(t.TempDir())

	require.NoError(t, store.InitTraceDir())

	base := store.traceDirPath()
	for _, sub := range []string{"sessions", "raw", "commits", "snapshots", "ai-lines"} {
		info, err := os.Stat(filepath.Join(base, sub))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	}
}

func TestWipeTraceDir(t *testing.T) {
	store := NewGitStore(t.TempDir())

	require.NoError(t, store.InitTraceDir())
	require.NoError(t, store.MarkTraceInitialized())

	// Populate one of every kind of state the wipe interacts with.
	require.NoError(t, store.SaveCommitRecord(&CommitRecord{SHA: "abc123", Message: "test", SessionIDs: []string{"sess-1"}, Timestamp: "2026-03-28T00:00:00Z"}))
	require.NoError(t, store.SaveSessionRecord(&SessionRecord{SessionID: "sess-1", Active: true, StartedAt: "2026-03-28T00:00:00Z"}))
	require.NoError(t, store.RecordLineRanges("sess-1", "src/foo.go", []aicodingsession.LineRange{{Start: 1, End: 5}}))
	require.NoError(t, store.SaveFileSnapshot("sess-1", "src/foo.go", []byte("before")))

	require.NoError(t, store.WipeTraceDir())

	// Sentinel preserved at trace dir root.
	assert.True(t, store.IsTraceInitialized())

	// Single-use state is gone (would otherwise leak into the next push).
	_, err := store.LoadFileSnapshot("sess-1", "src/foo.go")
	assert.True(t, os.IsNotExist(err), "snapshots should be wiped")

	// Attribution-bearing state survives so a future rebase that replays
	// these commits can be re-attributed via the trailer (PFM-5878).
	commits, err := store.LoadAllCommitRecords()
	require.NoError(t, err)
	assert.Len(t, commits, 1, "commits/ must survive the wipe — GC handles bounded growth")

	assert.True(t, store.SessionRecordExists("sess-1"), "session record must survive the wipe")

	attr := store.LoadAILineAttribution("sess-1")
	require.NotNil(t, attr)
	assert.Equal(t, []aicodingsession.LineRange{{Start: 1, End: 5}}, attr.Files["src/foo.go"],
		"ai-lines must survive the wipe so post-rebase attribution still works")
}

func TestGCOrphans(t *testing.T) {
	store := NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	// Three commits; only sha-live is reachable from a local branch.
	require.NoError(t, store.SaveCommitRecord(&CommitRecord{SHA: "sha-live", SessionIDs: []string{"sess-live"}, Timestamp: "2026-03-28T00:00:00Z"}))
	require.NoError(t, store.SaveCommitRecord(&CommitRecord{SHA: "sha-orphan", SessionIDs: []string{"sess-orphan"}, Timestamp: "2026-03-28T00:00:00Z"}))
	require.NoError(t, store.SaveCommitRecord(&CommitRecord{SHA: "sha-shared", SessionIDs: []string{"sess-live"}, Timestamp: "2026-03-28T00:00:00Z"}))

	// Both sessions have ai-lines + session records.
	require.NoError(t, store.SaveSessionRecord(&SessionRecord{SessionID: "sess-live", Active: true, StartedAt: "2026-03-28T00:00:00Z"}))
	require.NoError(t, store.SaveSessionRecord(&SessionRecord{SessionID: "sess-orphan", Active: false, StartedAt: "2026-03-28T00:00:00Z"}))
	require.NoError(t, store.RecordLineRanges("sess-live", "live.go", []aicodingsession.LineRange{{Start: 1, End: 5}}))
	require.NoError(t, store.RecordLineRanges("sess-orphan", "orphan.go", []aicodingsession.LineRange{{Start: 1, End: 5}}))

	require.NoError(t, store.GCOrphans(map[string]bool{"sha-live": true, "sha-shared": true}))

	commits, err := store.LoadAllCommitRecords()
	require.NoError(t, err)
	require.Len(t, commits, 2, "orphan SHA must be dropped; live and shared survive")

	gotSHAs := make([]string, 0, len(commits))
	for _, c := range commits {
		gotSHAs = append(gotSHAs, c.SHA)
	}
	assert.ElementsMatch(t, []string{"sha-live", "sha-shared"}, gotSHAs)

	// sess-live still has a live commit; sess-orphan does not.
	assert.True(t, store.SessionRecordExists("sess-live"))
	assert.False(t, store.SessionRecordExists("sess-orphan"), "session with no live commits must be GCed")

	live := store.LoadAILineAttribution("sess-live")
	assert.NotEmpty(t, live.Files, "ai-lines for live session must survive")
	orphan := store.LoadAILineAttribution("sess-orphan")
	assert.Empty(t, orphan.Files, "ai-lines for orphan session must be dropped")
}

// An empty liveSHAs (no local branches readable, transient git error) must
// be treated as "skip the GC", not "wipe everything". The latter would erase
// every CommitRecord and ai-lines/sessions entry on a single bad git call,
// taking a rebase-tolerant session with it.
func TestGCOrphans_EmptyLiveSHAsSkipsCleanup(t *testing.T) {
	store := NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	require.NoError(t, store.SaveCommitRecord(&CommitRecord{SHA: "sha-1", SessionIDs: []string{"sess-1"}, Timestamp: "2026-03-28T00:00:00Z"}))
	require.NoError(t, store.SaveSessionRecord(&SessionRecord{SessionID: "sess-1", Active: true, StartedAt: "2026-03-28T00:00:00Z"}))
	require.NoError(t, store.RecordLineRanges("sess-1", "foo.go", []aicodingsession.LineRange{{Start: 1, End: 3}}))

	require.NoError(t, store.GCOrphans(map[string]bool{}))

	commits, err := store.LoadAllCommitRecords()
	require.NoError(t, err)
	assert.Len(t, commits, 1, "empty liveSHAs must not drop commit records")
	assert.True(t, store.SessionRecordExists("sess-1"))
	assert.NotEmpty(t, store.LoadAILineAttribution("sess-1").Files)
}

func TestValidSessionID(t *testing.T) {
	for _, id := range []string{"sess-1", "ses_abc123", "9beb4bd3-6eee-42a5-ab49-8fc04eee921c"} {
		assert.True(t, ValidSessionID(id), id)
	}

	// An ID that is not a single path element would escape the agent's own
	// transcript directory when joined into a read path.
	for _, id := range []string{"", ".", "..", "../etc/passwd", "a/x", "../../secret"} {
		assert.False(t, ValidSessionID(id), id)
	}
}

func TestSanitizeID(t *testing.T) {
	// An agent-assigned ID needs no rewriting, so its on-disk name must not move.
	assert.Equal(t, "9beb4bd3-6eee-42a5-ab49-8fc04eee921c", SanitizeID("9beb4bd3-6eee-42a5-ab49-8fc04eee921c"))

	// IDs that collide under rewriting must not share a name.
	assert.NotEqual(t, SanitizeID("a/x"), SanitizeID("a_x"))
	assert.NotEqual(t, SanitizeID("."), SanitizeID(".."))
	for _, id := range []string{"a/x", ".", "..", "../etc/passwd"} {
		safe := SanitizeID(id)
		assert.Equal(t, safe, filepath.Base(safe), "sanitized %q must be a single path element", id)
		// GCOrphans and LoadAllAILineAttributions feed sanitized names back in.
		assert.Equal(t, safe, SanitizeID(safe), "SanitizeID must be idempotent")
	}
}

func TestSessionRecordExists(t *testing.T) {
	store := NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	t.Run("returns false when no record exists", func(t *testing.T) {
		assert.False(t, store.SessionRecordExists("nonexistent"))
	})

	t.Run("returns true after saving record", func(t *testing.T) {
		rec := &SessionRecord{SessionID: "sess-1", Active: true, StartedAt: "2026-03-28T00:00:00Z"}
		require.NoError(t, store.SaveSessionRecord(rec))
		assert.True(t, store.SessionRecordExists("sess-1"))
	})
}

func TestSessionRecords(t *testing.T) {
	store := NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	t.Run("save session record", func(t *testing.T) {
		rec := &SessionRecord{SessionID: "sess-1", Active: true, StartedAt: "2026-03-28T00:00:00Z"}
		require.NoError(t, store.SaveSessionRecord(rec))

		// Verify file was written
		data, err := os.ReadFile(filepath.Join(store.traceDirPath(), "sessions", "sess-1.json"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "sess-1")
	})
}

func TestCommitRecords(t *testing.T) {
	store := NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	t.Run("save and load commit records", func(t *testing.T) {
		rec1 := &CommitRecord{SHA: "abc123", Message: "first", SessionIDs: []string{"sess-1"}, Timestamp: "2026-03-28T00:00:00Z"}
		rec2 := &CommitRecord{SHA: "def456", Message: "second", Timestamp: "2026-03-28T00:01:00Z"}
		require.NoError(t, store.SaveCommitRecord(rec1))
		require.NoError(t, store.SaveCommitRecord(rec2))

		records, err := store.LoadAllCommitRecords()
		require.NoError(t, err)
		assert.Len(t, records, 2)

		// Find records by SHA
		shas := make(map[string]*CommitRecord)
		for _, r := range records {
			shas[r.SHA] = r
		}
		assert.Equal(t, []string{"sess-1"}, shas["abc123"].SessionIDs)
		assert.Empty(t, shas["def456"].SessionIDs)
	})

	t.Run("load from empty dir returns nil", func(t *testing.T) {
		records, err := NewGitStore(t.TempDir()).LoadAllCommitRecords()
		require.NoError(t, err)
		assert.Nil(t, records)
	})
}

func TestFileSnapshots(t *testing.T) {
	store := NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	t.Run("save and load snapshot", func(t *testing.T) {
		content := []byte("func main() {\n\tfmt.Println(\"hello\")\n}\n")
		require.NoError(t, store.SaveFileSnapshot("sess-1", "/path/to/file.go", content))

		loaded, err := store.LoadFileSnapshot("sess-1", "/path/to/file.go")
		require.NoError(t, err)
		assert.Equal(t, content, loaded)
	})

	t.Run("load nonexistent snapshot returns error", func(t *testing.T) {
		_, err := store.LoadFileSnapshot("sess-1", "/nonexistent/file.go")
		assert.Error(t, err)
	})

	t.Run("delete snapshot", func(t *testing.T) {
		content := []byte("content")
		require.NoError(t, store.SaveFileSnapshot("sess-1", "/delete/me.go", content))

		store.DeleteFileSnapshot("sess-1", "/delete/me.go")

		_, err := store.LoadFileSnapshot("sess-1", "/delete/me.go")
		assert.Error(t, err)
	})
}

func TestAILineAttribution(t *testing.T) {
	t.Run("record and load line ranges", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.InitTraceDir())

		ranges := []aicodingsession.LineRange{{Start: 1, End: 5}, {Start: 10, End: 15}}
		require.NoError(t, store.RecordLineRanges("sess-1", "/path/to/file.go", ranges))

		attr := store.LoadAILineAttribution("sess-1")
		assert.Equal(t, "sess-1", attr.SessionID)
		assert.Equal(t, ranges, attr.Files["/path/to/file.go"])
	})

	t.Run("accumulates ranges for same file", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.InitTraceDir())

		require.NoError(t, store.RecordLineRanges("sess-1", "/file.go", []aicodingsession.LineRange{{Start: 1, End: 5}}))
		require.NoError(t, store.RecordLineRanges("sess-1", "/file.go", []aicodingsession.LineRange{{Start: 20, End: 25}}))

		attr := store.LoadAILineAttribution("sess-1")
		assert.Len(t, attr.Files["/file.go"], 2)
	})

	t.Run("multiple files in one session", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.InitTraceDir())

		require.NoError(t, store.RecordLineRanges("sess-1", "/a.go", []aicodingsession.LineRange{{Start: 1, End: 3}}))
		require.NoError(t, store.RecordLineRanges("sess-1", "/b.go", []aicodingsession.LineRange{{Start: 5, End: 10}}))

		attr := store.LoadAILineAttribution("sess-1")
		assert.Len(t, attr.Files, 2)
	})

	t.Run("empty ranges record file touch", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.InitTraceDir())

		require.NoError(t, store.RecordLineRanges("sess-1", "/file.go", nil))

		attr := store.LoadAILineAttribution("sess-1")
		require.Contains(t, attr.Files, "/file.go")
		assert.Empty(t, attr.Files["/file.go"])

		attrs, err := store.LoadAllAILineAttributions()
		require.NoError(t, err)
		assert.Len(t, attrs, 1)
	})

	t.Run("load nonexistent returns empty", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		attr := store.LoadAILineAttribution("nonexistent")
		assert.Empty(t, attr.Files)
	})

	t.Run("load all attributions", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.InitTraceDir())

		require.NoError(t, store.RecordLineRanges("sess-1", "/a.go", []aicodingsession.LineRange{{Start: 1, End: 3}}))
		require.NoError(t, store.RecordLineRanges("sess-2", "/b.go", []aicodingsession.LineRange{{Start: 5, End: 10}}))

		attrs, err := store.LoadAllAILineAttributions()
		require.NoError(t, err)
		assert.Len(t, attrs, 2)
	})

	t.Run("load all reports the unsanitized session ID", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.InitTraceDir())

		// Sanitizing "org/sess-1" for the filename loses the original ID, which
		// is why the record itself carries it.
		rawID := "org/sess-1"
		require.NoError(t, store.RecordLineRanges(rawID, "/a.go", []aicodingsession.LineRange{{Start: 1, End: 3}}))

		attrs, err := store.LoadAllAILineAttributions()
		require.NoError(t, err)
		require.Len(t, attrs, 1)
		assert.Equal(t, rawID, attrs[0].SessionID, "consumers key off the agent-assigned ID, not the filename")
	})
}
