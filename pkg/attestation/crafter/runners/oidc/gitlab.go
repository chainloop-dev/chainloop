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
	"slices"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog"
)

// GitlabTokenEnv is the environment variable name for Gitlab OIDC token.
// #nosec G101 - This is just the name of an environment variable, not a credential
const GitlabTokenEnv = "GITLAB_OIDC"

// CIServerURLEnv is the environment variable name for Gitlab CI server URL.
const CIServerURLEnv = "CI_SERVER_URL"

// ExpectedAudience is the expected audience for the Gitlab OIDC token.
const ExpectedAudience = "chainloop"

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

func NewGitlabClient(ctx context.Context, authToken string, logger *zerolog.Logger) (*GitlabOIDCClient, error) {
	var c GitlabOIDCClient

	// retrieve the Gitlab server on which the pipeline is running, which is the provider URL
	providerURL := os.Getenv(CIServerURLEnv)
	logger.Debug().Str("providerURL", providerURL).Msg("retrieved provider URL")
	if providerURL == "" {
		return nil, fmt.Errorf("%s environment variable not set", CIServerURLEnv)
	}

	token, err := loadTokenFromEnvOrRawToken(ctx, authToken, providerURL, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	logger.Debug().Msg("OIDC token loaded successfully")

	c.Token = token
	return &c, nil
}

func parseToken(ctx context.Context, providerURL string, tokenString string) (*GitlabToken, error) {
	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to OIDC provider: %w", err)
	}

	verifier := provider.Verifier(&oidc.Config{
		SkipClientIDCheck: true, // Skip client ID check since we're just parsing
	})

	idToken, err := verifier.Verify(ctx, tokenString)
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	token := &GitlabToken{
		IDToken: *idToken,
	}

	// Verify the audience
	if !slices.Contains(idToken.Audience, ExpectedAudience) {
		return nil, fmt.Errorf("invalid audience: expected %q", ExpectedAudience)
	}

	// Extract claims to populate our custom fields
	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to extract claims: %w", err)
	}

	if configRefURI, ok := claims["ci_config_ref_uri"].(string); ok {
		token.ConfigRefURI = configRefURI
	}

	if runnerEnv, ok := claims["runner_environment"].(string); ok {
		token.RunnerEnvironment = runnerEnv
	}

	return token, nil
}

// To load the auth token we do it in two steps:
// 1 - check if it's explicitly set as an environment variable GITLAB_OIDC
// 2 - if not, try to parse it from the input token string
func loadTokenFromEnvOrRawToken(ctx context.Context, authToken string, providerURL string, logger *zerolog.Logger) (*GitlabToken, error) {
	tokenContent := os.Getenv(GitlabTokenEnv)
	if tokenContent == "" && authToken == "" {
		return nil, fmt.Errorf("no token provided, neither explicitly nor as an environment variable %s", GitlabTokenEnv)
	}

	if tokenContent != "" {
		logger.Debug().Msgf("retrieved token content from environment variable %s", GitlabTokenEnv)
		return parseToken(ctx, providerURL, tokenContent)
	}

	logger.Debug().Msg("no token content in environment variable, trying to parse from raw token")
	parsedToken, err := parseToken(ctx, providerURL, authToken)
	if err != nil {
		return nil, fmt.Errorf("Invalid authentication token provided, %w", err)
	}

	return parsedToken, nil
}
