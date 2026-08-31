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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/materials/aicodingsession"
	"github.com/go-git/go-billy/v6/osfs"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/format/gitattributes"
	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/utils/merkletrie"
)

// openRepo opens a git repository, detecting .git from the given path or its parents.
// EnableDotGitCommonDir is set to support git worktrees.
func openRepo(path string) (*gogit.Repository, error) {
	return gogit.PlainOpenWithOptions(path, &gogit.PlainOpenOptions{
		DetectDotGit: true,
	})
}

// GoGitClient implements Client using go-git (no external git binary required).
type GoGitClient struct{}

var _ Client = (*GoGitClient)(nil)

func NewGoGitClient() *GoGitClient { return &GoGitClient{} }

func (c *GoGitClient) Context(repoRoot string) (*aicodingsession.GitContext, error) {
	ctx := &aicodingsession.GitContext{WorkDir: repoRoot}

	repo, err := openRepo(repoRoot)
	if err != nil {
		return ctx, nil
	}

	// Remote URL
	remote, err := repo.Remote("origin")
	if err == nil && len(remote.Config().URLs) > 0 {
		ctx.Repository = remote.Config().URLs[0]
	}

	// Branch
	head, err := repo.Head()
	if err != nil {
		return ctx, nil
	}
	if head.Name().IsBranch() {
		ctx.Branch = head.Name().Short()
	} else {
		ctx.Branch = "HEAD"
	}

	return ctx, nil
}

func (c *GoGitClient) CodeChanges(repoRoot string) (*aicodingsession.CodeChanges, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return &aicodingsession.CodeChanges{}, nil
	}

	head, err := repo.Head()
	if err != nil {
		return &aicodingsession.CodeChanges{}, nil
	}

	baseBranch := detectBaseBranchGoGit(repo)
	if baseBranch == nil {
		return &aicodingsession.CodeChanges{}, nil
	}

	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return &aicodingsession.CodeChanges{}, nil
	}

	baseCommit, err := repo.CommitObject(baseBranch.Hash())
	if err != nil {
		return &aicodingsession.CodeChanges{}, nil
	}

	mergeBaseCommits, err := headCommit.MergeBase(baseCommit)
	if err != nil || len(mergeBaseCommits) == 0 {
		return &aicodingsession.CodeChanges{}, nil
	}

	return diffTrees(repo, mergeBaseCommits[0].Hash, head.Hash())
}

func (c *GoGitClient) CodeChangesForRange(repoRoot, startSHA, endSHA string) (*aicodingsession.CodeChanges, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return &aicodingsession.CodeChanges{}, nil
	}

	// We want changes introduced by startSHA through endSHA (startSHA^..endSHA).
	startCommit, err := repo.CommitObject(plumbing.NewHash(startSHA))
	if err != nil {
		return &aicodingsession.CodeChanges{}, nil
	}

	// Use the first parent of startSHA as the base
	var baseHash plumbing.Hash
	parents := startCommit.Parents()
	parent, err := parents.Next()
	if err == nil {
		baseHash = parent.Hash
	}
	// If no parent (root commit), baseHash is zero-value which gives an empty tree

	return diffTrees(repo, baseHash, plumbing.NewHash(endSHA))
}

// CodeChangesForCommits computes the union of per-commit (parent..commit)
// diffs for the given SHAs, summing line counts per file path. This is what
// runTracePush needs when a session's commits are non-contiguous on the branch
// — a SHA range diff would over-count by including unrelated commits in the
// gap. Unresolvable SHAs are skipped (best-effort) rather than failing the
// whole computation; that lets attestation continue when a stale CommitRecord
// references a SHA that's been pruned by git.
func (c *GoGitClient) CodeChangesForCommits(repoRoot string, shas []string) (*aicodingsession.CodeChanges, error) {
	out := &aicodingsession.CodeChanges{}
	if len(shas) == 0 {
		return out, nil
	}

	repo, err := openRepo(repoRoot)
	if err != nil {
		return out, nil
	}

	byPath := make(map[string]*fileAgg)
	for _, sha := range shas {
		commit, err := repo.CommitObject(plumbing.NewHash(sha))
		if err != nil {
			continue
		}

		var baseHash plumbing.Hash
		parents := commit.Parents()
		if parent, err := parents.Next(); err == nil {
			baseHash = parent.Hash
		}

		stats, err := diffTrees(repo, baseHash, commit.Hash)
		if err != nil {
			continue
		}
		addCommitStats(out, byPath, stats)
	}
	finalizeAggregatedFiles(out, byPath)

	return out, nil
}

func (c *GoGitClient) CommitHeadInfo(repoRoot string) (sha, message string, err error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return "", "", fmt.Errorf("open repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", "", fmt.Errorf("get HEAD: %w", err)
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", "", fmt.Errorf("get commit: %w", err)
	}

	return head.Hash().String(), strings.TrimSpace(commit.Message), nil
}

