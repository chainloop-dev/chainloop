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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"

	"github.com/go-git/go-git/v6/plumbing/filemode"
	"github.com/go-git/go-git/v6/plumbing/object"
)

// SnapshotIndex returns the content signature currently staged in git's index.
// It excludes symlinks and submodules to match SnapshotWorktree's file scope.
func (c *GoGitClient) SnapshotIndex(repoRoot string) (map[string]string, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	index, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	sig := make(map[string]string, len(index.Entries))
	for _, entry := range index.Entries {
		if !entry.Mode.IsFile() || entry.Mode == filemode.Symlink {
			continue
		}

		blob, err := repo.BlobObject(entry.Hash)
		if err != nil {
			return nil, fmt.Errorf("load staged blob %s: %w", entry.Name, err)
		}

		hash, err := hashBlob(blob)
		if err != nil {
			return nil, fmt.Errorf("hash staged blob %s: %w", entry.Name, err)
		}
		sig[entry.Name] = hash
	}

	return sig, nil
}

// IndexFile returns the exact staged content for path.
func (c *GoGitClient) IndexFile(repoRoot, path string) ([]byte, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}

	index, err := repo.Storer.Index()
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}

	for _, entry := range index.Entries {
		if entry.Name != path {
			continue
		}

		blob, err := repo.BlobObject(entry.Hash)
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

	return nil, fmt.Errorf("staged file %s not found", path)
}

// UnstagedFiles returns paths whose working-tree content differs from the
// index. Paths are repo-relative and slash-separated.
func (c *GoGitClient) UnstagedFiles(repoRoot string) (map[string]struct{}, error) {
	cmd := exec.Command("git", "-C", repoRoot, "diff",
		"--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", "--")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list unstaged paths: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}

	paths := make(map[string]struct{})
	for _, path := range bytes.Split(output, []byte{0}) {
		if len(path) > 0 {
			paths[string(path)] = struct{}{}
		}
	}

	return paths, nil
}

func hashBlob(blob *object.Blob) (string, error) {
	reader, err := blob.Reader()
	if err != nil {
		return "", err
	}
	defer func() { _ = reader.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
