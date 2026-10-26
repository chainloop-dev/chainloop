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
	"io"
	"os/exec"
	"strings"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/filemode"
)

// SnapshotIndex reads the effective index with Git, including split and sparse
// indexes and temporary indexes selected by GIT_INDEX_FILE.
func (c *ExecClient) SnapshotIndex(repoRoot string) (map[string]string, error) {
	cmd := exec.Command("git", "ls-files", "--stage", "-z", "--")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list staged files: %w", err)
	}

	sig := make(map[string]string)
	for _, record := range bytes.Split(out, []byte{0}) {
		header, path, ok := bytes.Cut(record, []byte{'\t'})
		if !ok {
			continue
		}
		fields := strings.Fields(string(header))
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid staged index entry %q", header)
		}
		if fields[2] == "0" && (fields[0] == "100644" || fields[0] == "100755") {
			sig[string(path)] = fields[1]
		}
	}

	return sig, nil
}

// IndexFile reads the staged blob selected by Git's effective index.
func (c *ExecClient) IndexFile(repoRoot, path string) ([]byte, error) {
	index, err := c.SnapshotIndex(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot staged index: %w", err)
	}
	hash, ok := index[path]
	if !ok {
		return nil, fmt.Errorf("staged file %s not found", path)
	}
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	return readIndexBlob(repo, plumbing.NewHash(hash), path)
}

// SnapshotIndex returns the content signature currently staged in git's index.
// It excludes symlinks and submodules to match SnapshotWorktree's file scope.
func (c *GoGitClient) SnapshotIndex(repoRoot string) (map[string]string, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	sig := make(map[string]string, len(idx.Entries))
	for _, entry := range idx.Entries {
		if entry.Stage != 0 || !entry.Mode.IsFile() || entry.Mode == filemode.Symlink {
			continue
		}
		sig[entry.Name] = entry.Hash.String()
	}

	return sig, nil
}

// IndexFile returns the exact staged content for path.
func (c *GoGitClient) IndexFile(repoRoot, path string) ([]byte, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	idx, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	for _, entry := range idx.Entries {
		if entry.Name != path || entry.Stage != 0 || !entry.Mode.IsFile() || entry.Mode == filemode.Symlink {
			continue
		}

		return readIndexBlob(repo, entry.Hash, path)
	}

	return nil, fmt.Errorf("staged file %s not found", path)
}

func readIndexBlob(repo *gogit.Repository, hash plumbing.Hash, path string) ([]byte, error) {
	blob, err := repo.BlobObject(hash)
	if err != nil {
		return nil, fmt.Errorf("load staged blob %s: %w", path, err)
	}
	reader, err := blob.Reader()
	if err != nil {
		return nil, fmt.Errorf("read staged blob %s: %w", path, err)
	}
	defer func() { _ = reader.Close() }()

	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read staged content %s: %w", path, err)
	}

	return content, nil
}
