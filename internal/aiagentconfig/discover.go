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

// patternDef pairs a glob pattern with the kind of file it matches.
type patternDef struct {
	pattern string
	kind    ConfigFileKind
}

// agentDef defines an AI agent and its exclusive file patterns.
type agentDef struct {
	name     string
	patterns []patternDef
}

// agents is the registry of supported AI agents and their exclusive file patterns.
var agents = []agentDef{
	{name: "claude", patterns: []patternDef{
		{"CLAUDE.md", ConfigFileKindInstruction},
		{".claude/CLAUDE.md", ConfigFileKindInstruction},
		{".claude/settings.json", ConfigFileKindConfiguration},
		{".claude/rules/*.md", ConfigFileKindInstruction},
		{".claude/agents/*.md", ConfigFileKindInstruction},
		{".claude/commands/*.md", ConfigFileKindInstruction},
		{".claude/skills/*/SKILL.md", ConfigFileKindInstruction},
	}},
	{name: "cursor", patterns: []patternDef{
		{".cursor/rules/*.md", ConfigFileKindInstruction},
		{".cursor/rules/*.mdc", ConfigFileKindInstruction},
		{".cursor/rules/*/*.md", ConfigFileKindInstruction},
		{".cursor/rules/*/*.mdc", ConfigFileKindInstruction},
		{".cursor/skills/*/SKILL.md", ConfigFileKindInstruction},
		{".cursor/agents/*.md", ConfigFileKindInstruction},
	}},
}

// sharedPatterns are file patterns not exclusive to any agent.
// They are included in every agent's evidence when that agent has exclusive files.
var sharedPatterns = []patternDef{
	{".mcp.json", ConfigFileKindConfiguration},
	{"AGENTS.md", ConfigFileKindInstruction},
}

// DiscoverAll searches basePath for AI agent configuration files and groups them by agent.
// Only agents with at least one exclusive file match are included.
// Shared files are appended to each detected agent's file list.
// Returns a map of agent name → sorted, deduplicated discovered files.
func DiscoverAll(basePath string) (map[string][]DiscoveredFile, error) {
	sharedFiles, err := matchPatterns(basePath, sharedPatterns)
	if err != nil {
		return nil, err
	}

	result := make(map[string][]DiscoveredFile)

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
		merged := make([]DiscoveredFile, 0, len(files)+len(sharedFiles))
		for _, f := range files {
			seen[f.Path] = struct{}{}
			merged = append(merged, f)
		}
		for _, f := range sharedFiles {
			if _, ok := seen[f.Path]; !ok {
				merged = append(merged, f)
			}
		}

		sort.Slice(merged, func(i, j int) bool { return merged[i].Path < merged[j].Path })
		result[agent.name] = merged
	}

	return result, nil
}

// matchPatterns expands glob patterns relative to basePath and returns
// deduplicated, sorted discovered files.
func matchPatterns(basePath string, patterns []patternDef) ([]DiscoveredFile, error) {
	seen := make(map[string]struct{})
	var results []DiscoveredFile

	for _, pd := range patterns {
		absPattern := filepath.Join(basePath, pd.pattern)
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
				results = append(results, DiscoveredFile{Path: rel, Kind: pd.kind})
			}
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Path < results[j].Path })

	return results, nil
}
