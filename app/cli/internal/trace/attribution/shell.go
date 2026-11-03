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
	"sort"
	"strings"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
)

// ownCommitPrefixes are the reflog messages of a commit that git wrote from
// the index: "git commit", also with --amend, and the first commit of a
// branch. A merge commit ("commit (merge)") is left out because its content
// comes from the merged branch.
var ownCommitPrefixes = []string{"commit: ", "commit (amend): ", "commit (initial): "}

// OwnCommits returns the HEAD moves of a shell command that are commits the
// command wrote itself, oldest first. entries are the HEAD reflog entries that
// the command wrote. Every other move, such as a rebase, a pull, a merge, a
// checkout or a reset, points HEAD at content that the command did not write.
//
// It returns nil when entries do not lead from oldHead to newHead: the reflog
// then does not tell the whole story of the command, and none of its moves can
// be trusted.
func OwnCommits(oldHead, newHead string, entries []tracegit.ReflogEntry) []tracegit.ReflogEntry {
	if len(entries) == 0 || entries[0].Old != oldHead || entries[len(entries)-1].New != newHead {
		return nil
	}

	var own []tracegit.ReflogEntry
	for _, e := range entries {
		for _, prefix := range ownCommitPrefixes {
			if strings.HasPrefix(e.Message, prefix) {
				own = append(own, e)
				break
			}
		}
	}

	return own
}

// ChangedPaths diffs two working-tree signatures (repo-relative path → content
// hash) captured before and after a shell command, returning the paths the
// command created or modified (changed) and the paths it removed (deleted).
// Both slices are sorted for deterministic recording. Used to attribute
// files touched by agent-run shell commands, which never fire the file-writing
// tool hooks.
func ChangedPaths(before, after map[string]string) (changed, deleted []string) {
	for path, hash := range after {
		if prev, ok := before[path]; !ok || prev != hash {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			deleted = append(deleted, path)
		}
	}

	sort.Strings(changed)
	sort.Strings(deleted)

	return changed, deleted
}
