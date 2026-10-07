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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHooksInstalled checks, for every registered provider, that HooksInstalled
// follows what InstallHooks and UninstallHooks leave in the repository. trace
// init relies on it to offer the harnesses already set up on a re-run.
func TestHooksInstalled(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(t *testing.T, p trace.Provider, repoRoot string)
		want  bool
	}{
		{
			name:  "a repository never set up",
			setup: func(*testing.T, trace.Provider, string) {},
			want:  false,
		},
		{
			name: "after init installs the hooks",
			setup: func(t *testing.T, p trace.Provider, repoRoot string) {
				require.NoError(t, p.InstallHooks(repoRoot))
			},
			want: true,
		},
		{
			name: "after the hooks are uninstalled again",
			setup: func(t *testing.T, p trace.Provider, repoRoot string) {
				require.NoError(t, p.InstallHooks(repoRoot))
				require.NoError(t, p.UninstallHooks(repoRoot))
			},
			want: false,
		},
	}

	for _, p := range All() {
		for _, tc := range testCases {
			t.Run(p.Name()+"/"+tc.name, func(t *testing.T) {
				repoRoot := t.TempDir()
				tc.setup(t, p, repoRoot)

				got, err := p.HooksInstalled(repoRoot)
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			})
		}
	}
}
