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
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
)

// ExecClient implements Client by shelling out to the git binary.
type ExecClient struct{}

var _ Client = (*ExecClient)(nil)

func NewExecClient() *ExecClient { return &ExecClient{} }

// resolveMergeBase finds the merge-base commit between the current branch and the default remote branch.
func resolveMergeBase(repoRoot string) string {
	baseBranch := detectBaseBranch(repoRoot)
	if baseBranch == "" {
		return ""
	}

	return gitOutput(repoRoot, "merge-base", baseBranch, "HEAD")
}

// Context collects repository metadata: remote URL, branch, commits since base.
func (c *ExecClient) Context(repoRoot string) (*aicodingsession.GitContext, error) {
	ctx := &aicodingsession.GitContext{
		WorkDir: repoRoot,
	}

	ctx.Repository = gitOutput(repoRoot, "config", "--get", "remote.origin.url")
	ctx.Branch = gitOutput(repoRoot, "rev-parse", "--abbrev-ref", "HEAD")

	return ctx, nil
}

// CodeChanges collects file-level diff stats between the merge base and HEAD.
func (c *ExecClient) CodeChanges(repoRoot string) (*aicodingsession.CodeChanges, error) {
	mergeBase := resolveMergeBase(repoRoot)
	if mergeBase == "" {
		return &aicodingsession.CodeChanges{}, nil
	}

	return parseDiffStats(repoRoot, fmt.Sprintf("%s...HEAD", mergeBase)), nil
}

// CodeChangesForRange collects file-level diff stats for a specific commit range.
// The range covers startSHA^..endSHA (changes introduced by the commits).
func (c *ExecClient) CodeChangesForRange(repoRoot, startSHA, endSHA string) (*aicodingsession.CodeChanges, error) {
	rangeSpec := fmt.Sprintf("%s^..%s", startSHA, endSHA)
	return parseDiffStats(repoRoot, rangeSpec), nil
}

// LocalReachableSHAs returns the union of SHAs reachable from every local
// branch tip. Implemented as a single `git rev-list` invocation across all
// refs under refs/heads/.
func (c *ExecClient) LocalReachableSHAs(repoRoot string) (map[string]bool, error) {
	out := gitOutput(repoRoot, "rev-list", "--branches=*")
	if out == "" {
		return map[string]bool{}, nil
	}

	shas := make(map[string]bool)
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			shas[line] = true
		}
	}

	return shas, nil
}

// emptyTreeSHA is git's well-known SHA-1 hash of an empty tree
// (`git hash-object -t tree /dev/null`). Used as the "before" side when
// diffing a root commit, which has no parent for `<sha>^` to resolve against.
const emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// CodeChangesForCommits aggregates per-commit (parent..commit) diffs across a
// possibly non-contiguous set of SHAs. See the GoGitClient counterpart for the
// rationale; this implementation walks each SHA via parseDiffStats and sums.
func (c *ExecClient) CodeChangesForCommits(repoRoot string, shas []string) (*aicodingsession.CodeChanges, error) {
	out := &aicodingsession.CodeChanges{}
	if len(shas) == 0 {
		return out, nil
	}

	byPath := make(map[string]*fileAgg)
	for _, sha := range shas {
		base := commitParentOrEmptyTree(repoRoot, sha)
		addCommitStats(out, byPath, parseDiffStats(repoRoot, fmt.Sprintf("%s..%s", base, sha)))
	}
	finalizeAggregatedFiles(out, byPath)

	return out, nil
}

// commitParentOrEmptyTree returns the parent SHA of `sha`, or the empty-tree
// SHA when `sha` is a root commit (`<sha>^` doesn't resolve). The fallback
// makes a root commit's diff the full tree as additions, matching the
// GoGitClient's zero-baseHash behavior.
func commitParentOrEmptyTree(repoRoot, sha string) string {
	if parent := gitOutput(repoRoot, "rev-parse", "--verify", "--quiet", sha+"^"); parent != "" {
		return parent
	}

	return emptyTreeSHA
}

// parseDiffStats runs git diff with the given range spec and returns parsed stats.
func parseDiffStats(repoRoot, rangeSpec string) *aicodingsession.CodeChanges {
	info := &aicodingsession.CodeChanges{}

	// Collect per-file line stats from numstat
	perFileStats := make(map[string][2]int) // path → [added, removed]
	numstat := gitOutput(repoRoot, "diff", "--numstat", rangeSpec)
	if numstat != "" {
		for _, line := range strings.Split(numstat, "\n") {
			fields := strings.SplitN(line, "\t", 3)
			if len(fields) != 3 {
				continue
			}

			added, _ := strconv.Atoi(fields[0])   // binary files show "-"
			removed, _ := strconv.Atoi(fields[1]) // which Atoi returns 0

			info.LinesAdded += added
			info.LinesRemoved += removed

			path := fields[2]
			perFileStats[path] = [2]int{added, removed}
			// For renames/copies numstat shows paths like "old => new"
			// or brace-style "dir/{old => new}/file"; resolve to destination.
			if dest := resolveNumstatRename(path); dest != "" {
				perFileStats[dest] = [2]int{added, removed}
			}
		}
	}

	nameStatus := gitOutput(repoRoot, "diff", "--name-status", rangeSpec)
	if nameStatus != "" {
		for _, line := range strings.Split(nameStatus, "\n") {
			// Rename/copy lines have 3 fields: "R100\told\tnew", others have 2: "M\tfile"
			fields := strings.Split(line, "\t")
			if len(fields) < 2 {
				continue
			}

			status := mapGitStatus(fields[0])
			// Use the last field — for renames/copies this is the destination path
			path := fields[len(fields)-1]
			fci := aicodingsession.FileChange{
				Path:   path,
				Status: status,
			}
			if stats, ok := perFileStats[path]; ok {
				fci.LinesAdded = stats[0]
				fci.LinesRemoved = stats[1]
			}
			info.Files = append(info.Files, fci)

			switch status {
			case StatusCreated:
				info.FilesCreated++
			case StatusDeleted:
				info.FilesDeleted++
			default:
				info.FilesModified++
			}
		}
	}

	return info
}

