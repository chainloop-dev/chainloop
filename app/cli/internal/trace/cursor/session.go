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

package cursor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sanitizeRepoPath encodes a repo root the way Cursor does for its projects
// directory: drop the leading path separator, replace any run of
// non-alphanumerics with a single dash.
//
// Example: /Users/me/proj/myrepo -> Users-me-proj-myrepo
func sanitizeRepoPath(repoRoot string) string {
	s := strings.TrimPrefix(repoRoot, string(filepath.Separator))

	var b strings.Builder
	b.Grow(len(s))
	prevDash := false
	for _, r := range s {
		if isAlphanumeric(r) {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}

	return strings.Trim(b.String(), "-")
}

func isAlphanumeric(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// transcriptDirForRepo returns the Cursor agent-transcripts directory for a repo:
// ~/.cursor/projects/<sanitized-repo>/agent-transcripts/
func transcriptDirForRepo(repoRoot string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".cursor", "projects", sanitizeRepoPath(repoRoot), "agent-transcripts")
}

// resolveSessionJSONL returns the path to the transcript file for sessionID
// inside dir, handling both the CLI flat layout (<dir>/<id>.jsonl) and the
// IDE nested layout (<dir>/<id>/<id>.jsonl). Returns an fs.ErrNotExist-wrapping
// error if neither candidate exists, and surfaces any other Stat error
// (e.g. permission denied) so callers don't silently treat it as missing.
func resolveSessionJSONL(dir, sessionID string) (string, error) {
	flat := filepath.Join(dir, sessionID+".jsonl")
	switch _, err := os.Stat(flat); {
	case err == nil:
		return flat, nil
	case !errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("stat %q: %w", flat, err)
	}

	nested := filepath.Join(dir, sessionID, sessionID+".jsonl")
	switch _, err := os.Stat(nested); {
	case err == nil:
		return nested, nil
	case !errors.Is(err, os.ErrNotExist):
		return "", fmt.Errorf("stat %q: %w", nested, err)
	}

	return "", fmt.Errorf("cursor transcript for %q not found in %q: %w", sessionID, dir, os.ErrNotExist)
}
