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
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

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

	changes, err := object.DiffTree(baseTree, tree)
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
		if to != nil {
			fl.Path = to.Path()
		} else if from != nil {
			fl.Path = from.Path()
		}

		for _, chunk := range fp.Chunks() {
			switch chunk.Type() {
			case fdiff.Add:
				fl.Added = append(fl.Added, splitLines(chunk.Content())...)
			case fdiff.Delete:
				fl.Removed = append(fl.Removed, splitLines(chunk.Content())...)
			case fdiff.Equal:
			}
		}
		out = append(out, fl)
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
		"--no-renames", "-U0", base, sha)
	if err != nil {
		return nil, fmt.Errorf("diff %s: %w", sha, err)
	}
	patch := string(raw)

	var out []FileLines
	var cur *FileLines
	var binary, inHunk bool
	flush := func() {
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
			case strings.HasPrefix(line, "Binary files "):
				binary = true
			case strings.HasPrefix(line, "@@"):
				inHunk = true
			}

			continue
		}

		switch {
		case strings.HasPrefix(line, "@@"):
		case strings.HasPrefix(line, "+"):
			cur.Added = append(cur.Added, line[1:])
		case strings.HasPrefix(line, "-"):
			cur.Removed = append(cur.Removed, line[1:])
		}
	}
	flush()

	return out, nil
}

// FileAt returns the content of path at the commit, or nil when the commit
// does not hold it.
func (c *ExecClient) FileAt(repoRoot, sha, path string) ([]byte, error) {
	if _, err := gitRawOutput(repoRoot, "rev-parse", "--verify", "--quiet", sha+"^{commit}"); err != nil {
		return nil, fmt.Errorf("resolve %s: %w", sha, err)
	}
	if _, err := gitRawOutput(repoRoot, "cat-file", "-e", sha+":"+path); err != nil {
		return nil, nil //nolint:nilerr // the commit resolves, so a failure here means the path is absent
	}

	out, err := gitRawOutput(repoRoot, "cat-file", "blob", sha+":"+path)
	if err != nil {
		return nil, fmt.Errorf("read %s at %s: %w", path, sha, err)
	}

	return out, nil
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
