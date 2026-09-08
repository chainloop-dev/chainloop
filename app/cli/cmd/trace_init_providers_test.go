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
	"testing"

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveTraceProviders(t *testing.T) {
	testCases := []struct {
		name        string
		flags       traceProviderFlags
		interactive bool
		// answer is what the user ticks in the multi-select
		answer []string
		want   []string
		// wantPrompted is whether the multi-select should have been shown
		wantPrompted bool
		wantErr      string
	}{
		{
			name:  "a single flag wins and asks nothing",
			flags: traceProviderFlags{cursor: true},
			// Interactive, to show the flag is what suppresses the question.
			interactive: true,
			want:        []string{"cursor"},
		},
		{
			name:        "several flags are all honored",
			flags:       traceProviderFlags{claude: true, opencode: true},
			interactive: true,
			want:        []string{"claude-code", "opencode"},
		},
		{
			name: "without a terminal the default provider is kept",
			want: []string{providers.DefaultProvider},
		},
		{
			name:         "on a terminal the user picks",
			interactive:  true,
			answer:       []string{"cursor", "opencode"},
			want:         []string{"cursor", "opencode"},
			wantPrompted: true,
		},
		{
			name:         "picking nothing is refused",
			interactive:  true,
			answer:       []string{},
			wantPrompted: true,
			wantErr:      "at least one",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			p := &fakePrompter{multiSelectAnswer: tc.answer}

			got, err := resolveTraceProviders(p, tc.flags, tc.interactive)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)

			if !tc.wantPrompted {
				assert.Empty(t, p.multiSelects, "no question should have been asked")
				return
			}

			require.Len(t, p.multiSelects, 1)
			// Every registered provider is offered, with the default ticked.
			assert.Equal(t, traceProviderNames(), p.multiSelects[0].options)
			assert.Equal(t, []string{providers.DefaultProvider}, p.multiSelects[0].defaults)
		})
	}
}

func TestResolveTraceProvidersPromptError(t *testing.T) {
	_, err := resolveTraceProviders(&fakePrompter{multiSelectErr: errAborted}, traceProviderFlags{}, true)
	require.ErrorIs(t, err, errAborted)
}

func TestTraceProviderFlags(t *testing.T) {
	t.Run("no flag set", func(t *testing.T) {
		assert.False(t, traceProviderFlags{}.any())
		assert.Empty(t, traceProviderFlags{}.names())
	})

	t.Run("names follow the registry order, not the flag order", func(t *testing.T) {
		f := traceProviderFlags{opencode: true, claude: true}
		assert.True(t, f.any())
		assert.Equal(t, []string{"claude-code", "opencode"}, f.names())
	})
}

// TestTraceProviderNamesMatchesRegistry guards the multi-select against a
// provider being added to the registry but never offered.
func TestTraceProviderNamesMatchesRegistry(t *testing.T) {
	names := traceProviderNames()
	require.Len(t, names, len(providers.All()))

	for _, name := range names {
		assert.NotNil(t, providers.ByName(name), "%q is offered but not registered", name)
	}

	assert.Contains(t, names, providers.DefaultProvider, "the default must be offerable")
}
