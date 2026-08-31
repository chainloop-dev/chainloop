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
	store := NewGitStore(t.TempDir())
	sessionID := "sess-123"
	sig := map[string]string{
		"a.go":       "hash-a",
		"sub/b.json": "hash-b",
	}

	require.NoError(t, store.SaveShellPreSignature(sessionID, sig))

	loaded, err := store.LoadShellPreSignature(sessionID)
	require.NoError(t, err)
	assert.Equal(t, sig, loaded)

	store.DeleteShellPreSignature(sessionID)

	_, err = store.LoadShellPreSignature(sessionID)
	assert.Error(t, err, "signature should be gone after delete")
}
