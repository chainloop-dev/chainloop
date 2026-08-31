package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SessionRecord holds metadata about an AI coding agent session.
type SessionRecord struct {
	// SessionID is the agent-assigned identifier for this session.
	SessionID string `json:"session_id"`
	// Provider identifies the agent that produced this session (e.g. "claude-code", "cursor").
	// Empty on records written before multi-provider support; consumers should treat that as claude-code.
	Provider string `json:"provider,omitempty"`
	// AgentVersion is the agent runtime version reported at session-start
	// (e.g., Cursor's cursor_version). Empty for providers whose hook payload
	// does not carry the version. Recorded once on the first hook only.
	AgentVersion string `json:"agent_version,omitempty"`
	// Model is the model identifier reported at session-start (e.g.,
	// Cursor's "model" field). Empty for providers whose hook payload does
	// not carry the model. Recorded once on the first hook only.
	Model string `json:"model,omitempty"`
	// Active reports whether the session is ongoing at the time of record write.
	Active bool `json:"active"`
	// StartedAt is the RFC3339 timestamp of when tracking began for this session.
	StartedAt string `json:"started_at"`
}

// SaveSessionRecord writes a session record to <dir>/chainloop-trace/sessions/<id>.json.
func (s *Store) SaveSessionRecord(rec *SessionRecord) error {
	dir := filepath.Join(s.traceDirPath(), traceDirSessions)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, sanitizeID(rec.SessionID)+".json"), data, 0600)
}

// SessionRecordExists checks whether a session record file exists for the given session ID.
func (s *Store) SessionRecordExists(sessionID string) bool {
	path := filepath.Join(s.traceDirPath(), traceDirSessions, sanitizeID(sessionID)+".json")
	_, err := os.Stat(path)
	return err == nil
}

// LoadSessionRecord reads the SessionRecord for sessionID. Returns nil, nil when not found.
func (s *Store) LoadSessionRecord(sessionID string) (*SessionRecord, error) {
	path := filepath.Join(s.traceDirPath(), traceDirSessions, sanitizeID(sessionID)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	var rec SessionRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}

	return &rec, nil
}

// LoadAllSessionRecords reads every SessionRecord under
// <dir>/chainloop-trace/sessions/ into a map keyed by SessionID. A missing
// directory yields an empty (non-nil) map and no error; per-file decode
// errors are skipped so a single malformed record doesn't abort the load.
func (s *Store) LoadAllSessionRecords() (map[string]*SessionRecord, error) {
	dir := filepath.Join(s.traceDirPath(), traceDirSessions)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*SessionRecord{}, nil
		}

		return nil, err
	}

	out := make(map[string]*SessionRecord, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var rec SessionRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		out[rec.SessionID] = &rec
	}

	return out, nil
}
