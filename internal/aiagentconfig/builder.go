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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// Build reads discovered files and constructs the AI agent config payload.
// basePath is the base directory, discovered contains files relative to basePath with their kinds.
// agentName identifies the AI agent (e.g. "claude", "cursor").
// gitCtx may be nil if not in a git repository.
// capturedAt is the timestamp to record in the output; pass time.Time{} to use the current time.
func Build(basePath string, discovered []DiscoveredFile, agentName string, gitCtx *GitContext, capturedAt time.Time) (*Data, error) {
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}

	// Resolve basePath to its real path so symlink comparisons are reliable
	realRoot, err := filepath.EvalSymlinks(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolving root dir: %w", err)
	}

	configFiles := make([]ConfigFile, 0, len(discovered))
	hashes := make([]string, 0, len(discovered))
	// Collect raw content from settings.json files to avoid base64 round-trip during MCP extraction.
	var rawSettingsFiles []rawConfigContent

	for _, df := range discovered {
		relPath := df.Path
		absPath := filepath.Join(basePath, relPath)

		content, realPath, err := safeReadFile(absPath, realRoot)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", relPath, err)
		}

		info, err := os.Stat(realPath)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", relPath, err)
		}

		hash := sha256.Sum256(content)
		hexHash := hex.EncodeToString(hash[:])
		hashes = append(hashes, fmt.Sprintf("%s:%s", relPath, hexHash))

		configFiles = append(configFiles, ConfigFile{
			Path:    relPath,
			Kind:    df.Kind,
			SHA256:  hexHash,
			Size:    info.Size(),
			Content: base64.StdEncoding.EncodeToString(content),
		})

		// Keep raw bytes for settings.json files to avoid base64 round-trip
		if df.Kind == ConfigFileKindConfiguration && filepath.Base(relPath) == "settings.json" {
			rawSettingsFiles = append(rawSettingsFiles, rawConfigContent{path: relPath, content: content})
		}
	}

	mcpServers := extractMCPServers(realRoot, rawSettingsFiles)

	data := Data{
		Agent:       Agent{Name: agentName},
		ConfigHash:  computeCombinedHash(hashes),
		CapturedAt:  capturedAt.Format(time.RFC3339),
		GitContext:  gitCtx,
		ConfigFiles: configFiles,
		MCPServers:  mcpServers,
	}

	return &data, nil
}

// rawConfigContent holds a file's raw bytes alongside its relative path,
// used to pass already-read content to MCP extraction without re-decoding.
type rawConfigContent struct {
	path    string
	content []byte
}

// extractMCPServers collects MCP server definitions from two sources:
// 1. .mcp.json at the root (read directly, not collected in config_files)
// 2. settings.json files already collected (passed as raw bytes)
// Servers are deduplicated by name (first occurrence wins) and sorted.
func extractMCPServers(realRoot string, settingsFiles []rawConfigContent) []MCPServer {
	seen := make(map[string]struct{})
	var servers []MCPServer

	addServers := func(extracted []MCPServer) {
		for _, s := range extracted {
			if _, ok := seen[s.Name]; !ok {
				seen[s.Name] = struct{}{}
				servers = append(servers, s)
			}
		}
	}

	// Source 1: .mcp.json read directly from disk.
	// Resolve symlinks before reading to prevent reading files outside the root.
	if extracted, err := readMCPFile(realRoot); err == nil && len(extracted) > 0 {
		addServers(extracted)
	}

	// Source 2: settings.json files (raw bytes from the Build loop)
	for _, sf := range settingsFiles {
		extracted, err := ExtractMCPServers(sf.content)
		if err != nil {
			log.Debug().Err(err).Str("path", sf.path).Msg("failed to parse MCP servers from settings")
			continue
		}
		addServers(extracted)
	}

	if len(servers) == 0 {
		return nil
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	return servers
}

// readMCPFile reads and parses .mcp.json from the given root directory.
// It resolves symlinks and verifies the file stays inside the root before reading.
func readMCPFile(realRoot string) ([]MCPServer, error) {
	mcpPath := filepath.Join(realRoot, ".mcp.json")

	content, _, err := safeReadFile(mcpPath, realRoot)
	if err != nil {
		return nil, err
	}

	servers, err := ExtractMCPServers(content)
	if err != nil {
		log.Debug().Err(err).Str("path", ".mcp.json").Msg("failed to parse MCP servers")
		return nil, err
	}

	return servers, nil
}

// safeReadFile resolves symlinks, verifies the resolved path stays inside rootDir,
// and reads the file content. Returns the content and the resolved real path.
func safeReadFile(path, rootDir string) ([]byte, string, error) {
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, "", err
	}

	if err := ensureInsideDir(realPath, rootDir); err != nil {
		return nil, "", err
	}

	content, err := os.ReadFile(realPath)
	if err != nil {
		return nil, "", err
	}

	return content, realPath, nil
}

// computeCombinedHash sorts individual hashes, concatenates them, and hashes the result.
func computeCombinedHash(hashes []string) string {
	sorted := make([]string, len(hashes))
	copy(sorted, hashes)
	sort.Strings(sorted)

	combined := sha256.Sum256([]byte(strings.Join(sorted, "")))

	return hex.EncodeToString(combined[:])
}

// ensureInsideDir verifies that filePath is inside dir. Both paths must be
// already resolved (no symlinks). Returns an error if the file escapes.
func ensureInsideDir(filePath, dir string) error {
	rel, err := filepath.Rel(dir, filePath)
	if err != nil {
		return fmt.Errorf("path escapes root directory via symlink")
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes root directory via symlink")
	}

	return nil
}
