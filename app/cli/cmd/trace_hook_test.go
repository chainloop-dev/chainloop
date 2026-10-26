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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraceHookPiCommands(t *testing.T) {
	root := newTraceHookCmd()
	for _, name := range []string{"session-start", "user-prompt-submit", "session-end", "pre-tool-use", "post-tool-use"} {
		command, args, err := root.Find([]string{"pi", name})
		require.NoError(t, err)
		assert.Equal(t, name, command.Name())
		assert.Empty(t, args)
	}
}

func TestApplyPrePushPolicy(t *testing.T) {
	failure := errors.New("setup failed")
	prePush := newTraceHookGitPrePushCmd()
	other := &cobra.Command{Use: "other"}

	testCases := []struct {
		name         string
		executed     *cobra.Command
		err          error
		requireTrace bool
		wantErr      bool
	}{
		{name: "success stays success", executed: prePush},
		{name: "pre-push failure fails open by default", executed: prePush, err: failure},
		{name: "pre-push failure blocks with requireTrace", executed: prePush, err: failure, requireTrace: true, wantErr: true},
		{name: "other commands keep their error", executed: other, err: failure, wantErr: true},
		{name: "unknown command keeps its error", err: failure, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.requireTrace {
				require.NoError(t, os.WriteFile(filepath.Join(dir, ".chainloop.yml"), []byte("requireTrace: true\n"), 0600))
			}
			t.Chdir(dir)

			err := applyPrePushPolicy(tc.executed, tc.err)
			if tc.wantErr {
				require.ErrorIs(t, err, failure)
				return
			}

			assert.NoError(t, err)
		})
	}
}
