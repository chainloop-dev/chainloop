//
// Copyright 2025 The Chainloop Authors.
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

package oidc_test

import (
	"context"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/oidc"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNewGitlabClient(t *testing.T) {
	testLogger := zerolog.New(zerolog.Nop()).Level(zerolog.Disabled)
	ctx := context.Background()

	// Save original environment variables
	originalServerURL := os.Getenv(oidc.CIServerURLEnv)
	originalToken := os.Getenv(oidc.GitlabTokenEnv)
	defer func() {
		t.Setenv(oidc.CIServerURLEnv, originalServerURL)
		t.Setenv(oidc.GitlabTokenEnv, originalToken)
	}()

	tests := []struct {
		name              string
		setupEnv          func(t *testing.T)
		expectErr         bool
		expectErrContains string
	}{
		{
			name: "Missing server URL",
			setupEnv: func(t *testing.T) {
				t.Setenv(oidc.CIServerURLEnv, "")
				t.Setenv(oidc.GitlabTokenEnv, "test-token")
			},
			expectErr:         true,
			expectErrContains: "environment variable not set",
		},
		{
			name: "Missing OIDC token",
			setupEnv: func(t *testing.T) {
				t.Setenv(oidc.CIServerURLEnv, "https://gitlab.example.com")
				t.Setenv(oidc.GitlabTokenEnv, "")
			},
			expectErr:         true,
			expectErrContains: "environment variable not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupEnv(t)
			client, err := oidc.NewGitlabClient(ctx, &testLogger)

			if tt.expectErr {
				assert.Error(t, err)
				if tt.expectErrContains != "" {
					assert.Contains(t, err.Error(), tt.expectErrContains)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}
