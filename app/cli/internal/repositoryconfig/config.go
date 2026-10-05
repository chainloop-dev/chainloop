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

// Package repositoryconfig reads the Chainloop configuration committed to a repository.
package repositoryconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tracegit "github.com/chainloop-dev/chainloop/app/cli/internal/trace/git"
	"gopkg.in/yaml.v3"
)

const (
	chainloopYMLFile  = ".chainloop.yml"
	chainloopYAMLFile = ".chainloop.yaml"
)

// ErrChainloopYMLNotFound indicates that neither supported config filename exists.
var ErrChainloopYMLNotFound = errors.New(".chainloop.yml not found")

// ChainloopYML represents the relevant fields in .chainloop.yml.
type ChainloopYML struct {
	ProjectName    string `yaml:"projectName"`
	ProjectVersion string `yaml:"projectVersion"`
	Organization   string `yaml:"organization,omitempty"`
	RequireTrace   *bool  `yaml:"requireTrace,omitempty"`
	WorkflowName   string `yaml:"workflowName,omitempty"`
}

// LoadChainloopYML loads the nearest .chainloop.yml (or .chainloop.yaml),
// walking from dir to the git repository root. In each directory, .yml takes
// precedence over .yaml. A config without projectName is still valid.
func LoadChainloopYML(dir string) (*ChainloopYML, string, error) {
	dir = resolveDir(dir)
	_, repoRoot, err := tracegit.FindGitDirAndRootFrom(dir)
	if err != nil {
		if !errors.Is(err, tracegit.ErrNotARepository) {
			return nil, "", err
		}
		repoRoot = dir
	}
	repoRoot = resolveDir(repoRoot)

	for {
		cfg, path, found, err := loadChainloopYMLFromDir(dir)
		if err != nil {
			return nil, path, err
		}
		if found {
			return cfg, path, nil
		}
		if dir == repoRoot {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil, "", ErrChainloopYMLNotFound
}

// FindChainloopYML returns the nearest repository config, or nil when it
// cannot be loaded. Call LoadChainloopYML when the path or error is needed.
func FindChainloopYML(dir string) *ChainloopYML {
	cfg, _, err := LoadChainloopYML(dir)
	if err != nil {
		return nil
	}
	return cfg
}

// LoadProjectFromYML returns projectName from the nearest repository config.
func LoadProjectFromYML(dir string) string {
	if cfg := FindChainloopYML(dir); cfg != nil {
		return cfg.ProjectName
	}
	return ""
}

func resolveDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	return resolved
}

func loadChainloopYMLFromDir(dir string) (*ChainloopYML, string, bool, error) {
	for _, name := range []string{chainloopYMLFile, chainloopYAMLFile} {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, path, true, fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}

		var cfg ChainloopYML
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, path, true, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
		}
		return &cfg, path, true, nil
	}
	return nil, "", false, nil
}
