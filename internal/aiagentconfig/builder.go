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

	"github.com/chainloop-dev/chainloop/internal/schemavalidators"
)

// Build reads discovered files and constructs the AI agent config payload.
// basePath is the base directory, filePaths are relative to basePath.
// agentName identifies the AI agent (e.g. "claude", "cursor").
// gitCtx may be nil if not in a git repository.
func Build(basePath string, filePaths []string, agentName string, gitCtx *GitContext) (*Evidence, error) {
	// Resolve basePath to its real path so symlink comparisons are reliable
	realRoot, err := filepath.EvalSymlinks(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolving root dir: %w", err)
	}

	configFiles := make([]ConfigFile, 0, len(filePaths))
	hashes := make([]string, 0, len(filePaths))

	for _, relPath := range filePaths {
		absPath := filepath.Join(basePath, relPath)

		// Resolve the full path through any symlinks (covers both symlinked
		// files and symlinked parent directories like .claude/) and verify
		// the resolved path stays within basePath.
		realPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", relPath, err)
		}
		if err := ensureInsideDir(realPath, realRoot); err != nil {
			return nil, fmt.Errorf("reading %s: %w", relPath, err)
		}

		content, err := os.ReadFile(realPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", relPath, err)
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
			SHA256:  hexHash,
			Size:    info.Size(),
			Content: base64.StdEncoding.EncodeToString(content),
		})
	}

	data := Evidence{
		SchemaVersion: string(schemavalidators.AIAgentConfigVersion0_1),
		Agent:         Agent{Name: agentName},
		ConfigHash:    computeCombinedHash(hashes),
		CapturedAt:    time.Now().UTC().Format(time.RFC3339),
		GitContext:    gitCtx,
		ConfigFiles:   configFiles,
	}

	return &data, nil
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
