//go:build integration

package opencode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealExportViaFile verifies that CopySessionData captures the full
// export output via file redirect, avoiding the pipe truncation bug where
// `opencode export` (Node.js) doesn't flush stdout to pipes properly.
func TestRealExportViaFile(t *testing.T) {
	const sessionID = "ses_0cbfb6771ffeqmW0YTMB4Sp0xN"

	store := state.NewGitStore(t.TempDir())
	provider := New()
	err := provider.CopySessionData(store, "/repo", sessionID)
	require.NoError(t, err)

	rawDir := store.RawSessionDir()
	path := filepath.Join(rawDir, state.SanitizeID(sessionID)+".jsonl")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	// Must exceed the 64 KiB pipe truncation limit.
	assert.Greater(t, len(data), 65536, "file redirect should capture full output")
	t.Logf("Captured %d bytes via file redirect", len(data))

	var export exportData
	require.NoError(t, json.Unmarshal(data, &export))
	assert.Equal(t, sessionID, export.Info.ID)
	assert.NotEmpty(t, export.Messages)
	t.Logf("Session: %s, messages: %d", export.Info.Title, len(export.Messages))

	// Verify it parses all the way through the provider.
	result, err := provider.ParseSession(context.Background(), &trace.ParseOpts{
		SessionDir: rawDir,
		SessionID:  sessionID,
	})
	require.NoError(t, err)
	assert.Equal(t, "opencode", result.Data.Agent.Name)
	assert.Equal(t, sessionID, result.Data.Session.ID)
	assert.NotEmpty(t, result.Data.Conversation.TotalMessages)
	t.Logf("Parsed: %d messages, %d tool invocations",
		result.Data.Conversation.TotalMessages,
		result.Data.ToolsUsed.TotalInvocations)
}
