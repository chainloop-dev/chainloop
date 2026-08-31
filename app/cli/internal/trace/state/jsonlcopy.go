package state

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopySessionJSONL copies the Claude Code JSONL file to <dir>/chainloop-trace/raw/<id>.jsonl.
// It also copies subagent files if they exist.
func (s *Store) CopySessionJSONL(sessionID, sourceDir string) error {
	rawDir := filepath.Join(s.traceDirPath(), traceDirRaw)
	if err := os.MkdirAll(rawDir, 0755); err != nil {
		return err
	}

	// The source lives in the agent's own directory, named by the ID the agent
	// assigned; only our copy gets the sanitized name. Joining an unchecked ID
	// here would read outside sourceDir.
	if !ValidSessionID(sessionID) {
		return fmt.Errorf("session ID %q cannot name a transcript file", sessionID)
	}
	srcPath := filepath.Join(sourceDir, sessionID+".jsonl")

	if err := copyFile(srcPath, RawSessionPath(rawDir, sessionID)); err != nil {
		return fmt.Errorf("copy session JSONL: %w", err)
	}

	// Copy subagent directory if present — skip when the directory doesn't exist
	subagentsSrc := filepath.Join(sourceDir, sessionID, "subagents")
	subagentsDst := RawSubagentDir(rawDir, sessionID)
	if err := copyDir(subagentsSrc, subagentsDst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("copy subagent files: %w", err)
	}

	return nil
}

// RawSessionDir returns the path to <dir>/chainloop-trace/raw/ for use as session dir in parsing.
func (s *Store) RawSessionDir() string {
	return filepath.Join(s.traceDirPath(), traceDirRaw)
}

// CopyRawSessionFile copies a single session transcript file from srcPath
// to <dir>/chainloop-trace/raw/<sanitized-sessionID>.jsonl. Used by providers
// whose transcripts are single files (e.g. Cursor); Claude's multi-file
// layout (main + subagents) goes through CopySessionJSONL instead.
func (s *Store) CopyRawSessionFile(sessionID, srcPath string) error {
	rawDir := s.RawSessionDir()
	dst := filepath.Join(rawDir, SanitizeID(sessionID)+".jsonl")

	return copyFile(srcPath, dst)
}

// RawSessionPath returns the path a session's raw transcript is copied to
// inside rawDir. Readers and writers must both name the file through this
// helper: the sanitized ID it applies is not recoverable from the raw one.
func RawSessionPath(rawDir, sessionID string) string {
	return filepath.Join(rawDir, SanitizeID(sessionID)+".jsonl")
}

// RawSubagentDir returns the directory a session's copied subagent transcripts
// live in inside rawDir. Same naming contract as RawSessionPath.
func RawSubagentDir(rawDir, sessionID string) string {
	return filepath.Join(rawDir, SanitizeID(sessionID), "subagents")
}

// FindRawSessionFile returns the path to the raw session file for sessionID
// within rawDir, or an error wrapping os.ErrNotExist when missing.
func FindRawSessionFile(rawDir, sessionID string) (string, error) {
	path := RawSessionPath(rawDir, sessionID)
	if _, err := os.Stat(path); err != nil {
		return "", err
	}

	return path, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
