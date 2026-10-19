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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	fdiff "github.com/go-git/go-git/v6/plumbing/format/diff"
	"github.com/go-git/go-git/v6/plumbing/object"
)

// CommitLines returns the lines that the commit added and removed, per file,
// against its first parent.
func (c *GoGitClient) CommitLines(repoRoot, sha string) ([]FileLines, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	commit, err := repo.CommitObject(plumbing.NewHash(sha))
	if err != nil {
		return nil, fmt.Errorf("get commit %s: %w", sha, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("get tree of %s: %w", sha, err)
	}

	// A root commit has no parent, and its diff is against the empty tree.
	var baseTree *object.Tree
	parent, err := commit.Parents().Next()
	switch {
	case errors.Is(err, io.EOF):
	case err != nil:
		return nil, fmt.Errorf("get parent of %s: %w", sha, err)
	default:
		if baseTree, err = parent.Tree(); err != nil {
			return nil, fmt.Errorf("get parent tree of %s: %w", sha, err)
		}
	}

	changes, err := object.DiffTreeWithOptions(context.Background(), baseTree, tree, object.DefaultDiffTreeOptions)
	if err != nil {
		return nil, fmt.Errorf("diff %s: %w", sha, err)
	}
	patch, err := changes.Patch()
	if err != nil {
		return nil, fmt.Errorf("patch %s: %w", sha, err)
	}

	var out []FileLines
	for _, fp := range patch.FilePatches() {
		if fp.IsBinary() {
			continue
		}

		from, to := fp.Files()
		fl := FileLines{}
		switch {
		case to == nil:
			fl.Path = from.Path()
		case from != nil && from.Path() != to.Path():
			fl.Path, fl.OldPath = to.Path(), from.Path()
		default:
			fl.Path = to.Path()
		}

		// A hunk is a run of removed and added chunks between two equal ones.
		// oldLine is the line of the parent's file that the next chunk starts
		// at.
		oldLine := 1
		var cur *Hunk
		closeHunk := func() {
			if cur != nil {
				fl.Hunks = append(fl.Hunks, *cur)
				cur = nil
			}
		}
		for _, chunk := range fp.Chunks() {
			lines := splitLines(chunk.Content())
			switch chunk.Type() {
			case fdiff.Equal:
				closeHunk()
				oldLine += len(lines)
			case fdiff.Delete:
				if cur == nil {
					cur = &Hunk{OldStart: oldLine}
				}
				cur.Removed = append(cur.Removed, lines...)
				fl.Removed = append(fl.Removed, lines...)
				oldLine += len(lines)
			case fdiff.Add:
				if cur == nil {
					cur = &Hunk{OldStart: oldLine}
				}
				cur.Added = append(cur.Added, lines...)
				fl.Added = append(fl.Added, lines...)
			}
		}
		closeHunk()
		out = append(out, fl)
	}

	return out, nil
}

// Renames returns the new path of each file renamed between two revisions, by
// its old path.
func (c *GoGitClient) Renames(repoRoot, from, to string) (map[string]string, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	trees := make([]*object.Tree, 0, 2)
	for _, rev := range []string{from, to} {
		hash, err := repo.ResolveRevision(plumbing.Revision(rev))
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", rev, err)
		}
		commit, err := repo.CommitObject(*hash)
		if err != nil {
			return nil, fmt.Errorf("get commit %s: %w", rev, err)
		}
		tree, err := commit.Tree()
		if err != nil {
			return nil, fmt.Errorf("get tree of %s: %w", rev, err)
		}
		trees = append(trees, tree)
	}

	changes, err := object.DiffTreeWithOptions(context.Background(), trees[0], trees[1], object.DefaultDiffTreeOptions)
	if err != nil {
		return nil, fmt.Errorf("diff %s %s: %w", from, to, err)
	}

	out := make(map[string]string)
	for _, ch := range changes {
		if ch.From.Name != "" && ch.To.Name != "" && ch.From.Name != ch.To.Name {
			out[ch.From.Name] = ch.To.Name
		}
	}

	return out, nil
}