// GeneratedMatcher returns a predicate that reports whether a repo-relative path
// is marked linguist-generated=true in the repository's .gitattributes, parsed
// in-process via go-git's gitattributes parser.
func (c *GoGitClient) GeneratedMatcher(repoRoot string) func(path string) bool {
	fs := osfs.New(repoRoot)
	// allowMacro=false: macros are only valid in .git/info/attributes
	attrs, err := gitattributes.ReadAttributesFile(fs, nil, ".gitattributes", false)
	if err != nil || len(attrs) == 0 {
		return func(string) bool { return false }
	}

	// go-git's Matcher.Match iterates the stack from end to start and overwrites
	// the result map on each match. With Match(parts, nil) the loop runs to
	// completion, so the LAST iteration that matches wins — i.e. the FIRST entry
	// in the stack. To get git's documented "last pattern in the file wins"
	// semantics, reverse the stack here. Only safe because we always pass a nil
	// attribute filter; a non-nil filter would invert the early-return logic.
	for i, j := 0, len(attrs)-1; i < j; i, j = i+1, j-1 {
		attrs[i], attrs[j] = attrs[j], attrs[i]
	}

	matcher := gitattributes.NewMatcher(attrs)
	return func(path string) bool {
		if path == "" {
			return false
		}
		parts := strings.Split(path, "/")
		result, matched := matcher.Match(parts, nil)
		if !matched {
			return false
		}
		attr, ok := result["linguist-generated"]
		if !ok {
			return false
		}
		// `linguist-generated` alone is treated as set; `linguist-generated=true`
		// is the explicit form used by GitHub Linguist.
		if attr.IsSet() {
			return true
		}
		if attr.IsValueSet() && strings.EqualFold(attr.Value(), "true") {
			return true
		}

		return false
	}
}

// StagedFiles returns repo-relative paths of files staged for the current commit.
func (c *GoGitClient) StagedFiles(repoRoot string) ([]string, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return nil, fmt.Errorf("get status: %w", err)
	}

	var files []string
	for path, s := range status {
		if s.Staging != gogit.Unmodified && s.Staging != gogit.Untracked {
			files = append(files, path)
		}
	}

	sort.Strings(files)

	return files, nil
}

// SnapshotWorktree returns a signature of the working tree (repo-relative path
// → SHA-256 content hash), honoring .gitignore and skipping the .git directory.
func (c *GoGitClient) SnapshotWorktree(repoRoot string) (map[string]string, error) {
	return snapshotWorktree(repoRoot)
}

// snapshotWorktree walks repoRoot and hashes every regular, non-ignored file.
// gitignore patterns are read in-process via go-git's parser (best-effort: an
// unreadable .gitignore yields no patterns rather than an error). Symlinks and
// unreadable files are skipped. The walk never fails the caller — attribution
// is best-effort and must not block the agent.
func snapshotWorktree(repoRoot string) (map[string]string, error) {
	wtfs := osfs.New(repoRoot)
	patterns, _ := gitignore.ReadPatterns(wtfs, nil)
	matcher := gitignore.NewMatcher(patterns)
	tracked := trackedPaths(repoRoot)

	sig := make(map[string]string)
	err := filepath.WalkDir(repoRoot, func(p string, d iofs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries, keep walking
		}

		rel, relErr := filepath.Rel(repoRoot, p)
		if relErr != nil || rel == "." {
			return nil
		}
		slashRel := filepath.ToSlash(rel)
		parts := strings.Split(slashRel, "/")

		if d.IsDir() {
			if d.Name() == ".git" || matcher.Match(parts, true) {
				return filepath.SkipDir
			}

			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}
		// Ignore rules apply only to untracked files, matching git: a tracked
		// file that also matches a .gitignore pattern (e.g. force-added
		// generated code) is still part of the working tree and must be
		// snapshotted so edits to it are attributed. (Tracked files nested
		// inside an ignored directory are the one gap — that directory is
		// pruned above for walk performance.)
		if matcher.Match(parts, false) && !tracked[slashRel] {
			return nil
		}

		h, herr := hashFileContent(p)
		if herr != nil {
			return nil
		}
		sig[slashRel] = h

		return nil
	})

	return sig, err
}

// trackedPaths returns the set of repo-relative paths (forward-slash separated)
// recorded in the git index. Best-effort: returns nil when the repository or
// its index cannot be read, in which case ignore rules apply to every file.
func trackedPaths(repoRoot string) map[string]bool {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil
	}

	idx, err := repo.Storer.Index()
	if err != nil {
		return nil
	}

	tracked := make(map[string]bool, len(idx.Entries))
	for _, e := range idx.Entries {
		tracked[e.Name] = true
	}

	return tracked
}

// hashFileContent returns the hex-encoded SHA-256 of the file's content.
func hashFileContent(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// IsMergeCommit reports whether the commit at sha has more than one parent.
func (c *GoGitClient) IsMergeCommit(repoRoot, sha string) (bool, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return false, fmt.Errorf("open repo: %w", err)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(sha))
	if err != nil {
		return false, fmt.Errorf("resolve commit %s: %w", sha, err)
	}

	return commit.NumParents() > 1, nil
}

