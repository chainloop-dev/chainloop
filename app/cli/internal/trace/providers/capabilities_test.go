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

// TestSupportsSystemMessage pins which agents can actually show the
// session-start banner. Callers use this to decide whether building the
// banner is worth its cost, so a provider claiming support it does not have
// buys a control-plane round trip for a message nobody reads.
func TestSupportsSystemMessage(t *testing.T) {
	testCases := []struct {
		provider string
		want     bool
		why      string
	}{
		{
			provider: claude.Name,
			want:     true,
			why:      "Claude Code renders the systemMessage field of a hook response",
		},
		{
			provider: cursor.Name,
			want:     false,
			why:      "Cursor's hook response has no channel for displaying text",
		},
		{
			provider: opencode.Name,
			want:     true,
			why:      "the opencode plugin reads the hook's stdout and shows the banner as a TUI toast",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.provider, func(t *testing.T) {
			p := ByName(tc.provider)
			require.NotNil(t, p, "provider %q is not registered", tc.provider)

			assert.Equal(t, tc.want, p.SupportsSystemMessage(), tc.why)
		})
	}
}

// TestSupportsSystemMessageCoversEveryProvider fails when a provider is added
// without a decision recorded above, since the default zero value would
// silently claim no support.
func TestSupportsSystemMessageCoversEveryProvider(t *testing.T) {
	known := map[string]bool{claude.Name: true, cursor.Name: true, opencode.Name: true}

	for _, p := range All() {
		assert.True(t, known[p.Name()], "provider %q has no SupportsSystemMessage expectation", p.Name())
	}
}