// FileAt returns the content of path at the commit, or nil when the commit
// does not hold it. sha can be any revision, such as HEAD.
func (c *GoGitClient) FileAt(repoRoot, sha, path string) ([]byte, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	hash, err := repo.ResolveRevision(plumbing.Revision(sha))
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", sha, err)
	}
	commit, err := repo.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("get commit %s: %w", sha, err)
	}

	file, err := commit.File(path)
	if errors.Is(err, object.ErrFileNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get %s at %s: %w", path, sha, err)
	}

	content, err := file.Contents()
	if err != nil {
		return nil, fmt.Errorf("read %s at %s: %w", path, sha, err)
	}

	return []byte(content), nil
}

// CommitLines parses `git diff -U0` of the commit against its first parent.
func (c *ExecClient) CommitLines(repoRoot, sha string) ([]FileLines, error) {
	base := commitParentOrEmptyTree(repoRoot, sha)
	raw, err := gitRawOutput(repoRoot, "-c", "core.quotePath=false", "diff", "--no-color", "--no-ext-diff",
		"-M", "-U0", base, sha)
	if err != nil {
		return nil, fmt.Errorf("diff %s: %w", sha, err)
	}
	patch := string(raw)

	var out []FileLines
	var cur *FileLines
	var hunk *Hunk
	var binary, inHunk bool
	closeHunk := func() {
		if cur != nil && hunk != nil {
			cur.Hunks = append(cur.Hunks, *hunk)
		}
		hunk = nil
	}
	flush := func() {
		closeHunk()
		if cur != nil && !binary {
			out = append(out, *cur)
		}
	}

	for _, line := range strings.Split(patch, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			cur, binary, inHunk = &FileLines{}, false, false

			continue
		}
		if cur == nil {
			continue
		}

		// Before the first hunk, the lines are headers. In a hunk, a removed
		// line that starts with "-- " looks like a header, so the two are kept
		// apart by position. Git ends a header path that holds a space with a
		// tab.
		if !inHunk {
			switch {
			case strings.HasPrefix(line, "--- a/"):
				cur.Path = strings.TrimSuffix(strings.TrimPrefix(line, "--- a/"), "\t")
			case strings.HasPrefix(line, "+++ b/"):
				cur.Path = strings.TrimSuffix(strings.TrimPrefix(line, "+++ b/"), "\t")
			// A rename with no changed line has no "---" and "+++" headers.
			case strings.HasPrefix(line, "rename from "):
				cur.OldPath = strings.TrimPrefix(line, "rename from ")
			case strings.HasPrefix(line, "rename to "):
				cur.Path = strings.TrimPrefix(line, "rename to ")
			case strings.HasPrefix(line, "Binary files "):
				binary = true
			case strings.HasPrefix(line, "@@"):
				inHunk = true
				hunk = &Hunk{OldStart: hunkOldStart(line)}
			}

			continue
		}

		switch {
		case strings.HasPrefix(line, "@@"):
			closeHunk()
			hunk = &Hunk{OldStart: hunkOldStart(line)}
		case strings.HasPrefix(line, "+"):
			cur.Added = append(cur.Added, line[1:])
			hunk.Added = append(hunk.Added, line[1:])
		case strings.HasPrefix(line, "-"):
			cur.Removed = append(cur.Removed, line[1:])
			hunk.Removed = append(hunk.Removed, line[1:])
		}
	}
	flush()

	return out, nil
}

// Renames reads `git diff --name-status` between two revisions, with rename
// detection on, whatever the configuration of the user says.
func (c *ExecClient) Renames(repoRoot, from, to string) (map[string]string, error) {
	raw, err := gitRawOutput(repoRoot, "diff", "--no-ext-diff", "-M", "--name-status", "-z", "--diff-filter=R", from, to)
	if err != nil {
		return nil, fmt.Errorf("diff %s %s: %w", from, to, err)
	}

	// Each rename is three fields: its status, the old path, and the new
	// path.
	fields := bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0})
	out := make(map[string]string)
	for i := 0; i+2 < len(fields); i += 3 {
		out[string(fields[i+1])] = string(fields[i+2])
	}

	return out, nil
}

// hunkOldStart returns the first line of the parent's file that a hunk
// header "@@ -a,b +c,d @@" covers. A hunk that removes nothing gives a as the
// line before the insertion, so the hunk starts at a+1.
func hunkOldStart(header string) int {
	fields := strings.Fields(header)
	if len(fields) < 2 {
		return 0
	}

	start, count, hasCount := strings.Cut(strings.TrimPrefix(fields[1], "-"), ",")
	a, err := strconv.Atoi(start)
	if err != nil {
		return 0
	}
	if hasCount && count == "0" {
		return a + 1
	}

	return a
}

