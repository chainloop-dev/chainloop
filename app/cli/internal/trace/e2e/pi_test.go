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

package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPiDisableSessionStart(t *testing.T) {
	t.Run("disables the event and fallback", func(t *testing.T) {
		repo := t.TempDir()
		path := filepath.Join(repo, piExtensionPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(
			piSessionStartHandler+"\n"+piSessionStartFallback+"\n",
		), 0o600))

		require.NoError(t, (&Pi{}).DisableSessionStart(repo))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotContains(t, string(data), piSessionStartHandler)
		assert.NotContains(t, string(data), piSessionStartFallback)
		assert.Contains(t, string(data), piDisabledSessionStartHandler)
		assert.Contains(t, string(data), piDisabledSessionStartFallback)
	})

	t.Run("rejects an unexpected extension", func(t *testing.T) {
		repo := t.TempDir()
		path := filepath.Join(repo, piExtensionPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("export default function () {}\n"), 0o600))

		err := (&Pi{}).DisableSessionStart(repo)

		require.ErrorContains(t, err, "session-start handlers")
	})
}

func TestPiHookMessages(t *testing.T) {
	sessionDir := t.TempDir()
	session := `{"type":"session","version":3,"id":"session-1","timestamp":"2026-10-23T00:00:00Z"}
{"type":"custom_message","id":"one","parentId":null,"timestamp":"2026-10-23T00:00:01Z","customType":"chainloop-trace-instruction:session-1","content":"full instruction\n\nCapture only in these cases: reminder","display":false}
{"type":"custom_message","id":"two","parentId":"one","timestamp":"2026-10-23T00:00:02Z","customType":"chainloop-trace-spec-reminder","content":"later reminder","display":false}
{"type":"custom_message","id":"three","parentId":"two","timestamp":"2026-10-23T00:00:03Z","customType":"other","content":"ignore","display":false}
`
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "session.jsonl"), []byte(session), 0o600))

	got, err := piHookMessages(sessionDir)

	require.NoError(t, err)
	assert.Equal(t, []HookMessage{
		{Event: piSessionStartEvent, ModelContext: "full instruction\n\nCapture only in these cases: reminder"},
		{Event: piPromptSubmitEvent, ModelContext: "full instruction\n\nCapture only in these cases: reminder"},
		{Event: piPromptSubmitEvent, ModelContext: "later reminder"},
	}, got)
}
