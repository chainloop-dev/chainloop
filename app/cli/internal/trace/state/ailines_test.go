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

package state

import (
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLineCounts(t *testing.T) {
	c := LineCounts{}
	c.Add("x", "x", "y")
	assert.Equal(t, 3, c.Len())

	clone := c.Clone()
	assert.True(t, c.Take("x"))
	assert.True(t, c.Take("x"))
	assert.False(t, c.Take("x"), "only two copies were added")
	assert.Equal(t, 1, c.Len())
	assert.Equal(t, 3, clone.Len(), "a clone does not share counts")

	present := LineCounts{"y": 1, "z": 1}
	assert.Empty(t, c.Missing(present))
	assert.Equal(t, 2, present.Len(), "Missing does not change its argument")
	assert.Equal(t, []string{"x", "x"}, LineCounts{"x": 2, "y": 1}.Missing(LineCounts{"y": 3}))
}

func TestAILineAttribution_Lines(t *testing.T) {
	const oldLine = "old"

	edit := func(added, removed []string) EditRecord {
		return EditRecord{
			Ranges:       []aicodingsession.LineRange{{Start: 1, End: len(added)}},
			Lines:        LineSet{Added: added, Removed: removed},
			AddedCount:   len(added),
			RemovedCount: len(removed),
		}
	}

	t.Run("an edit opens the lines it wrote and removed", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.RecordEdit("sess", fileA, edit([]string{"a", "b"}, []string{oldLine})))

		attr := store.LoadAILineAttribution("sess")
		assert.Equal(t, LineCounts{"a": 1, "b": 1}, attr.Written[fileA])
		assert.Equal(t, LineCounts{oldLine: 1}, attr.Erased[fileA])
		assert.Equal(t, LineCounts{"a": 1, "b": 1}, attr.Open[fileA].Added)
		assert.Equal(t, LineCounts{oldLine: 1}, attr.Open[fileA].Removed)
		assert.Contains(t, attr.Pending, fileA)
		assert.False(t, attr.Legacy[fileA])
	})

	t.Run("removing an open line of the session closes it", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.RecordEdit("sess", fileA, edit([]string{"a"}, nil)))
		require.NoError(t, store.RecordEdit("sess", fileA, edit(nil, []string{"a"})))

		attr := store.LoadAILineAttribution("sess")
		assert.Zero(t, attr.Open[fileA].Added.Len())
		assert.Zero(t, attr.Open[fileA].Removed.Len(), "the line never reached a commit, so no commit can remove it")
		assert.NotContains(t, attr.Pending, fileA)
		assert.Equal(t, LineCounts{"a": 1}, attr.Erased[fileA], "the erased history keeps every removal")
	})

	t.Run("lines changed outside the session close", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.RecordEdit("sess", fileA, edit([]string{"a", "b"}, nil)))
		require.NoError(t, store.SettleLines("sess", fileA, LineSet{Added: []string{"b"}}))

		attr := store.LoadAILineAttribution("sess")
		assert.Equal(t, LineCounts{"a": 1}, attr.Open[fileA].Added)
		assert.Equal(t, LineCounts{"a": 1, "b": 1}, attr.Written[fileA], "written lines are history")
		assert.Contains(t, attr.Pending, fileA)

		require.NoError(t, store.SettleLines("sess", fileA, LineSet{Added: []string{"a"}}))
		assert.NotContains(t, store.LoadAILineAttribution("sess").Pending, fileA,
			"a file with no open lines does not credit the session on a later commit")
	})

	t.Run("a commit closes only the lines it took (R-006)", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.RecordEdit("sess", fileA, edit([]string{"a", "b"}, nil)))
		require.NoError(t, store.MarkCommitted("sess", "sha1", map[string]LineSet{fileA: {Added: []string{"a"}}}))

		attr := store.LoadAILineAttribution("sess")
		assert.Equal(t, LineCounts{"b": 1}, attr.Open[fileA].Added)
		assert.Contains(t, attr.Pending, fileA, "the rest of the file stays pending for the session")

		require.NoError(t, store.MarkCommitted("sess", "sha2", map[string]LineSet{fileA: {Added: []string{"b"}}}))
		assert.NotContains(t, store.LoadAILineAttribution("sess").Pending, fileA)
	})

	t.Run("an edit recorded after the commit snapshot stays open", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.RecordEdit("sess", fileA, edit([]string{"a"}, nil)))
		pending := store.LoadAILineAttribution("sess").Pending
		require.NoError(t, store.RecordEdit("sess", fileA, edit([]string{"c"}, nil)))
		require.NoError(t, store.markCommittedAt(pending, "sess", "sha1", map[string]LineSet{fileA: {Added: []string{"a"}}}))

		attr := store.LoadAILineAttribution("sess")
		assert.Equal(t, LineCounts{"c": 1}, attr.Open[fileA].Added)
		assert.Contains(t, attr.Pending, fileA)
	})

	t.Run("generated line counts include edits that never reach a commit (R-011)", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.RecordEdit("sess", fileA, edit([]string{"a", "b"}, []string{"x"})))
		require.NoError(t, store.RecordEdit("sess", "b.go", EditRecord{AddedCount: 4, RemovedCount: 2, Lines: LineSet{}}))
		require.NoError(t, store.SettleLines("sess", fileA, LineSet{Added: []string{"a", "b"}}))

		attr := store.LoadAILineAttribution("sess")
		assert.Equal(t, 6, attr.GeneratedAdded)
		assert.Equal(t, 3, attr.GeneratedRemoved)
	})

	t.Run("an entry without fingerprints is legacy", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		require.NoError(t, store.RecordLineRanges("sess", fileA, []aicodingsession.LineRange{{Start: 1, End: 2}}))
		require.NoError(t, store.RecordEdit("sess", "b.go", edit([]string{"a"}, nil)))

		attr := store.LoadAILineAttribution("sess")
		assert.True(t, attr.Legacy[fileA])
		assert.False(t, attr.Legacy["b.go"])
		assert.Contains(t, attr.Pending, fileA)
		assert.Contains(t, attr.Pending, "b.go")

		// A commit of the legacy file retires it as a whole, as before.
		require.NoError(t, store.MarkCommitted("sess", "sha1", map[string]LineSet{fileA: {}}))
		assert.NotContains(t, store.LoadAILineAttribution("sess").Pending, fileA)
	})

	t.Run("the pending edit time is the last edit of the file", func(t *testing.T) {
		store := NewGitStore(t.TempDir())
		first := time.Now().UTC().Add(-time.Hour)
		require.NoError(t, store.RecordEditAt("sess", fileA, edit([]string{"a"}, nil), first))
		require.NoError(t, store.RecordEditAt("sess", fileA, edit([]string{"b"}, nil), first.Add(time.Minute)))

		assert.True(t, store.LoadAILineAttribution("sess").Pending[fileA].At.Equal(first.Add(time.Minute)))
	})
}
