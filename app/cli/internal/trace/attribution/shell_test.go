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

package attribution

import (
	"testing"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"github.com/stretchr/testify/assert"
)

func TestOwnCommits(t *testing.T) {
	entry := func(old, new, msg string) tracegit.ReflogEntry {
		return tracegit.ReflogEntry{Old: old, New: new, Message: msg}
	}

	tests := []struct {
		name    string
		oldHead string
		newHead string
		entries []tracegit.ReflogEntry
		want    []tracegit.ReflogEntry
	}{
		{
			name:    "no move",
			oldHead: "a", newHead: "a",
		},
		{
			name:    "a commit",
			oldHead: "a", newHead: "b",
			entries: []tracegit.ReflogEntry{entry("a", "b", "commit: feat")},
			want:    []tracegit.ReflogEntry{entry("a", "b", "commit: feat")},
		},
		{
			name:    "an amend",
			oldHead: "a", newHead: "b",
			entries: []tracegit.ReflogEntry{entry("a", "b", "commit (amend): feat")},
			want:    []tracegit.ReflogEntry{entry("a", "b", "commit (amend): feat")},
		},
		{
			name:    "a rebase writes nothing of the command",
			oldHead: "a", newHead: "d",
			entries: []tracegit.ReflogEntry{
				entry("a", "c", "rebase (start): checkout main"),
				entry("c", "d", "rebase (pick): feat"),
				entry("d", "d", "rebase (finish): returning to refs/heads/feature"),
			},
		},
		{
			name:    "a fast-forward pull, a checkout and a reset",
			oldHead: "a", newHead: "d",
			entries: []tracegit.ReflogEntry{
				entry("a", "b", "pull: Fast-forward"),
				entry("b", "c", "checkout: moving from feature to main"),
				entry("c", "d", "reset: moving to d"),
			},
		},
		{
			name:    "a merge commit is not the command's own",
			oldHead: "a", newHead: "m",
			entries: []tracegit.ReflogEntry{entry("a", "m", "commit (merge): Merge branch 'main'")},
		},
		{
			name:    "a commit before and after a rebase",
			oldHead: "a", newHead: "e",
			entries: []tracegit.ReflogEntry{
				entry("a", "b", "commit: one"),
				entry("b", "c", "rebase (start): checkout main"),
				entry("c", "d", "rebase (pick): one"),
				entry("d", "d", "rebase (finish): returning to refs/heads/feature"),
				entry("d", "e", "commit: two"),
			},
			want: []tracegit.ReflogEntry{entry("a", "b", "commit: one"), entry("d", "e", "commit: two")},
		},
		{
			name:    "entries that do not start at the old HEAD",
			oldHead: "a", newHead: "c",
			entries: []tracegit.ReflogEntry{entry("x", "c", "commit: feat")},
		},
		{
			name:    "entries that do not end at the new HEAD",
			oldHead: "a", newHead: "z",
			entries: []tracegit.ReflogEntry{entry("a", "c", "commit: feat")},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, OwnCommits(tc.oldHead, tc.newHead, tc.entries))
		})
	}
}

func TestChangedPaths(t *testing.T) {
	t.Run("detects created, modified, and deleted paths", func(t *testing.T) {
		before := map[string]string{"a": "1", "b": "2", "c": "3"}
		after := map[string]string{"a": "1", "b": "9", "d": "4"}

		changed, deleted := ChangedPaths(before, after)

		assert.Equal(t, []string{"b", "d"}, changed) // b modified, d created
		assert.Equal(t, []string{"c"}, deleted)      // c removed
	})

	t.Run("empty when signatures are equal", func(t *testing.T) {
		sig := map[string]string{"a": "1", "b": "2"}

		changed, deleted := ChangedPaths(sig, sig)

		assert.Empty(t, changed)
		assert.Empty(t, deleted)
	})

	t.Run("all created from empty before", func(t *testing.T) {
		after := map[string]string{"b": "2", "a": "1"}

		changed, deleted := ChangedPaths(nil, after)

		assert.Equal(t, []string{"a", "b"}, changed) // sorted
		assert.Empty(t, deleted)
	})
}
