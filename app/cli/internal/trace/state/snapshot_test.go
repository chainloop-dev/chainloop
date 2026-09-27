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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShellPreSignatureRoundTrip(t *testing.T) {
	cases := []struct {
		name    string
		agentID string
	}{
		{name: "main session", agentID: ""},
		{name: "subagent", agentID: "afd65659e2015d48d"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := NewGitStore(t.TempDir())
			sessionID := "sess-123"
			sig := map[string]string{
				"a.go":       "hash-a",
				"sub/b.json": "hash-b",
			}

			require.NoError(t, store.SaveShellPreSignature(sessionID, tc.agentID, sig))

			loaded, err := store.LoadShellPreSignature(sessionID, tc.agentID)
			require.NoError(t, err)
			assert.Equal(t, sig, loaded)

			store.DeleteShellPreSignature(sessionID, tc.agentID)

			_, err = store.LoadShellPreSignature(sessionID, tc.agentID)
			assert.Error(t, err, "signature should be gone after delete")
		})
	}
}

// A subagent shares its parent's session ID. Their shell commands can overlap,
// so each agent needs its own slot or one deletes the other's signature.
func TestShellPreSignaturePerAgent(t *testing.T) {
	store := NewGitStore(t.TempDir())
	const sessionID = "sess-123"
	parent := map[string]string{"a.go": "parent"}
	sub1 := map[string]string{"a.go": "sub1"}
	sub2 := map[string]string{"a.go": "sub2"}

	require.NoError(t, store.SaveShellPreSignature(sessionID, "", parent))
	require.NoError(t, store.SaveShellPreSignature(sessionID, "agent-1", sub1))
	require.NoError(t, store.SaveShellPreSignature(sessionID, "agent-2", sub2))

	store.DeleteShellPreSignature(sessionID, "agent-1")

	got, err := store.LoadShellPreSignature(sessionID, "")
	require.NoError(t, err)
	assert.Equal(t, parent, got)

	got, err = store.LoadShellPreSignature(sessionID, "agent-2")
	require.NoError(t, err)
	assert.Equal(t, sub2, got)

	_, err = store.LoadShellPreSignature(sessionID, "agent-1")
	assert.Error(t, err)
}
