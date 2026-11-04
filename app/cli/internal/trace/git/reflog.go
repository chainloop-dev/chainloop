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
	"strings"
)

// ErrReflogRewritten reports that the HEAD reflog is shorter than an offset
// read from it earlier: something expired or rewrote it in between, so the
// offset no longer marks where the new entries start.
var ErrReflogRewritten = errors.New("HEAD reflog was rewritten")

// ReflogEntry is one move of HEAD, as the reflog records it.
type ReflogEntry struct {
	// Old and New are the commits HEAD pointed to before and after the move.
	Old, New string
	// Message says what moved HEAD, for example "commit: <subject>" or
	// "rebase (finish): returning to refs/heads/main".
	Message string
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
	if fi, err := os.Stat(filepath.Join(commonDir, "reftable")); err == nil && fi.IsDir() {
		return "", true, nil
	}

	return filepath.Join(gitDir, "logs", "HEAD"), false, nil
}

// HeadReflogSize returns the position of the end of the HEAD reflog of the
// checkout at repoRoot, zero when it has none yet. Taken before a command, it
// marks where the entries that the command writes start. With the files ref
// storage it is the size of the reflog in bytes: a stat, cheap enough for a
// hook that runs before every shell command. With reftable it is the number of
// entries, which git has to count.
func HeadReflogSize(repoRoot string) (int64, error) {
	path, reftable, err := headReflog(repoRoot)
	if err != nil {
		return 0, err
	}
	if reftable {
		entries, err := gitHeadReflog(repoRoot)
		if err != nil {
			return 0, err
		}

		return int64(len(entries)), nil
	}

	fi, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("stat HEAD reflog: %w", err)
	}

	return fi.Size(), nil
}

// HeadReflogSince returns the HEAD reflog entries written after offset, a
// position that HeadReflogSize returned earlier, oldest first. It returns
// ErrReflogRewritten when the reflog is now shorter than offset.
func HeadReflogSince(repoRoot string, offset int64) ([]ReflogEntry, error) {
	path, reftable, err := headReflog(repoRoot)
	if err != nil {
		return nil, err
	}
	if reftable {
		return gitHeadReflogSince(repoRoot, offset)
	}

	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		if offset > 0 {
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
	if fi.Size() < offset {
		return nil, ErrReflogRewritten
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
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

// gitHeadReflog returns the HEAD reflog entries through git, newest first, for
// a ref storage that only git reads. git prints the commit that each entry
// moved HEAD to, not the one it left. The old side of an entry is then the new
// side of the entry before it, or the zero ID for the first one. An unborn
// HEAD with no reflog yet has no entries.
func gitHeadReflog(repoRoot string) ([]ReflogEntry, error) {
	cmd := exec.Command("git", "reflog", "show", "--format=%H%x09%gs", "HEAD", "--")
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

	var entries []ReflogEntry
	for line := range strings.Lines(string(out)) {
		newOID, msg, ok := strings.Cut(strings.TrimSuffix(line, "\n"), "\t")
		if !ok {
			return nil, fmt.Errorf("parse git reflog line %q", line)
		}
		entries = append(entries, ReflogEntry{New: newOID, Message: msg})
	}
	for i := range entries {
		if i+1 < len(entries) {
			entries[i].Old = entries[i+1].New
		} else {
			entries[i].Old = strings.Repeat("0", len(entries[i].New))
		}
	}

	return entries, nil
}

// gitHeadReflogSince is HeadReflogSince for a ref storage that only git reads,
// where offset is a number of entries.
func gitHeadReflogSince(repoRoot string, offset int64) ([]ReflogEntry, error) {
	all, err := gitHeadReflog(repoRoot)
	if err != nil {
		return nil, err
	}
	if int64(len(all)) < offset {
		return nil, ErrReflogRewritten
	}

	added := all[:int64(len(all))-offset]
	slices.Reverse(added)

	return added, nil
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
