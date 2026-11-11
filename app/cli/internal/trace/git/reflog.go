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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// ErrReflogRewritten reports that the HEAD reflog no longer holds a position
// read from it earlier: something expired or rewrote it in between, so the
// position no longer marks where the new entries start.
var ErrReflogRewritten = errors.New("HEAD reflog was rewritten")

// ReflogEntry is one move of HEAD, as the reflog records it.
type ReflogEntry struct {
	// Old and New are the commits HEAD pointed to before and after the move.
	Old, New string
	// Message says what moved HEAD, for example "commit: <subject>" or
	// "rebase (finish): returning to refs/heads/main".
	Message string
}

// ReflogPos marks the end of the HEAD reflog at one moment. Taken before a
// command, it marks where the entries that the command writes start.
type ReflogPos struct {
	// Offset is the size in bytes of the reflog file, with the files ref
	// storage.
	Offset int64 `json:"offset,omitempty"`
	// Top is the newest entry, as git prints it, with the reftable ref
	// storage. It is empty when the reflog has no entry.
	Top string `json:"top,omitempty"`
}

// IsZeroOID reports whether oid is the all-zero object ID that a reflog
// entry holds for "no commit", as the old side of the first commit.
func IsZeroOID(oid string) bool {
	return oid != "" && strings.Trim(oid, "0") == ""
}

// headReflog locates the HEAD reflog of the checkout at repoRoot. A linked
// worktree has its own HEAD, and its own reflog. With the files ref storage
// the reflog is the file logs/HEAD in the checkout's git dir. With the
// reftable ref storage (extensions.refStorage) it lives in binary tables that
// only git reads, and reftable is true.
func headReflog(repoRoot string) (path string, reftable bool, err error) {
	gitDir, _, err := FindGitDirAndRootFrom(repoRoot)
	if err != nil {
		return "", false, err
	}

	commonDir := gitDir
	if data, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		commonDir = strings.TrimSpace(string(data))
		if !filepath.IsAbs(commonDir) {
			commonDir = filepath.Join(gitDir, commonDir)
		}
	}
	if fi, err := os.Stat(filepath.Join(commonDir, "reftable")); err == nil && fi.IsDir() { //nolint:gosec // the common dir that git wrote for this checkout
		return "", true, nil
	}

	return filepath.Join(gitDir, "logs", "HEAD"), false, nil
}

// HeadReflogPos returns the position of the end of the HEAD reflog of the
// checkout at repoRoot. It is cheap enough for a hook that runs before every
// shell command, whatever the length of the reflog: a stat with the files ref
// storage, and one entry read through git with reftable.
func HeadReflogPos(repoRoot string) (ReflogPos, error) {
	path, reftable, err := headReflog(repoRoot)
	if err != nil {
		return ReflogPos{}, err
	}
	if reftable {
		lines, err := gitReflogLines(repoRoot, 1)
		if err != nil || len(lines) == 0 {
			return ReflogPos{}, err
		}

		return ReflogPos{Top: lines[0].line}, nil
	}

	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ReflogPos{}, nil
	}
	if err != nil {
		return ReflogPos{}, fmt.Errorf("stat HEAD reflog: %w", err)
	}

	return ReflogPos{Offset: fi.Size()}, nil
}

// HeadReflogSince returns the HEAD reflog entries written after pos, a
// position that HeadReflogPos returned earlier, oldest first. It returns
// ErrReflogRewritten when the reflog no longer holds pos.
func HeadReflogSince(repoRoot string, pos ReflogPos) ([]ReflogEntry, error) {
	path, reftable, err := headReflog(repoRoot)
	if err != nil {
		return nil, err
	}
	if reftable {
		return gitHeadReflogSince(repoRoot, pos.Top)
	}

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		if pos.Offset > 0 {
			return nil, ErrReflogRewritten
		}

		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open HEAD reflog: %w", err)
	}
	defer func() { _ = f.Close() }()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat HEAD reflog: %w", err)
	}
	if fi.Size() < pos.Offset {
		return nil, ErrReflogRewritten
	}
	if _, err := f.Seek(pos.Offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek HEAD reflog: %w", err)
	}

	var entries []ReflogEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		entry, ok := parseReflogLine(sc.Text())
		if !ok {
			return nil, fmt.Errorf("parse HEAD reflog line %q", sc.Text())
		}
		entries = append(entries, entry)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read HEAD reflog: %w", err)
	}

	return entries, nil
}

