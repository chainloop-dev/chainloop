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
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// File statuses reported in aicodingsession.FileChange.Status. The control
// plane buckets on these exact strings when it ingests the evidence, so they
// are a wire contract, not free-form labels.
const (
	StatusCreated  = "created"
	StatusModified = "modified"
	StatusDeleted  = "deleted"
	StatusRenamed  = "renamed"
)

// Client abstracts git operations so the implementation can be swapped
// (e.g. from exec-based to go-git).
type Client interface {
	// Context collects repository metadata: remote URL, branch, commits since base.
	Context(repoRoot string) (*aicodingsession.GitContext, error)
	// CodeChanges collects file-level diff stats between the merge base and HEAD.
	CodeChanges(repoRoot string) (*aicodingsession.CodeChanges, error)
	// CodeChangesForRange collects file-level diff stats for a specific commit range.
	// The range covers startSHA^..endSHA (i.e. changes introduced by the commits).
	CodeChangesForRange(repoRoot, startSHA, endSHA string) (*aicodingsession.CodeChanges, error)
	// CodeChangesForCommits collects file-level diff stats for the union of the
	// provided commit SHAs. Each commit's parent..commit diff is computed
	// individually and summed by file path, so a non-contiguous set (e.g. one
	// session's commits interleaved with another's on the same branch) is
	// handled correctly. The order in which commits are visited is unspecified;
	// per-file Status reflects the most recently visited commit's view.
	CodeChangesForCommits(repoRoot string, shas []string) (*aicodingsession.CodeChanges, error)
	// CommitHeadInfo returns the SHA and message of HEAD.
	CommitHeadInfo(repoRoot string) (sha, message string, err error)
	// GeneratedMatcher returns a predicate that reports whether a repo-relative
	// path is marked linguist-generated=true in the repository's .gitattributes.
	// Implementations must return a no-op matcher (always false) when attributes
	// can't be read — filtering is best-effort and must not block attribution.
	GeneratedMatcher(repoRoot string) func(path string) bool
	// StagedFiles returns repo-relative paths of files staged for the current commit.
	StagedFiles(repoRoot string) ([]string, error)
	// SnapshotWorktree returns a signature of the working tree as a map of
	// repo-relative path (forward slashes) → content hash, skipping the .git
	// directory and any .gitignore'd paths. Used to detect files changed by an
	// agent-run shell command by diffing a before/after signature.
	SnapshotWorktree(repoRoot string) (map[string]string, error)
	// IsMergeCommit reports whether the commit identified by sha has more than
	// one parent. Returns an error when the commit cannot be resolved.
	IsMergeCommit(repoRoot, sha string) (bool, error)
	// BranchCommitSHAs returns the SHAs reachable from HEAD that are not
	// reachable from the default remote branch (HEAD ^merge-base). The merge
	// base itself is excluded. Order is unspecified.
	BranchCommitSHAs(repoRoot string) ([]string, error)
	// LocalReachableSHAs returns the union of SHAs reachable from every
	// local branch tip (refs/heads/*). Used by the post-push GC to identify
	// CommitRecords that are still alive on at least one branch — anything
	// missing is an orphan from a deleted branch or a pre-rebase rewrite and
	// is safe to drop.
	LocalReachableSHAs(repoRoot string) (map[string]bool, error)
}
