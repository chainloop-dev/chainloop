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

package aiagentconfig

import (
	"path/filepath"
	"sort"
)

// claudePatterns are glob patterns for Claude agent configuration files,
// evaluated relative to a root directory.
var claudePatterns = []string{
	"CLAUDE.md",
	".claude/CLAUDE.md",
	".claude/settings.json",
	".mcp.json",
	".claude/rules/*.md",
	".claude/agents/*.md",
	".claude/commands/*.md",
	".claude/skills/*/SKILL.md",
}

// Discover searches basePath for AI agent configuration files.
// It only looks in basePath and its subdirectories, never in parent directories.
// Returns deduplicated relative paths sorted alphabetically.
func Discover(basePath string) ([]string, error) {
	seen := make(map[string]struct{})
	var results []string

	for _, pattern := range claudePatterns {
		absPattern := filepath.Join(basePath, pattern)
		matches, err := filepath.Glob(absPattern)
		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			rel, err := filepath.Rel(basePath, match)
			if err != nil {
				return nil, err
			}

			if _, ok := seen[rel]; !ok {
				seen[rel] = struct{}{}
				results = append(results, rel)
			}
		}
	}

	sort.Strings(results)

	return results, nil
}
