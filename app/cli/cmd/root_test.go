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
	"testing"

	"github.com/spf13/cobra"
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
