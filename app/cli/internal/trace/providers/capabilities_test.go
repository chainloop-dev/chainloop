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

package providers

import (
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/claude"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/cursor"
	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/opencode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSessionStartChannels pins which agents can actually receive each part of
// the session-start message. Callers use this to decide whether building a
// part is worth its cost, so a provider claiming support it does not have buys
// a control-plane round trip for a banner nobody reads, or an instruction no
// model acts on.
func TestSessionStartChannels(t *testing.T) {
	testCases := []struct {
		provider        string
		wantBanner      bool
		wantInstruction bool
		wantReminder    bool
		why             string
	}{
		{
			provider:        claude.Name,
			wantBanner:      true,
			wantInstruction: true,
			wantReminder:    true,
			why:             "Claude Code renders systemMessage to the user and feeds additionalContext of SessionStart and UserPromptSubmit to the model",
		},
		{
			provider:        cursor.Name,
			wantBanner:      false,
			wantInstruction: true,
			wantReminder:    false,
			why:             "Cursor's sessionStart response feeds additional_context to the model, and Cursor documents no channel that adds context at prompt submit",
		},
		{
			provider:        opencode.Name,
			wantBanner:      true,
			wantInstruction: true,
			wantReminder:    true,
			why:             "the opencode plugin posts the instruction and the reminder as context-only messages, and shows the banner as a toast in OpenCode 1.x and as the description of a synthetic message in OpenCode 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.provider, func(t *testing.T) {
			p := ByName(tc.provider)
			require.NotNil(t, p, "provider %q is not registered", tc.provider)

			assert.Equal(t, tc.wantBanner, p.SupportsSessionStartBanner(), tc.why)
			assert.Equal(t, tc.wantInstruction, p.SupportsSessionStartInstruction(), tc.why)
			assert.Equal(t, tc.wantReminder, p.SupportsPromptReminder(), tc.why)
		})
	}
}

// TestSessionStartChannelsCoverEveryProvider fails when a provider is added
// without a decision recorded above, since the default zero value would
// silently claim no support.
func TestSessionStartChannelsCoverEveryProvider(t *testing.T) {
	known := map[string]bool{claude.Name: true, cursor.Name: true, opencode.Name: true}

	for _, p := range All() {
		assert.True(t, known[p.Name()], "provider %q has no session-start channel expectation", p.Name())
	}
}
