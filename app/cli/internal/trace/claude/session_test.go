package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeCWDForClaudePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/Users/me/projects/myrepo", "-Users-me-projects-myrepo"},
		{"/tmp/test", "-tmp-test"},
		{"/a/b/c/d", "-a-b-c-d"},
		{"/home/user/project/.claude/worktrees/name", "-home-user-project--claude-worktrees-name"},
		{"/home/user/.config/test", "-home-user--config-test"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, encodeCWDForClaudePath(tt.input))
	}
}

func TestDiscoverClaudeSession(t *testing.T) {
	t.Run("no sessions directory", func(t *testing.T) {
		session, err := discoverClaudeSession("/nonexistent/repo")
		assert.NoError(t, err)
		assert.Nil(t, session)
	})

	t.Run("discovers matching session", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		repoRoot := "/tmp/test-repo"
		sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
		require.NoError(t, os.MkdirAll(sessionsDir, 0755))

		sf := claudeSessionFile{
			PID:       os.Getpid(),
			SessionID: "test-session-id",
			CWD:       repoRoot,
			StartedAt: 1774306212237,
		}
		data, err := json.Marshal(sf)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "12345.json"), data, 0600))

		projectsDir := filepath.Join(homeDir, ".claude", "projects", encodeCWDForClaudePath(repoRoot))
		require.NoError(t, os.MkdirAll(projectsDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(projectsDir, "test-session-id.jsonl"), []byte("{}"), 0600))

		session, err := discoverClaudeSession(repoRoot)
		require.NoError(t, err)
		require.NotNil(t, session)

		assert.Equal(t, "test-session-id", session.sessionID)
		assert.Equal(t, repoRoot, session.cwd)
		assert.True(t, session.isActive)
	})

	t.Run("ignores sessions for other directories", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
		require.NoError(t, os.MkdirAll(sessionsDir, 0755))

		sf := claudeSessionFile{
			PID:       os.Getpid(),
			SessionID: "other-session",
			CWD:       "/other/repo",
			StartedAt: 1774306212237,
		}
		data, err := json.Marshal(sf)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "99999.json"), data, 0600))

		session, err := discoverClaudeSession("/my/repo")
		assert.NoError(t, err)
		assert.Nil(t, session)
	})

	t.Run("prefers active over inactive sessions", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)

		repoRoot := "/tmp/test-repo"
		sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
		require.NoError(t, os.MkdirAll(sessionsDir, 0755))

		inactive := claudeSessionFile{
			PID:       999999999,
			SessionID: "inactive-session",
			CWD:       repoRoot,
			StartedAt: 2000000000000,
		}
		data, err := json.Marshal(inactive)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "999999999.json"), data, 0600))

		active := claudeSessionFile{
			PID:       os.Getpid(),
			SessionID: "active-session",
			CWD:       repoRoot,
			StartedAt: 1000000000000,
		}
		data, err = json.Marshal(active)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "current.json"), data, 0600))

		session, err := discoverClaudeSession(repoRoot)
		require.NoError(t, err)
		require.NotNil(t, session)
		assert.Equal(t, "active-session", session.sessionID)
		assert.True(t, session.isActive)
	})
}

func TestIsPIDAlive(t *testing.T) {
	assert.True(t, isPIDAlive(os.Getpid()))
	assert.False(t, isPIDAlive(999999999))
	assert.False(t, isPIDAlive(0))
	assert.False(t, isPIDAlive(-1))
}

func TestDiscoverSessionViaProvider(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	repoRoot := "/tmp/provider-test"
	sessionsDir := filepath.Join(homeDir, ".claude", "sessions")
	require.NoError(t, os.MkdirAll(sessionsDir, 0755))

	sf := claudeSessionFile{
		PID:       os.Getpid(),
		SessionID: "provider-session",
		CWD:       repoRoot,
		StartedAt: 1774306212237,
	}
	data, err := json.Marshal(sf)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "pid.json"), data, 0600))

	projectsDir := filepath.Join(homeDir, ".claude", "projects", encodeCWDForClaudePath(repoRoot))
	require.NoError(t, os.MkdirAll(projectsDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(projectsDir, "provider-session.jsonl"), []byte("{}"), 0600))

	provider := New()
	assert.Equal(t, "claude-code", provider.Name())

	discovered, err := provider.DiscoverSession(repoRoot)
	require.NoError(t, err)
	require.NotNil(t, discovered)

	assert.Equal(t, "provider-session", discovered.SessionID)
	assert.True(t, discovered.IsActive)
	assert.NotEmpty(t, discovered.SessionDir)
}
