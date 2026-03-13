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
)

// BuildEvidence reads discovered files and constructs the evidence payload.
// rootDir is the base directory, filePaths are relative to rootDir.
// gitCtx may be nil if not in a git repository.
func BuildEvidence(rootDir string, filePaths []string, gitCtx *GitContext) (*Evidence, error) {
	configFiles := make([]ConfigFile, 0, len(filePaths))
	hashes := make([]string, 0, len(filePaths))

	for _, relPath := range filePaths {
		absPath := filepath.Join(rootDir, relPath)

		content, err := os.ReadFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", relPath, err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", relPath, err)
		}

		hash := sha256.Sum256(content)
		hexHash := hex.EncodeToString(hash[:])
		hashes = append(hashes, hexHash)

		configFiles = append(configFiles, ConfigFile{
			Path:          relPath,
			SHA256:        hexHash,
			Size:          info.Size(),
			Base64Content: base64.StdEncoding.EncodeToString(content),
		})
	}

	data := Data{
		SchemaVersion: "v1alpha",
		Agent:         Agent{Name: "claude"},
		ConfigHash:    computeCombinedHash(hashes),
		CapturedAt:    time.Now().UTC().Format(time.RFC3339),
		GitContext:    gitCtx,
		ConfigFiles:   configFiles,
	}

	return NewEvidence(data), nil
}

// computeCombinedHash sorts individual hashes, concatenates them, and hashes the result.
func computeCombinedHash(hashes []string) string {
	sorted := make([]string, len(hashes))
	copy(sorted, hashes)
	sort.Strings(sorted)

	combined := sha256.Sum256([]byte(strings.Join(sorted, "")))

	return hex.EncodeToString(combined[:])
}
