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
	"bytes"
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/internal/orcarouter"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrcaRouterCommandsAreDiscoverable proves both credential paths are
// subcommands of the provider's command, so a user with a key and a user
// without one each find their way without reading the source.
func TestOrcaRouterCommandsAreDiscoverable(t *testing.T) {
	cmd := newOrcaRouterCmd()
	assert.Equal(t, "orcarouter", cmd.Use)
	assert.Contains(t, cmd.Short, orcarouter.Name)

	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"login", "connect", "status", "logout", "models", "chat"} {
		assert.True(t, names[want], "subcommand %q is missing", want)
	}
}

// TestOrcaRouterHelpNamesBothCredentialPaths checks the two choices are
// described side by side, not only in code.
func TestOrcaRouterHelpNamesBothCredentialPaths(t *testing.T) {
	cmd := newOrcaRouterCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	require.NoError(t, cmd.Help())
	help := out.String()

	assert.Contains(t, help, "login")
	assert.Contains(t, help, "connect")
	assert.Contains(t, help, "OAuth 2.0 + PKCE")
	assert.Contains(t, help, "sk-orca-")
	assert.Contains(t, help, orcarouter.DocsURL)
	assert.Contains(t, help, orcarouter.ModelsURL)
	assert.Contains(t, help, orcarouter.BaseURL)
	assert.Contains(t, help, orcarouter.AuthBaseURL)
}

func TestDescribeConnectError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "denied", err: orcarouter.ErrAccessDenied, want: "declined"},
		{name: "state", err: orcarouter.ErrStateMismatch, want: "did not belong"},
		{name: "exchange", err: orcarouter.ErrExchangeRejected, want: "rejected"},
		{name: "scope", err: orcarouter.ErrScopeDowngrade, want: "scope"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := describeConnectError(tc.err).Error()
			assert.Contains(t, strings.ToLower(msg), tc.want)
			assert.NotContains(t, msg, "sk-orca-")
		})
	}
}

func TestOrcaRouterChatRequiresCredential(t *testing.T) {
	// With no stored credential the command fails with an actionable error and
	// never reaches the network.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	viper.Reset()
	t.Cleanup(viper.Reset)

	err := orcaRouterChat(t.Context(), &bytes.Buffer{}, orcarouter.DefaultModel, "hi")
	require.Error(t, err)
	assert.ErrorIs(t, err, orcarouter.ErrNoCredential)
}
