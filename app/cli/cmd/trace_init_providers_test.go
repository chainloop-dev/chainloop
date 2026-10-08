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

	"github.com/chainloop-dev/chainloop/app/cli/internal/trace/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// providerClaudeCode is the wire name of the default provider, spelled out
// rather than taken from the package constant so that renaming it shows up here
// as a failing test instead of passing silently.
const providerClaudeCode = "claude-code"

func TestResolveTraceProviders(t *testing.T) {
	testCases := []struct {
		name        string
		flags       traceProviderFlags
		interactive bool
		// configured are the harnesses whose hooks the repository already has
		configured []string
		// answer is what the user ticks in the multi-select
		answer []string
		want   []string
		// wantPrompted is whether the multi-select should have been shown
		wantPrompted bool
		// wantDefaults is what the multi-select comes up with ticked
		wantDefaults []string
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
			want:        []string{providerClaudeCode, "opencode"},
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
			wantDefaults: []string{providers.DefaultProvider},
		},
		{
			name:         "a re-run comes up with the configured harnesses ticked",
			interactive:  true,
			configured:   []string{providerClaudeCode, "cursor", "opencode"},
			answer:       []string{providerClaudeCode, "cursor", "opencode"},
			want:         []string{providerClaudeCode, "cursor", "opencode"},
			wantPrompted: true,
			wantDefaults: []string{providerClaudeCode, "cursor", "opencode"},
		},
		{
			name:         "a re-run without the default ticks only what is configured",
			interactive:  true,
			configured:   []string{"cursor"},
			answer:       []string{"cursor"},
			want:         []string{"cursor"},
			wantPrompted: true,
			wantDefaults: []string{"cursor"},
		},
		{
			name:        "flags still win over the configured harnesses",
			flags:       traceProviderFlags{opencode: true},
			interactive: true,
			configured:  []string{"cursor"},
			want:        []string{"opencode"},
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

			var readConfigured bool
			configured := func() []string {
				readConfigured = true
				return tc.configured
			}

			got, err := resolveTraceProviders(p, tc.flags, tc.interactive, configured)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)

			if !tc.wantPrompted {
				assert.Empty(t, p.multiSelects, "no question should have been asked")
				assert.False(t, readConfigured, "the repository is only read to preselect the question")
				return
			}

			require.Len(t, p.multiSelects, 1)
			// Every registered provider is offered.
			assert.Equal(t, traceProviderNames(), p.multiSelects[0].options)
			assert.Equal(t, tc.wantDefaults, p.multiSelects[0].defaults)
		})
	}
}

func TestResolveTraceProvidersPromptError(t *testing.T) {
	_, err := resolveTraceProviders(&fakePrompter{multiSelectErr: errAborted}, traceProviderFlags{}, true, func() []string { return nil })
	require.ErrorIs(t, err, errAborted)
}

// TestConfiguredTraceProviders reads the harnesses back from hooks the real
// providers wrote, which is what a re-run of trace init finds (PFM-7633).
func TestConfiguredTraceProviders(t *testing.T) {
	testCases := []struct {
		name      string
		installed []string
		want      []string
	}{
		{
			name: "a repository never set up",
			want: []string{},
		},
		{
			name:      "some harnesses set up",
			installed: []string{"opencode", "cursor"},
			// In the registry's order, the same order the prompt lists them in.
			want: []string{"cursor", "opencode"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			for _, p := range providers.ByNames(tc.installed) {
				require.NoError(t, p.InstallHooks(repoRoot))
			}

			assert.Equal(t, tc.want, configuredTraceProviders(repoRoot))
		})
	}
}

// TestConfiguredTraceProvidersSkipsUnreadableSettings keeps a broken harness
// configuration from stopping init, which would otherwise be the way to repair it.
func TestConfiguredTraceProvidersSkipsUnreadableSettings(t *testing.T) {
	repoRoot := t.TempDir()

	cursor := providers.ByName("cursor")
	require.NoError(t, cursor.InstallHooks(repoRoot))

	claudeSettings := providers.ByName(providerClaudeCode).SettingsFile(repoRoot)
	require.NoError(t, os.MkdirAll(filepath.Dir(claudeSettings), 0o755))
	require.NoError(t, os.WriteFile(claudeSettings, []byte("{not json"), 0o600))

	assert.Equal(t, []string{"cursor"}, configuredTraceProviders(repoRoot))
}

func TestTraceProviderFlags(t *testing.T) {
	t.Run("no flag set", func(t *testing.T) {
		assert.False(t, traceProviderFlags{}.any())
		assert.Empty(t, traceProviderFlags{}.names())
	})

	t.Run("names follow the registry order, not the flag order", func(t *testing.T) {
		f := traceProviderFlags{opencode: true, claude: true}
		assert.True(t, f.any())
		assert.Equal(t, []string{providerClaudeCode, "opencode"}, f.names())
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
