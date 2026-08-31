package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeMockOpenCode creates a fake `opencode` binary in a temp dir that
// prints the given JSON to stdout. Returns the temp dir (to use as PATH).
// Uses printf (a POSIX shell builtin) so it works even when PATH is
// restricted to only the temp dir.
func writeMockOpenCode(t *testing.T, output string) string {
	t.Helper()
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "opencode")
	// printf is a shell builtin — no PATH dependency.
	script := "#!/bin/sh\nprintf '%s' " + quoteForSh(output) + "\n"
	//nolint:gosec // the mock is executed through PATH, so it must be executable
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0755))
	return tmpDir
}

// quoteForSh wraps a string in single quotes for safe shell use.
// Test data uses double quotes for JSON, so no embedded single-quote
// escaping is needed.
func quoteForSh(s string) string {
	return "'" + s + "'"
}

func TestDiscoverOpenCodeSessionNoBinary(t *testing.T) {
	// Force PATH to empty so opencode is not found.
	t.Setenv("PATH", "")
	session, err := discoverOpenCodeSession("/repo")
	assert.NoError(t, err)
	assert.Nil(t, session)
}

func TestDiscoverOpenCodeSessionFromMockBinary(t *testing.T) {
	output := `[{"id":"sess_old","title":"Old","directory":"/other/repo","updated":1000,"created":900},{"id":"sess_recent","title":"Recent","directory":"/my/repo","updated":2000,"created":1900},{"id":"sess_older","title":"Older","directory":"/my/repo","updated":1500,"created":1400}]`
	t.Setenv("PATH", writeMockOpenCode(t, output))

	session, err := discoverOpenCodeSession("/my/repo")
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, "sess_recent", session.ID)
	assert.Equal(t, "/my/repo", session.Directory)
}

func TestDiscoverOpenCodeSessionNoMatch(t *testing.T) {
	output := `[{"id":"sess1","title":"S1","directory":"/other/repo","updated":1000,"created":900}]`
	t.Setenv("PATH", writeMockOpenCode(t, output))

	session, err := discoverOpenCodeSession("/my/repo")
	assert.NoError(t, err)
	assert.Nil(t, session)
}

func TestDiscoverOpenCodeSessionBinaryFails(t *testing.T) {
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "opencode")
	// printf to stderr is not a builtin, but exit 1 is enough to fail.
	script := "#!/bin/sh\nexit 1\n"
	//nolint:gosec // the mock is executed through PATH, so it must be executable
	require.NoError(t, os.WriteFile(binPath, []byte(script), 0755))
	t.Setenv("PATH", tmpDir)

	session, err := discoverOpenCodeSession("/my/repo")
	assert.NoError(t, err)
	assert.Nil(t, session)
}

func TestDiscoverOpenCodeSessionInvalidJSON(t *testing.T) {
	t.Setenv("PATH", writeMockOpenCode(t, "not json"))

	session, err := discoverOpenCodeSession("/my/repo")
	assert.NoError(t, err)
	assert.Nil(t, session)
}

func TestDiscoverSessionViaProvider(t *testing.T) {
	output := `[{"id":"provider-session","title":"Test","directory":"/tmp/provider-test","updated":2000,"created":1000}]`
	t.Setenv("PATH", writeMockOpenCode(t, output))

	provider := New()
	assert.Equal(t, "opencode", provider.Name())

	discovered, err := provider.DiscoverSession("/tmp/provider-test")
	require.NoError(t, err)
	require.NotNil(t, discovered)
	assert.Equal(t, "provider-session", discovered.SessionID)
	assert.True(t, discovered.IsActive)
}

func TestSessionListEntryParsing(t *testing.T) {
	raw := `[{"id":"s1","title":"T1","directory":"/r","updated":1.5,"created":1.0}]`
	var entries []sessionListEntry
	require.NoError(t, json.Unmarshal([]byte(raw), &entries))
	require.Len(t, entries, 1)
	assert.Equal(t, "s1", entries[0].ID)
	assert.Equal(t, "/r", entries[0].Directory)
}
