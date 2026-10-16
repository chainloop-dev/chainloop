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

package claude

import (
	"encoding/json"
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/state"
	"github.com/stretchr/testify/require"
)

func TestTagShellCommand_PreservesInputAndExportsCallToken(t *testing.T) {
	// Given
	input := &trace.HookInput{
		Command:      "git commit -m test",
		RawToolInput: json.RawMessage(`{"command":"git commit -m test","description":"commit changes","timeout":120000}`),
	}

	// When
	out := captureStdout(t, func() {
		require.NoError(t, New().TagShellCommand(input, "opaque-token"))
	})

	// Then
	var response struct {
		HookSpecificOutput struct {
			HookEventName string `json:"hookEventName"`
			UpdatedInput  struct {
				Command     string `json:"command"`
				Description string `json:"description"`
				Timeout     int    `json:"timeout"`
			} `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &response))
	require.Equal(t, eventPreToolUse, response.HookSpecificOutput.HookEventName)
	require.Equal(t, "export "+state.ShellCallEnv+"=opaque-token; git commit -m test",
		response.HookSpecificOutput.UpdatedInput.Command)
	require.Equal(t, "commit changes", response.HookSpecificOutput.UpdatedInput.Description)
	require.Equal(t, 120000, response.HookSpecificOutput.UpdatedInput.Timeout)
}
