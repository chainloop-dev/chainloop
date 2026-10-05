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

package state

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellPreSignatureRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		key  ShellCallKey
	}{
		{name: "main session", key: ShellCallKey{SessionID: "sess-123"}},
		{name: "subagent", key: ShellCallKey{SessionID: "sess-123", AgentID: "afd65659e2015d48d"}},
		{name: "tool call", key: ShellCallKey{SessionID: "sess-123", ToolUseID: "toolu_01ABC"}},
		{name: "subagent tool call", key: ShellCallKey{SessionID: "sess-123", AgentID: "afd65659e2015d48d", ToolUseID: "toolu_01ABC"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewGitStore(t.TempDir())
			sig := map[string]string{
				"a.go":       "hash-a",
				"sub/b.json": "hash-b",
			}

			require.NoError(t, store.SaveShellPreSignature(tc.key, sig))

			loaded, err := store.LoadShellPreSignature(tc.key)
			require.NoError(t, err)
			assert.Equal(t, sig, loaded)

			store.DeleteShellPreSignature(tc.key)

			_, err = store.LoadShellPreSignature(tc.key)
			assert.Error(t, err, "signature should be gone after delete")
		})
	}
}

// Shell commands can overlap: a subagent shares its parent's session ID, and
// one agent can run several commands at once. Each command needs its own slot,
// or the first post hook to finish deletes a signature another command still
// needs.
func TestShellPreSignatureSlots(t *testing.T) {
	const sessionID = "sess-123"

	keys := []ShellCallKey{
		{SessionID: sessionID},
		{SessionID: sessionID, AgentID: "agent-1"},
		{SessionID: sessionID, AgentID: "agent-2"},
		{SessionID: sessionID, ToolUseID: "toolu_1"},
		{SessionID: sessionID, ToolUseID: "toolu_2"},
		{SessionID: sessionID, AgentID: "agent-1", ToolUseID: "toolu_3"},
	}

	store := NewGitStore(t.TempDir())
	for i, key := range keys {
		require.NoError(t, store.SaveShellPreSignature(key, map[string]string{"a.go": fmt.Sprint(i)}))
	}

	deleted := keys[1]
	store.DeleteShellPreSignature(deleted)

	for i, key := range keys {
		got, err := store.LoadShellPreSignature(key)
		if key == deleted {
			assert.Error(t, err, "deleted slot %+v", key)
			continue
		}

		require.NoError(t, err, "slot %+v", key)
		assert.Equal(t, map[string]string{"a.go": fmt.Sprint(i)}, got, "slot %+v keeps its own signature", key)
	}
}
