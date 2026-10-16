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
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type indexEntry struct {
	mode   string
	object string
	path   string
	stage  int
}

// SnapshotIndex returns the content signature currently staged in git's index.
// It excludes symlinks and submodules to match SnapshotWorktree's file scope.
func (c *GoGitClient) SnapshotIndex(repoRoot string) (map[string]string, error) {
	entries, err := loadIndexEntries(repoRoot)
	if err != nil {
		return nil, err
	}

	parts := make(map[string][]string, len(entries))
	for _, entry := range entries {
		if !entry.isRegular() {
			continue
		}
		parts[entry.path] = append(parts[entry.path], fmt.Sprintf("%d:%s", entry.stage, entry.object))
	}

	sig := make(map[string]string, len(parts))
	for path, stages := range parts {
		sort.Strings(stages)
		sig[path] = strings.Join(stages, ",")
	}

	return sig, nil
}

// IndexFile returns the exact staged content for path.
func (c *GoGitClient) IndexFile(repoRoot, path string) ([]byte, error) {
	entries, err := loadIndexEntries(repoRoot)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.path != path || entry.stage != 0 || !entry.isRegular() {
			continue
		}

		content, err := commandOutput(exec.Command("git", "--no-replace-objects", "-C", repoRoot, "cat-file", "blob", entry.object))
		if err != nil {
			return nil, fmt.Errorf("read staged file %s: %w", path, err)
		}

		return content, nil
	}

	return nil, fmt.Errorf("staged file %s not found", path)
}

// UnstagedFiles returns paths whose working-tree content differs from the
// index. Paths are repo-relative and slash-separated.
func (c *GoGitClient) UnstagedFiles(repoRoot string) (map[string]struct{}, error) {
	output, err := commandOutput(exec.Command("git", "-C", repoRoot, "diff",
		"--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", "--"))
	if err != nil {
		return nil, fmt.Errorf("list unstaged paths: %w", err)
	}

	paths := make(map[string]struct{})
	for _, path := range nullDelimitedPaths(output) {
		paths[path] = struct{}{}
	}

	return paths, nil
}

func loadIndexEntries(repoRoot string) ([]indexEntry, error) {
	output, err := commandOutput(exec.Command("git", "-C", repoRoot, "ls-files", "--stage", "-z", "--"))
	if err != nil {
		return nil, fmt.Errorf("list index entries: %w", err)
	}

	var entries []indexEntry
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		metadata, path, ok := bytes.Cut(record, []byte{'\t'})
		fields := bytes.Fields(metadata)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("parse index entry %q", record)
		}
		stage, err := strconv.Atoi(string(fields[2]))
		if err != nil {
			return nil, fmt.Errorf("parse index stage %q: %w", fields[2], err)
		}
		entries = append(entries, indexEntry{
			mode:   string(fields[0]),
			object: string(fields[1]),
			path:   string(path),
			stage:  stage,
		})
	}

	return entries, nil
}

func (e indexEntry) isRegular() bool {
	return strings.HasPrefix(e.mode, "100")
}

func nullDelimitedPaths(output []byte) []string {
	var paths []string
	for _, path := range bytes.Split(output, []byte{0}) {
		if len(path) > 0 {
			paths = append(paths, string(path))
		}
	}

	return paths
}

func commandOutput(cmd *exec.Cmd) ([]byte, error) {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git command: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}

	return output, nil
}
