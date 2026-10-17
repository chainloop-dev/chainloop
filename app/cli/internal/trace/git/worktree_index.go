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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/filemode"
	"github.com/go-git/go-git/v6/plumbing/format/gitattributes"
	formatindex "github.com/go-git/go-git/v6/plumbing/format/index"
	"github.com/go-git/go-git/v6/utils/convert"
)

// UnstagedFiles returns regular-file paths whose working-tree content differs
// from the effective index. Paths are repo-relative and slash-separated.
func (c *GoGitClient) UnstagedFiles(repoRoot string) (map[string]struct{}, error) {
	repo, err := openRepo(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("open repository: %w", err)
	}
	idx, err := loadEffectiveIndex(repo)
	if err != nil {
		return nil, err
	}
	cfg, err := repo.Config()
	if err != nil {
		return nil, fmt.Errorf("load repository config: %w", err)
	}
	worktree, err := snapshotWorktree(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("snapshot worktree: %w", err)
	}

	indexed := make(map[string]*formatindex.Entry)
	for _, entry := range idx.Entries {
		if entry.Stage == 0 && !entry.IntentToAdd && entry.Mode.IsFile() && entry.Mode != filemode.Symlink {
			indexed[entry.Name] = entry
		}
	}
	attributes, err := attributeMatcher(repo, repoRoot, indexed)
	if err != nil {
		return nil, fmt.Errorf("load git attributes: %w", err)
	}

	paths := make(map[string]struct{})
	for path := range worktree {
		entry, ok := indexed[path]
		if !ok {
			paths[path] = struct{}{}
			continue
		}
		same, err := worktreeMatchesIndex(repoRoot, path, entry, repo, cfg.Core.AutoCRLF, cfg.Core.FileMode, attributes)
		if err != nil {
			return nil, err
		}
		if !same {
			paths[path] = struct{}{}
		}
	}
	for path := range indexed {
		if _, ok := worktree[path]; !ok {
			paths[path] = struct{}{}
		}
	}

	return paths, nil
}

func worktreeMatchesIndex(
	repoRoot, path string,
	entry *formatindex.Entry,
	repo *gogit.Repository,
	autoCRLF string,
	fileMode bool,
	attributes gitattributes.Matcher,
) (bool, error) {
	fullPath := filepath.Join(repoRoot, filepath.FromSlash(path))
	info, err := os.Stat(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("stat worktree file %s: %w", path, err)
	}
	if fileMode {
		mode, err := filemode.NewFromOSFileMode(info.Mode())
		if err != nil {
			return false, fmt.Errorf("read worktree mode %s: %w", path, err)
		}
		if mode != entry.Mode {
			return false, nil
		}
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return false, fmt.Errorf("read worktree file %s: %w", path, err)
	}

	blob, err := repo.BlobObject(entry.Hash)
	if err != nil {
		return false, fmt.Errorf("load index blob %s: %w", path, err)
	}
	reader, err := blob.Reader()
	if err != nil {
		return false, fmt.Errorf("read index blob %s: %w", path, err)
	}
	defer func() { _ = reader.Close() }()
	staged, err := io.ReadAll(reader)
	if err != nil {
		return false, fmt.Errorf("read index content %s: %w", path, err)
	}
	if bytes.Equal(content, staged) {
		return true, nil
	}

	content, canCompare := cleanWorktreeContent(path, content, autoCRLF, attributes)
	if !canCompare {
		return false, nil
	}

	return bytes.Equal(content, staged), nil
}

func cleanWorktreeContent(
	path string,
	content []byte,
	autoCRLF string,
	matcher gitattributes.Matcher,
) ([]byte, bool) {
	result, _ := matcher.Match(strings.Split(filepath.ToSlash(path), "/"), nil)
	if filter, ok := result["filter"]; ok && !filter.IsUnset() {
		// Raw equality is handled before this function. Reproducing an
		// arbitrary configured filter driver would execute repository code, so
		// transformed content stays conservatively dirty.
		return nil, false
	}

	normalize := autoCRLF == "true" || autoCRLF == "input"
	forceText := false
	if text, ok := result["text"]; ok {
		switch {
		case text.IsUnset():
			normalize = false
		case text.IsSet():
			normalize, forceText = true, true
		case text.IsValueSet() && strings.EqualFold(text.Value(), "auto"):
			normalize = true
		}
	}
	if eol, ok := result["eol"]; ok && !eol.IsUnset() {
		normalize, forceText = true, true
	}
	if !normalize {
		return content, true
	}

	stat, err := convert.GetStat(bytes.NewReader(content))
	if err != nil || stat.IsBinary() && !forceText {
		return content, true
	}
	var cleaned bytes.Buffer
	if _, err := io.Copy(convert.NewLFWriter(&cleaned), bytes.NewReader(content)); err != nil {
		return nil, false
	}

	return cleaned.Bytes(), true
}