// reflogLine is one HEAD reflog entry as git prints it.
type reflogLine struct {
	// line is the whole entry: the commit, the selector with the time of the
	// entry, and the message. It identifies the entry as a ReflogPos.Top.
	line string
	// newOID is the commit that the entry moved HEAD to.
	newOID  string
	message string
}

// gitReflogLines returns the newest n entries of the HEAD reflog through git,
// newest first, for a ref storage that only git reads. The selector holds the
// time of the entry, not its index, so the line of an entry stays the same
// when newer entries are added. An unborn HEAD with no reflog yet has no
// entries.
func gitReflogLines(repoRoot string, n int) ([]reflogLine, error) {
	cmd := exec.Command("git", "reflog", "show", "-n", strconv.Itoa(n), "--date=unix", "--format=%H%x09%gd%x09%gs", "HEAD", "--") //nolint:gosec // a fixed git command; n is a number
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if hasHead := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", "-q", "HEAD").Run() == nil; !hasHead {
			return nil, nil
		}

		return nil, fmt.Errorf("git reflog HEAD: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var lines []reflogLine
	for line := range strings.Lines(string(out)) {
		line = strings.TrimSuffix(line, "\n")
		fields := strings.SplitN(line, "\t", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("parse git reflog line %q", line)
		}
		lines = append(lines, reflogLine{line: line, newOID: fields[0], message: fields[2]})
	}

	return lines, nil
}

// gitHeadReflogSince is HeadReflogSince for a ref storage that only git reads.
// top is the newest entry before the command, empty when there was none. It
// reads the reflog newest first, twice as many entries each time, until it
// finds top: a command that wrote few entries costs one read, whatever the
// length of the reflog.
//
// git prints the commit that each entry moved HEAD to, not the one it left.
// The old side of an entry is the new side of the entry before it, or the zero
// ID for the first entry of the reflog.
func gitHeadReflogSince(repoRoot, top string) ([]ReflogEntry, error) {
	for n := 16; ; n *= 2 {
		lines, err := gitReflogLines(repoRoot, n)
		if err != nil {
			return nil, err
		}

		end := -1
		if top != "" {
			end = slices.IndexFunc(lines, func(l reflogLine) bool { return l.line == top })
		}
		if end < 0 {
			if len(lines) == n {
				continue // more entries to read
			}
			if top != "" {
				return nil, ErrReflogRewritten
			}
			end = len(lines)
		}

		added := make([]ReflogEntry, 0, end)
		for i := end - 1; i >= 0; i-- {
			old := strings.Repeat("0", len(lines[i].newOID))
			if i+1 < len(lines) {
				old = lines[i+1].newOID
			}
			added = append(added, ReflogEntry{Old: old, New: lines[i].newOID, Message: lines[i].message})
		}

		return added, nil
	}
}

// CommitPaths returns the repo-relative paths, with forward slashes, that
// differ between the commits from and to. A zero from is the first commit of
// a branch, and every path of to then counts.
func CommitPaths(repoRoot, from, to string) (map[string]bool, error) {
	args := []string{"diff-tree", "-r", "--no-renames", "--no-commit-id", "--name-only", "-z"}
	if IsZeroOID(from) {
		args = append(args, "--root", to)
	} else {
		args = append(args, from, to)
	}
	out, err := gitRawOutput(repoRoot, args...)
	if err != nil {
		return nil, fmt.Errorf("git diff-tree %s %s: %w", from, to, err)
	}

	paths := make(map[string]bool)
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			paths[p] = true
		}
	}

	return paths, nil
}

// parseReflogLine parses one reflog line:
// "<old> <new> <name> <<email>> <time> <tz>\t<message>".
func parseReflogLine(line string) (ReflogEntry, bool) {
	head, msg, _ := strings.Cut(line, "\t")
	fields := strings.Fields(head)
	if len(fields) < 2 {
		return ReflogEntry{}, false
	}

	return ReflogEntry{Old: fields[0], New: fields[1], Message: msg}, true
}
