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

	"github.com/stretchr/testify/require"
)

func TestLocalMarker(t *testing.T) {
	store := NewGitStore(t.TempDir())

	require.False(t, store.IsLocal(), "a fresh store is not in local mode")

	require.NoError(t, store.MarkLocal())
	require.True(t, store.IsLocal(), "MarkLocal puts the store in local mode")

	// Marking twice is idempotent.
	require.NoError(t, store.MarkLocal())
	require.True(t, store.IsLocal())

	require.NoError(t, store.RemoveLocal())
	require.False(t, store.IsLocal(), "RemoveLocal clears local mode")

	// Removing when absent is a no-op, not an error.
	require.NoError(t, store.RemoveLocal())
}

// TestLocalMarkerSurvivesWipe guards the property the pre-push skip relies on:
// a per-push wipe keeps the clone in local mode, so attestation stays off after
// the first session as well.
func TestLocalMarkerSurvivesWipe(t *testing.T) {
	store := NewGitStore(t.TempDir())

	require.NoError(t, store.MarkLocal())
	require.NoError(t, store.WipeTraceDir())

	require.True(t, store.IsLocal(), "local mode survives WipeTraceDir")
}