// resolveNumstatRename resolves a numstat rename path to the destination.
// Handles both simple "old => new" and brace-style "dir/{old => new}/file" patterns.
// Returns empty string if the path is not a rename.
func resolveNumstatRename(path string) string {
	if !strings.Contains(path, " => ") {
		return ""
	}

	// Brace-style: "dir/{old => new}/suffix"
	if braceStart := strings.Index(path, "{"); braceStart >= 0 {
		if braceEnd := strings.Index(path, "}"); braceEnd > braceStart {
			inner := path[braceStart+1 : braceEnd]
			if _, dest, ok := strings.Cut(inner, " => "); ok {
				prefix := path[:braceStart]
				suffix := path[braceEnd+1:]
				return prefix + dest + suffix
			}
		}
	}

	// Simple: "old => new"
	if _, dest, ok := strings.Cut(path, " => "); ok {
		return dest
	}
	return ""
}

// mapGitStatus converts a git diff status letter to a human-readable status.
func mapGitStatus(code string) string {
	if len(code) == 0 {
		return StatusModified
	}

	switch code[0] {
	case 'A':
		return StatusCreated
	case 'D':
		return StatusDeleted
	case 'R':
		return StatusRenamed
	case 'C':
		return "copied"
	default:
		return StatusModified
	}
}

// GeneratedMatcher returns a predicate that reports whether a repo-relative path
// is marked linguist-generated=true, by shelling out to `git check-attr` per call.
// `git check-attr` returns "" on error (e.g. non-git directory), which degrades
// to a no-op matcher.
func (c *ExecClient) GeneratedMatcher(repoRoot string) func(path string) bool {
	return func(path string) bool {
		if path == "" {
			return false
		}

		// Output format: "<path>: linguist-generated: <value>"
		// Values: "true", "false", "set" (bare attribute), "unspecified".
		out := gitOutput(repoRoot, "check-attr", "linguist-generated", "--", path)
		if out == "" {
			return false
		}

		idx := strings.LastIndex(out, ": ")
		if idx < 0 {
			return false
		}
		value := out[idx+2:]

		return value == "true" || value == "set"
	}
}

// CommitHeadInfo returns the SHA and message of HEAD.
func (c *ExecClient) CommitHeadInfo(repoRoot string) (sha, message string, err error) {
	out := gitOutput(repoRoot, "log", "-1", "--format=%H%n%B", "HEAD")
	if out == "" {
		return "", "", fmt.Errorf("git log HEAD: no output")
	}

	sha, message, _ = strings.Cut(out, "\n")
	return sha, strings.TrimSpace(message), nil
}

// StagedFiles returns repo-relative paths of files staged for the current commit.
func (c *ExecClient) StagedFiles(repoRoot string) ([]string, error) {
	out := gitOutput(repoRoot, "diff", "--cached", "--name-only")
	if out == "" {
		return nil, nil
	}

	return strings.Split(strings.TrimRight(out, "\n"), "\n"), nil
}

// SnapshotWorktree returns a signature of the working tree (repo-relative path
// → content hash). The walk is git-binary-independent, so it shares the
// implementation used by GoGitClient.
func (c *ExecClient) SnapshotWorktree(repoRoot string) (map[string]string, error) {
	return snapshotWorktree(repoRoot)
}

// IsMergeCommit reports whether the commit at sha has more than one parent.
func (c *ExecClient) IsMergeCommit(repoRoot, sha string) (bool, error) {
	cmd := exec.Command("git", "rev-list", "--no-walk", "--parents", sha)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("rev-list %s: %w", sha, err)
	}

	// Output format: "<sha> <parent1> <parent2> ..."
	fields := strings.Fields(strings.TrimSpace(string(out)))

	return len(fields) > 2, nil
}

// BranchCommitSHAs returns the SHAs reachable from HEAD that are not reachable
// from the default remote branch. The merge base is excluded. Order matches
// `git rev-list HEAD ^<base>` (newest first). When no default remote branch
// is detected, returns every commit reachable from HEAD.
func (c *ExecClient) BranchCommitSHAs(repoRoot string) ([]string, error) {
	args := []string{"rev-list", "HEAD"}
	if base := detectBaseBranch(repoRoot); base != "" {
		args = append(args, "^"+base)
	}

	out := gitOutput(repoRoot, args...)
	if out == "" {
		return nil, nil
	}

	return strings.Split(out, "\n"), nil
}

// detectBaseBranch tries to find the default remote branch (main, master, etc.)
func detectBaseBranch(repoRoot string) string {
	ref := gitOutput(repoRoot, "symbolic-ref", "refs/remotes/origin/HEAD")
	if ref != "" {
		return ref
	}

	for _, name := range []string{"origin/main", "origin/master"} {
		out := gitOutput(repoRoot, "rev-parse", "--verify", "--quiet", name)
		if out != "" {
			return name
		}
	}

	return ""
}

// gitOutput runs a git command and returns trimmed stdout, or empty string on error.
func gitOutput(repoRoot string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(out))
}
