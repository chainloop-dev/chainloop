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

package oidc

import (
	"context"
	"fmt"
	"os"

	"github.com/coreos/go-oidc/v3/oidc"
)

// GITLAB_OIDC_TOKEN_ENV_KEY is the environment variable name for Gitlab OIDC token.
var GITLAB_OIDC_TOKEN_ENV_KEY = "GITLAB_OIDC"

// CI_SERVER_URL_ENV_KEY is the environment variable name for Gitlab CI server URL.
var CI_SERVER_URL_ENV_KEY = "CI_SERVER_URL"

type GitlabToken struct {
	oidc.IDToken

	// ConfigRefURI is a reference to the current job workflow.
	ConfigRefURI string `json:"ci_config_ref_uri"`

	// RunnerEnvironment is the environment the runner is running in.
	RunnerEnvironment string `json:"runner_environment"`
}

type GitlabOIDCClient struct {
	Token *GitlabToken
}

func NewGitlabClient(ctx context.Context) (*GitlabOIDCClient, error) {
	var c GitlabOIDCClient

	// retrieve the Gitlab server on which the pipeline is running, which is the provider URL
	providerURL := os.Getenv(CI_SERVER_URL_ENV_KEY)
	if providerURL == "" {
		return nil, fmt.Errorf("%s environment variable not set", CI_SERVER_URL_ENV_KEY)
	}

	tokenContent := os.Getenv(GITLAB_OIDC_TOKEN_ENV_KEY)
	if tokenContent == "" {
		return nil, fmt.Errorf("%s environment variable not set", GITLAB_OIDC_TOKEN_ENV_KEY)
	}

	token, err := parseToken(ctx, providerURL, tokenContent)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	c.Token = token
	return &c, nil
}

func parseToken(ctx context.Context, providerURL string, tokenString string) (*GitlabToken, error) {
	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to OIDC provider: %v", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true, // Skip client ID check since we're just parsing
		SkipExpiryCheck:   true, // Skip expiry check to allow viewing expired tokens
	})

	idToken, err := verifier.Verify(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %v", err)
	}

	token := &GitlabToken{
		IDToken: *idToken,
	}

	// Extract claims to populate our custom fields
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %v", err)
	}

	if configRefURI, ok := claims["ci_config_ref_uri"].(string); ok {
		token.ConfigRefURI = configRefURI
	}

	if runnerEnv, ok := claims["runner_environment"].(string); ok {
		token.RunnerEnvironment = runnerEnv
	}

	return token, nil
}