// BranchCommitSHAs returns the SHAs reachable from HEAD that are not reachable
// from the default remote branch. The merge base is excluded. Order is
// unspecified.
func (c *GoGitClient) BranchCommitSHAs(repoRoot string) ([]string, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	headCommit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, fmt.Errorf("resolve HEAD commit: %w", err)
	}

	// Pre-seed the iterator's seen-set with the merge base(s). The preorder
	// iter skips any commit already in seen and stops descending into its
	// parents, so a single walk from HEAD naturally prunes at the merge base.
	seen := make(map[plumbing.Hash]bool)
	if baseRef := detectBaseBranchGoGit(repo); baseRef != nil {
		if baseCommit, err := repo.CommitObject(baseRef.Hash()); err == nil {
			if mergeBases, err := headCommit.MergeBase(baseCommit); err == nil {
				for _, mb := range mergeBases {
					seen[mb.Hash] = true
				}
			}
		}
	}

	var shas []string
	iter := object.NewCommitPreorderIter(headCommit, seen, nil)
	if err := iter.ForEach(func(c *object.Commit) error {
		shas = append(shas, c.Hash.String())

		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk commits: %w", err)
	}

	return shas, nil
}

// LocalReachableSHAs walks every local branch tip (refs/heads/*) and returns
// the union of reachable SHAs. The shared `seen` map both deduplicates work
// across branch walks (preorder iter skips already-seen commits and stops
// descending into their parents) and records every visited commit so the
// caller can build the live-SHA set.
func (c *GoGitClient) LocalReachableSHAs(repoRoot string) (map[string]bool, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	branches, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	defer branches.Close()

	// go-git's NewCommitPreorderIter accepts `seen` as an input-only skip set
	// and does not mutate it. Track visits explicitly via the callback so the
	// resulting set really is "everything reachable".
	seen := make(map[plumbing.Hash]bool)
	if err := branches.ForEach(func(ref *plumbing.Reference) error {
		commit, err := repo.CommitObject(ref.Hash())
		if err != nil {
			// Branch points at a non-commit (or pruned object) — skip rather
			// than fail the whole GC.
			return nil
		}
		iter := object.NewCommitPreorderIter(commit, seen, nil)

		return iter.ForEach(func(c *object.Commit) error {
			seen[c.Hash] = true

			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("walk branches: %w", err)
	}

	out := make(map[string]bool, len(seen))
	for h := range seen {
		out[h.String()] = true
	}

	return out, nil
}

// detectBaseBranchGoGit finds the default remote branch reference.
func detectBaseBranchGoGit(repo *gogit.Repository) *plumbing.Reference {
	// Try refs/remotes/origin/HEAD
	ref, err := repo.Reference(plumbing.NewRemoteReferenceName("origin", "HEAD"), true)
	if err == nil {
		return ref
	}

	// Fallback to origin/main, then origin/master
	for _, name := range []string{"main", "master"} {
		ref, err = repo.Reference(plumbing.NewRemoteReferenceName("origin", name), true)
		if err == nil {
			return ref
		}
	}

	return nil
}

// diffTrees computes diff stats between two commits identified by hash.
// A zero-value baseHash means diff against an empty tree.
// Note: go-git's tree diff does not detect renames/copies; they appear as delete+create.
func diffTrees(repo *gogit.Repository, baseHash, headHash plumbing.Hash) (*aicodingsession.CodeChanges, error) {
	info := &aicodingsession.CodeChanges{}

	headCommit, err := repo.CommitObject(headHash)
	if err != nil {
		return info, nil
	}
	headTree, err := headCommit.Tree()
	if err != nil {
		return info, nil
	}

	var baseTree *object.Tree
	if !baseHash.IsZero() {
		baseCommit, err := repo.CommitObject(baseHash)
		if err != nil {
			return info, nil
		}
		baseTree, err = baseCommit.Tree()
		if err != nil {
			return info, nil
		}
	}

	changes, err := object.DiffTree(baseTree, headTree)
	if err != nil {
		return info, nil
	}

	// Collect file statuses from changes
	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			continue
		}

		var status, path string
		switch action {
		case merkletrie.Insert:
			status = StatusCreated
			path = change.To.Name
			info.FilesCreated++
		case merkletrie.Delete:
			status = StatusDeleted
			path = change.From.Name
			info.FilesDeleted++
		default:
			status = StatusModified
			path = change.To.Name
			info.FilesModified++
		}

		info.Files = append(info.Files, aicodingsession.FileChange{
			Path:   path,
			Status: status,
		})
	}

	// Get line stats from patch and populate per-file counts
	patch, err := changes.Patch()
	if err != nil {
		return info, nil
	}
	perFileStats := make(map[string][2]int) // path → [added, removed]
	for _, stat := range patch.Stats() {
		info.LinesAdded += stat.Addition
		info.LinesRemoved += stat.Deletion
		perFileStats[stat.Name] = [2]int{stat.Addition, stat.Deletion}
	}
	for i := range info.Files {
		if stats, ok := perFileStats[info.Files[i].Path]; ok {
			info.Files[i].LinesAdded = stats[0]
			info.Files[i].LinesRemoved = stats[1]
		}
	}

	return info, nil
}
