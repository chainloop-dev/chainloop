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

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resetHookLoggerState restores the package-level logger globals to their
// defaults so tests don't leak state to each other.
func resetHookLoggerState(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		closeHookLogFile()
		stderrMinLevel = zerolog.WarnLevel
	})
}

func TestInitHookLogFile(t *testing.T) {
	store := state.NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())
	resetHookLoggerState(t)

	require.NoError(t, initHookLogFile(store))
	defer closeHookLogFile()

	// Write a debug message — should appear in the log file
	logger.Debug().Msg("test debug message")

	// Close to flush
	closeHookLogFile()

	logPath := store.LogFilePath()
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test debug message")
}

func TestInitHookLogFileCreatesFile(t *testing.T) {
	store := state.NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	resetHookLoggerState(t)

	logPath := store.LogFilePath()
	_, err := os.Stat(logPath)
	assert.True(t, os.IsNotExist(err))

	require.NoError(t, initHookLogFile(store))
	closeHookLogFile()

	_, err = os.Stat(logPath)
	assert.NoError(t, err)
}

func TestInitHookLogFileAppendsToExisting(t *testing.T) {
	store := state.NewGitStore(t.TempDir())
	require.NoError(t, store.InitTraceDir())

	resetHookLoggerState(t)

	logPath := store.LogFilePath()
	require.NoError(t, os.WriteFile(logPath, []byte("existing content\n"), 0600))

	require.NoError(t, initHookLogFile(store))
	logger.Debug().Msg("appended message")
	closeHookLogFile()

	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "existing content")
	assert.Contains(t, string(content), "appended message")
}

func TestInitHookLogFileFailsOnBadPath(t *testing.T) {
	err := initHookLogFile(state.NewGitStore(filepath.Join(t.TempDir(), "nonexistent", "deep")))
	assert.Error(t, err)
}

func TestCloseHookLogFileIdempotent(_ *testing.T) {
	// Calling close without init should not panic
	closeHookLogFile()
	closeHookLogFile()
}