// FileAt returns the content of path at the commit, or nil when the commit
// does not hold it.
func (c *ExecClient) FileAt(repoRoot, sha, path string) ([]byte, error) {
	exists, err := pathAt(repoRoot, sha, path)
	if err != nil || !exists {
		return nil, err
	}

	out, err := gitRawOutput(repoRoot, "cat-file", "blob", sha+":"+path)
	if err != nil {
		return nil, fmt.Errorf("read %s at %s: %w", path, sha, err)
	}

	return out, nil
}

// pathAt reports whether the commit holds path, without reading its content.
// An error means the commit does not resolve.
func pathAt(repoRoot, sha, path string) (bool, error) {
	if _, err := gitRawOutput(repoRoot, "rev-parse", "--verify", "--quiet", sha+"^{commit}"); err != nil {
		return false, fmt.Errorf("resolve %s: %w", sha, err)
	}
	if _, err := gitRawOutput(repoRoot, "cat-file", "-e", sha+":"+path); err != nil {
		return false, nil //nolint:nilerr // the commit resolves, so a failure here means the path is absent
	}

	return true, nil
}

// gitRawOutput runs git and returns its output unchanged. gitOutput trims
// it, which would change file content.
func gitRawOutput(repoRoot string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot

	return cmd.Output()
}

// splitLines splits text into lines without their line ends. A final line
// with no newline is a line too, and empty text has no lines.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}

	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\n")
	}

	return lines
}

// Blame returns the last writer of each line of path at rev. On a long history
// it is many times slower than the git binary, which is why NewGitClient
// prefers the exec client.
func (c *GoGitClient) Blame(repoRoot, rev, path string) ([]BlameLine, error) {
	return goGitBlame(repoRoot, rev, path)
}

// Blame returns the last writer of each line of path at rev.
func (c *ExecClient) Blame(repoRoot, rev, path string) ([]BlameLine, error) {
	return blamePorcelain(repoRoot, rev, path)
}

// blamePorcelain runs `git blame --porcelain` and reads the commit and the
// text of each line. A path that rev does not hold has no lines.
func blamePorcelain(repoRoot, rev, path string) ([]BlameLine, error) {
	exists, err := pathAt(repoRoot, rev, path)
	if err != nil || !exists {
		return nil, err
	}

	raw, err := gitRawOutput(repoRoot, "blame", "--porcelain", rev, "--", path)
	if err != nil {
		return nil, fmt.Errorf("blame %s at %s: %w", path, rev, err)
	}

	// Each line starts with a header "<sha> <orig> <final> [<count>]",
	// followed on the first use of a commit by key-value lines, and ends with
	// its text after a tab.
	var out []BlameLine
	var sha string
	for _, line := range strings.Split(string(raw), "\n") {
		if text, ok := strings.CutPrefix(line, "\t"); ok {
			out = append(out, BlameLine{SHA: sha, Text: text})
			continue
		}
		if fields := strings.Fields(line); len(fields) >= 3 && len(fields[0]) == 40 && isHex(fields[0]) {
			sha = fields[0]
		}
	}

	return out, nil
}

// goGitBlame blames with go-git.
func goGitBlame(repoRoot, rev, path string) ([]BlameLine, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	hash, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", rev, err)
	}
	commit, err := repo.CommitObject(*hash)
	if err != nil {
		return nil, fmt.Errorf("get commit %s: %w", rev, err)
	}
	if _, err := commit.File(path); errors.Is(err, object.ErrFileNotFound) {
		return nil, nil
	}

	res, err := gogit.Blame(commit, path)
	if err != nil {
		return nil, fmt.Errorf("blame %s at %s: %w", path, rev, err)
	}

	out := make([]BlameLine, 0, len(res.Lines))
	for _, l := range res.Lines {
		out = append(out, BlameLine{SHA: l.Hash.String(), Text: l.Text})
	}

	return out, nil
}

// isHex reports whether s holds only lowercase hexadecimal digits.
func isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}

	return true
}
