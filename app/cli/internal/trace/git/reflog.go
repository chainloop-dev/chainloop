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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// headReflogPath returns the HEAD reflog of the checkout at repoRoot. A linked
// worktree has its own HEAD, and its own reflog in its private git dir.
func headReflogPath(repoRoot string) (string, error) {
	gitDir, _, err := FindGitDirAndRootFrom(repoRoot)
	if err != nil {
		return "", err
	}

	return filepath.Join(gitDir, "logs", "HEAD"), nil
}

// HeadReflogSize returns the size in bytes of the HEAD reflog of the checkout
// at repoRoot, zero when it has none yet. Taken before a command, it marks
// where the entries that the command writes start. It only stats the file, so
// it is cheap enough for a hook that runs before every shell command.
func HeadReflogSize(repoRoot string) (int64, error) {
	path, err := headReflogPath(repoRoot)
	if err != nil {
		return 0, err
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

// HeadReflogSince returns the HEAD reflog entries written after offset, a size
// that HeadReflogSize returned earlier, oldest first. It returns
// ErrReflogRewritten when the reflog is now shorter than offset.
func HeadReflogSince(repoRoot string, offset int64) ([]ReflogEntry, error) {
	path, err := headReflogPath(repoRoot)
	if err != nil {
		return nil, err
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
