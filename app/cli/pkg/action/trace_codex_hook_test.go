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

package action

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/codex"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexMultiFilePatchAttribution(t *testing.T) {
	root := initTempGitRepo(t)
	t.Chdir(root)
	t.Setenv("CODEX_HOME", t.TempDir())
	require.NoError(t, os.WriteFile(filepath.Join(root, ".chainloop.yml"), []byte("traceStorage: LOCAL\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "one.txt"), []byte("human\nold\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "gone.txt"), []byte("gone\n"), 0600))
	payload, _ := json.Marshal(map[string]any{"session_id": "codex-patch", "cwd": root, "tool_name": "apply_patch", "tool_use_id": "call-1", "tool_input": map[string]string{"command": "*** Begin Patch\n*** Update File: one.txt\n@@\n human\n-old\n+new\n*** Add File: two.txt\n+two\n*** Delete File: gone.txt\n*** End Patch"}})
	withStdin(t, string(payload))
	require.NoError(t, HandleAgentPreToolUse(codex.New(), zerolog.Nop()))
	require.NoError(t, os.WriteFile(filepath.Join(root, "one.txt"), []byte("human\nnew\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "two.txt"), []byte("two\n"), 0600))
	require.NoError(t, os.Remove(filepath.Join(root, "gone.txt")))
	withStdin(t, string(payload))
	require.NoError(t, HandleAgentPostToolUse(codex.New(), zerolog.Nop()))
	store := state.NewGitStore(filepath.Join(root, ".git"))
	ledger := store.LoadAILineAttribution("codex-patch")
	assert.Contains(t, ledger.Open, "one.txt")
	assert.Contains(t, ledger.Open, "two.txt")
	assert.Contains(t, ledger.Open, "gone.txt")
	assert.Len(t, ledger.Open["gone.txt"].Removed, 1)
	assert.Equal(t, 2, ledger.GeneratedAdded)
	assert.Equal(t, 2, ledger.GeneratedRemoved)
	// An unchanged human line must not become an AI addition.
	assert.Len(t, ledger.Open["one.txt"].Added, 1)
	session, err := store.LoadSessionRecord("codex-patch")
	require.NoError(t, err)
	assert.Equal(t, codex.Name, session.Provider)
	for _, name := range []string{"one.txt", "two.txt", "gone.txt"} {
		_, err := store.LoadFileSnapshot("codex-patch", filepath.Join(root, name))
		assert.ErrorIs(t, err, os.ErrNotExist)
	}
}
