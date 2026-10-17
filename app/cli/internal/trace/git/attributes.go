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
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-git/go-billy/v6/osfs"
	gogit "github.com/go-git/go-git/v6"
	gogitconfig "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing/format/gitattributes"
	formatindex "github.com/go-git/go-git/v6/plumbing/format/index"
	filesystemstorage "github.com/go-git/go-git/v6/storage/filesystem"
	"github.com/go-git/go-git/v6/x/plugin"
)

func attributeMatcher(
	repo *gogit.Repository,
	repoRoot string,
	indexed map[string]*formatindex.Entry,
) (gitattributes.Matcher, error) {
	system, err := gitattributes.LoadSystemPatterns(osfs.Default)
	if err != nil {
		return nil, err
	}
	global, err := gitattributes.LoadGlobalPatterns(osfs.Default)
	if err != nil {
		return nil, err
	}
	configSource, err := plugin.Get(plugin.ConfigLoader())
	if err != nil {
		return nil, err
	}
	globalStore, err := configSource.Load(gogitconfig.GlobalScope)
	if err != nil {
		return nil, err
	}
	globalConfig, err := globalStore.Config()
	if err != nil {
		return nil, err
	}
	if path := globalConfig.Raw.Section("core").Options.Get("attributesfile"); path != "" {
		global, err = configuredAttributes(path, repoRoot)
		if err != nil {
			return nil, err
		}
	}
	cfg, err := repo.Config()
	if err != nil {
		return nil, err
	}
	if path := cfg.Raw.Section("core").Options.Get("attributesfile"); path != "" {
		global, err = configuredAttributes(path, repoRoot)
		if err != nil {
			return nil, err
		}
	}
	attrs := make([]gitattributes.MatchAttribute, 0, len(system)+len(global))
	attrs = append(attrs, system...)
	attrs = append(attrs, global...)

	worktreeFS := osfs.New(repoRoot)
	root, err := gitattributes.ReadAttributesFile(worktreeFS, nil, ".gitattributes", true)
	if err != nil {
		return nil, err
	}
	attrs = append(attrs, root...)

	dirs := make(map[string]struct{})
	for path := range indexed {
		for dir := filepath.ToSlash(filepath.Dir(path)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			dirs[dir] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(dirs))
	for dir := range dirs {
		ordered = append(ordered, dir)
	}
	sort.Slice(ordered, func(i, j int) bool {
		leftDepth, rightDepth := strings.Count(ordered[i], "/"), strings.Count(ordered[j], "/")
		if leftDepth == rightDepth {
			return ordered[i] < ordered[j]
		}
		return leftDepth < rightDepth
	})
	for _, dir := range ordered {
		nested, err := gitattributes.ReadAttributesFile(worktreeFS, strings.Split(dir, "/"), ".gitattributes", false)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, nested...)
	}

	storage, ok := repo.Storer.(*filesystemstorage.Storage)
	if !ok {
		return nil, fmt.Errorf("load info attributes: unsupported repository storage %T", repo.Storer)
	}
	info, err := gitattributes.ReadAttributesFile(storage.Filesystem(), nil, "info/attributes", true)
	if err != nil {
		return nil, err
	}
	attrs = append(attrs, info...)

	// go-git beta.1 walks the matcher stack backwards while overwriting prior
	// results, so reverse Git's low-to-high source order before constructing it.
	for i, j := 0, len(attrs)-1; i < j; i, j = i+1, j-1 {
		attrs[i], attrs[j] = attrs[j], attrs[i]
	}

	return gitattributes.NewMatcher(attrs), nil
}

func configuredAttributes(path, repoRoot string) ([]gitattributes.MatchAttribute, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	} else if !filepath.IsAbs(path) {
		path = filepath.Join(repoRoot, path)
	}

	return gitattributes.ReadAttributesFile(osfs.Default, nil, filepath.Clean(path), true)
}
