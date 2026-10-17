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
	"os"
	"path/filepath"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/filemode"
	formatcfg "github.com/go-git/go-git/v6/plumbing/format/config"
	formatindex "github.com/go-git/go-git/v6/plumbing/format/index"
	gogithash "github.com/go-git/go-git/v6/plumbing/hash"
	"github.com/go-git/go-git/v6/plumbing/object"
)

// StagedFiles returns repo-relative paths changed between HEAD and the index
// used by the current git process. Git exposes temporary indexes (for example,
// during git commit --only) through GIT_INDEX_FILE.
func (c *GoGitClient) StagedFiles(repoRoot string) ([]string, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	idx, err := loadEffectiveIndex(repo)
	if err != nil {
		return nil, err
	}

	head, err := headEntries(repo)
	if err != nil {
		return nil, err
	}

	staged := make(map[string]struct{})
	current := make(map[string]*formatindex.Entry)
	for _, entry := range idx.Entries {
		if entry.IntentToAdd {
			continue
		}
		if entry.Stage != 0 {
			staged[entry.Name] = struct{}{}
			continue
		}
		current[entry.Name] = entry
	}

	for path, entry := range current {
		before, ok := head[path]
		if !ok || before.hash != entry.Hash || before.mode != entry.Mode {
			staged[path] = struct{}{}
		}
	}
	for path := range head {
		if _, ok := current[path]; !ok {
			staged[path] = struct{}{}
		}
	}

	files := make([]string, 0, len(staged))
	for path := range staged {
		files = append(files, path)
	}
	sort.Strings(files)

	return files, nil
}

// SnapshotIndex returns the content signature currently staged in git's index.
// It excludes symlinks and submodules to match SnapshotWorktree's file scope.
func (c *GoGitClient) SnapshotIndex(repoRoot string) (map[string]string, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	idx, err := loadEffectiveIndex(repo)
	if err != nil {
		return nil, err
	}

	parts := make(map[string][]string, len(idx.Entries))
	for _, entry := range idx.Entries {
		if !entry.Mode.IsFile() || entry.Mode == filemode.Symlink {
			continue
		}
		parts[entry.Name] = append(parts[entry.Name], fmt.Sprintf("%d:%s", entry.Stage, entry.Hash))
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
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	idx, err := loadEffectiveIndex(repo)
	if err != nil {
		return nil, err
	}

	for _, entry := range idx.Entries {
		if entry.Name != path || entry.Stage != 0 || !entry.Mode.IsFile() || entry.Mode == filemode.Symlink {
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

type treeEntry struct {
	hash plumbing.Hash
	mode filemode.FileMode
}

func headEntries(repo *gogit.Repository) (map[string]treeEntry, error) {
	entries := make(map[string]treeEntry)
	ref, err := repo.Head()
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return entries, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load HEAD: %w", err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("load HEAD commit: %w", err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("load HEAD tree: %w", err)
	}

	walker := object.NewTreeWalker(tree, true, nil)
	defer walker.Close()
	for {
		name, entry, err := walker.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("walk HEAD tree: %w", err)
		}
		if entry.Mode == filemode.Dir {
			continue
		}
		entries[name] = treeEntry{hash: entry.Hash, mode: entry.Mode}
	}

	return entries, nil
}

func loadEffectiveIndex(repo *gogit.Repository) (*formatindex.Index, error) {
	path := os.Getenv("GIT_INDEX_FILE")
	if path == "" {
		idx, err := repo.Storer.Index()
		if err != nil {
			return nil, fmt.Errorf("load index: %w", err)
		}

		return idx, nil
	}

	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open effective index directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	f, err := root.Open(filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("open effective index: %w", err)
	}
	defer func() { _ = f.Close() }()

	cfg, err := repo.Config()
	if err != nil {
		return nil, fmt.Errorf("load repository config: %w", err)
	}
	objectFormat := cfg.Extensions.ObjectFormat
	if objectFormat == formatcfg.UnsetObjectFormat {
		objectFormat = formatcfg.DefaultObjectFormat
	}
	hasher, err := gogithash.FromObjectFormat(objectFormat)
	if err != nil {
		return nil, fmt.Errorf("select index hash: %w", err)
	}

	idx := &formatindex.Index{}
	if err := formatindex.NewDecoder(f, hasher).Decode(idx); err != nil {
		return nil, fmt.Errorf("decode effective index: %w", err)
	}
	if stat, err := f.Stat(); err == nil {
		idx.ModTime = stat.ModTime()
	}

	return idx, nil
}
