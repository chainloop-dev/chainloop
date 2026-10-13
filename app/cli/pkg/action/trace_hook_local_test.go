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
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsLocalTrace(t *testing.T) {
	t.Run("a normal clone is not local", func(t *testing.T) {
		_, gitDir := initGitRepo(t)
		require.NoError(t, state.NewGitStore(gitDir).InitTraceDir())

		assert.False(t, isLocalTrace())
	})

	t.Run("a clone marked local is local", func(t *testing.T) {
		_, gitDir := initGitRepo(t)
		store := state.NewGitStore(gitDir)
		require.NoError(t, store.InitTraceDir())
		require.NoError(t, store.MarkLocal())

		assert.True(t, isLocalTrace())
	})
}

// TestHandlePrePushHookSkipsLocal checks the pre-push hook pushes no attestation
// in local mode. ActionOpts is nil on purpose: were the hook to reach the
// attestation path with a session to push, building the executor would fail, so
// a clean return is only possible because the local marker short-circuits it.
func TestHandlePrePushHookSkipsLocal(t *testing.T) {
	_, gitDir := initGitRepo(t)
	store := state.NewGitStore(gitDir)
	require.NoError(t, store.InitTraceDir())
	require.NoError(t, store.MarkLocal())

	// drainPushStdin reads to EOF; give it an empty input so the test never
	// blocks on the terminal.
	devNull, err := os.Open(os.DevNull)
	require.NoError(t, err)
	t.Cleanup(func() { _ = devNull.Close() })
	origStdin := os.Stdin
	os.Stdin = devNull
	t.Cleanup(func() { os.Stdin = origStdin })

	require.NoError(t, HandlePrePushHook(t.Context(), zerolog.Nop(), RunTracePushOpts{AllowEmpty: true}))
}
