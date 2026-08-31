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

// discoverCursorSession walks the agent-transcripts directory for repoRoot
// and returns the most recently modified transcript file. Returns nil when
// no transcripts exist.
func discoverCursorSession(repoRoot string) (sessionID, jsonlPath string, modTime int64, err error) {
	dir := transcriptDirForRepo(repoRoot)
	if dir == "" {
		return "", "", 0, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", 0, nil
		}

		return "", "", 0, err
	}

	var bestID, bestPath string
	var bestTime int64

	for _, entry := range entries {
		name := entry.Name()
		var candidateID, candidatePath string
		var candidateTime int64

		switch {
		case entry.IsDir():
			nested := filepath.Join(dir, name, name+".jsonl")
			info, err := os.Stat(nested)
			if err != nil {
				continue
			}
			candidateID = name
			candidatePath = nested
			candidateTime = info.ModTime().UnixNano()
		case strings.HasSuffix(name, ".jsonl"):
			info, err := entry.Info()
			if err != nil {
				continue
			}
			candidateID = strings.TrimSuffix(name, ".jsonl")
			candidatePath = filepath.Join(dir, name)
			candidateTime = info.ModTime().UnixNano()
		default:
			continue
		}

		if candidateTime > bestTime {
			bestID = candidateID
			bestPath = candidatePath
			bestTime = candidateTime
		}
	}

	return bestID, bestPath, bestTime, nil
}
