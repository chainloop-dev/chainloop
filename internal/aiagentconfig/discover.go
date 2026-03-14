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

// agentDef defines an AI agent and its exclusive file patterns.
type agentDef struct {
	name     string
	patterns []string
}

// agents is the registry of supported AI agents and their exclusive file patterns.
var agents = []agentDef{
	{name: "claude", patterns: []string{
		"CLAUDE.md",
		".claude/CLAUDE.md",
		".claude/settings.json",
		".claude/rules/*.md",
		".claude/agents/*.md",
		".claude/commands/*.md",
		".claude/skills/*/SKILL.md",
	}},
	{name: "cursor", patterns: []string{
		".cursor/rules/*.md",
		".cursor/rules/*.mdc",
		".cursor/rules/*/*.md",
		".cursor/rules/*/*.mdc",
		".cursor/skills/*/SKILL.md",
		".cursor/agents/*.md",
	}},
}

// sharedPatterns are file patterns not exclusive to any agent.
// They are included in every agent's evidence when that agent has exclusive files.
var sharedPatterns = []string{
	".mcp.json",
	"AGENTS.md",
}

// DiscoverAll searches basePath for AI agent configuration files and groups them by agent.
// Only agents with at least one exclusive file match are included.
// Shared files are appended to each detected agent's file list.
// Returns a map of agent name → sorted, deduplicated relative paths.
func DiscoverAll(basePath string) (map[string][]string, error) {
	sharedFiles, err := matchPatterns(basePath, sharedPatterns)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string)

	for _, agent := range agents {
		files, err := matchPatterns(basePath, agent.patterns)
		if err != nil {
			return nil, err
		}

		if len(files) == 0 {
			continue
		}

		// Merge shared files into this agent's list, deduplicating
		seen := make(map[string]struct{}, len(files)+len(sharedFiles))
		merged := make([]string, 0, len(files)+len(sharedFiles))
		for _, f := range files {
			seen[f] = struct{}{}
			merged = append(merged, f)
		}
		for _, f := range sharedFiles {
			if _, ok := seen[f]; !ok {
				merged = append(merged, f)
			}
		}

		sort.Strings(merged)
		result[agent.name] = merged
	}

	return result, nil
}

// matchPatterns expands glob patterns relative to basePath and returns
// deduplicated, sorted relative paths.
func matchPatterns(basePath string, patterns []string) ([]string, error) {
	seen := make(map[string]struct{})
	var results []string

	for _, pattern := range patterns {
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
