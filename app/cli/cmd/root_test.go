// Copyright 2024-2026 The Chainloop Authors.
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
	"testing"

	"github.com/rs/zerolog"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAuthToken(t *testing.T) {
	const (
		envToken  = "env-token"
		userTok   = "user-token"
		flagToken = "flag-token"
	)

	testCases := []struct {
		name string
		// inputs
		envVar         string
		userToken      string
		flag           string
		apiTokenPref   bool
		expectedToken  string
		expectedIsUser bool
	}{
		{
			name:           "only env var set, non API-preferred command",
			envVar:         envToken,
			expectedToken:  envToken,
			expectedIsUser: false,
		},
		{
			name:           "only user token set",
			userToken:      userTok,
			expectedToken:  userTok,
			expectedIsUser: true,
		},
		{
			name:           "both set, command does not prefer API token, env var wins",
			envVar:         envToken,
			userToken:      userTok,
			apiTokenPref:   false,
			expectedToken:  envToken,
			expectedIsUser: false,
		},
		{
			name:           "both set, command prefers API token, env var wins",
			envVar:         envToken,
			userToken:      userTok,
			apiTokenPref:   true,
			expectedToken:  envToken,
			expectedIsUser: false,
		},
		{
			name:           "flag takes precedence over env var and user token",
			envVar:         envToken,
			userToken:      userTok,
			flag:           flagToken,
			expectedToken:  flagToken,
			expectedIsUser: false,
		},
		{
			name:           "nothing set returns empty user token",
			expectedToken:  "",
			expectedIsUser: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset and set up the global state the function relies on
			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.Set(confOptions.authToken.viperKey, tc.userToken)

			if tc.envVar != "" {
				t.Setenv(tokenEnvVarName, tc.envVar)
			}

			// apiToken is a package-level variable bound to the --token flag
			prevAPIToken := apiToken
			apiToken = tc.flag
			t.Cleanup(func() { apiToken = prevAPIToken })

			cmd := &cobra.Command{Annotations: map[string]string{}}
			if tc.apiTokenPref {
				cmd.Annotations[useAPIToken] = trueString
			}

			got, isUser, err := loadAuthToken(cmd)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedToken, got)
			assert.Equal(t, tc.expectedIsUser, isUser)
		})
	}
}

func TestNewRootCmdBindsOrganizationEnvironment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	t.Setenv(CalculateEnvVarName(confOptions.organization.viperKey), "environment-org")

	root := NewRootCmd(zerolog.Nop())
	assert.Equal(t, "environment-org", viper.GetString(confOptions.organization.viperKey))

	require.NoError(t, root.PersistentFlags().Set(confOptions.organization.flagName, "flag-org"))
	assert.Equal(t, "flag-org", viper.GetString(confOptions.organization.viperKey))
}

// Cobra rejects unknown subcommands and invalid flags before PersistentPreRunE
// runs, so main prints those errors through a Logger() that PersistentPreRunE
// never got to initialize. NewRootCmd must leave the root logger usable.
func TestNewRootCmdInitializesLogger(t *testing.T) {
	previous := logger
	t.Cleanup(func() { logger = previous })

	var out bytes.Buffer
	NewRootCmd(zerolog.New(&out))

	rootLogger := Logger()
	rootLogger.Error().Msg("boom")
	assert.Contains(t, out.String(), "boom")
}

// A command flag with the name or the shorthand of a global flag hides the global flag on that
// command. For example, a local --org hides the global --org that selects the organization.
func TestNoCommandShadowsAGlobalFlag(t *testing.T) {
	root := NewRootCmd(zerolog.Nop())

	globalNames := map[string]bool{}
	globalShorthands := map[string]bool{}
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		globalNames[f.Name] = true
		if f.Shorthand != "" {
			globalShorthands[f.Shorthand] = true
		}
	})

	// Flags that hid a global flag before this test existed. Do not add to this list.
	known := map[string]bool{
		"chainloop artifact download --output": true,
		"chainloop trace uninstall --yes":      true,
		"chainloop trace uninstall -y":         true,
	}

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		for _, sub := range c.Commands() {
			sub.LocalFlags().VisitAll(func(f *pflag.Flag) {
				if name := sub.CommandPath() + " --" + f.Name; globalNames[f.Name] && !known[name] {
					assert.Failf(t, "global flag hidden", "%q defines --%s, which hides the global flag", sub.CommandPath(), f.Name)
				}
				if name := sub.CommandPath() + " -" + f.Shorthand; f.Shorthand != "" && globalShorthands[f.Shorthand] && !known[name] {
					assert.Failf(t, "global flag hidden", "%q defines -%s, which hides the global flag", sub.CommandPath(), f.Shorthand)
				}
			})
			walk(sub)
		}
	}
	walk(root)
}
