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

// The code is a modified version of the code from the SLSA GitHub generator
// https://github.com/slsa-framework/slsa-github-generator.

package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"

	"bytes"
	"io"

	"github.com/coreos/go-oidc/v3/oidc"
)

// DefaultActionsProviderURL is the default URL for GitHub Actions OIDC provider
var DefaultActionsProviderURL = "https://token.actions.githubusercontent.com"

const (
	// RequestTokenEnvKey is the environment variable for the GitHub Actions OIDC token request token
	RequestTokenEnvKey = "ACTIONS_ID_TOKEN_REQUEST_TOKEN"
	// RequestURLEnvKey is the environment variable for the GitHub Actions OIDC token request URL
	RequestURLEnvKey = "ACTIONS_ID_TOKEN_REQUEST_URL"
)

type GitHubOIDCClient struct {
	requestURL   *url.URL
	verifierFunc func(context.Context) (*oidc.IDTokenVerifier, error)
	bearerToken  string
	audience     []string
	token        *Token
}

// Token represents the contents of a GitHub OIDC JWT token.
type Token struct {
	oidc.IDToken

	// JobWorkflowRef is a reference to the current job workflow.
	JobWorkflowRef string `json:"job_workflow_ref"`

	// RunnerEnvironment is the environment the runner is running in.
	RunnerEnvironment string `json:"runner_environment"`

	// RawToken is the raw token string (for testing)
	RawToken string `json:"-"`
}

// Option is a functional option for configuring a GitHubOIDCClient.
type Option func(*GitHubOIDCClient)

// WithAudience sets the audience for the OIDC token.
func WithAudience(audience []string) Option {
	return func(c *GitHubOIDCClient) {
		c.audience = audience
	}
}

// NewGitHubClient returns new GitHub OIDC provider client.
func NewGitHubClient(opts ...Option) (*GitHubOIDCClient, error) {
	var c GitHubOIDCClient
	for _, opt := range opts {
		opt(&c)
	}

	// Get the request URL and token from env vars
	requestURL := os.Getenv(RequestURLEnvKey)
	if requestURL == "" {
		return nil, fmt.Errorf("url: %s environment variable not set; does your workflow have `id-token: write` scope?", RequestURLEnvKey)
	}

	parsedURL, err := url.ParseRequestURI(requestURL)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: invalid request URL %q: %w; does your workflow have `id-token: write` scope?",
			errURLError,
			requestURL, err,
		)
	}

	bearerToken := os.Getenv(RequestTokenEnvKey)
	if len(bearerToken) == 0 {
		return nil, fmt.Errorf("token: %s environment variable not set; does your workflow have `id-token: write` scope?", RequestTokenEnvKey)
	}

	c = GitHubOIDCClient{
		requestURL:  parsedURL,
		bearerToken: bearerToken,
	}

	// Only set the default verifier function if it hasn't been set by an option
	if c.verifierFunc == nil {
		c.verifierFunc = func(ctx context.Context) (*oidc.IDTokenVerifier, error) {
			provider, err := oidc.NewProvider(ctx, DefaultActionsProviderURL)
			if err != nil {
				return nil, err
			}
			return provider.Verifier(&oidc.Config{
				// we skip the check since we are not using a client IDs
				SkipClientIDCheck: true,
			}), nil
		}
	}

	return &c, nil
}

// Token requests an OIDC token from GitHub's provider, verifies it, and returns the token.
func (c *GitHubOIDCClient) Token(ctx context.Context) (any, error) {
	if c.token != nil {
		return c.token, nil
	}

	tokenBytes, err := c.requestToken(ctx, c.audience)
	if err != nil {
		return nil, fmt.Errorf("error retrieving request token: %w", err)
	}

	tokenPayload, err := c.decodePayload(tokenBytes)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	t, err := c.verifyToken(ctx, c.audience, tokenPayload)
	if err != nil {
		return nil, fmt.Errorf("error verifying token: %w", err)
	}

	token, err := c.decodeToken(t)
	if err != nil {
		return nil, fmt.Errorf("error decoding token: %w", err)
	}

	if err := c.verifyClaims(token); err != nil {
		return nil, fmt.Errorf("error verifying claims: %w", err)
	}

	// store the token for later re-use
	c.token = token
	return token, nil
}

func (c *GitHubOIDCClient) newRequestURL(audience []string) string {
	requestURL := *c.requestURL
	q := requestURL.Query()
	for _, a := range audience {
		q.Add("audience", a)
	}
	requestURL.RawQuery = q.Encode()
	return requestURL.String()
}

func (c *GitHubOIDCClient) requestToken(ctx context.Context, audience []string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.newRequestURL(audience), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: creating request: %w", errRequestError, err)
	}
	req.Header.Add("Authorization", "bearer "+c.bearerToken)
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRequestError, err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: reading response: %w", errRequestError, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: response: %s: %s", errRequestError, resp.Status, string(b))
	}
	return b, nil
}

func (c *GitHubOIDCClient) decodePayload(b []byte) (string, error) {
	var payload struct {
		Value string `json:"value"`
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	if err := decoder.Decode(&payload); err != nil {
		return "", fmt.Errorf("%w: parsing JSON: %w", errToken, err)
	}
	return payload.Value, nil
}

// verifyToken verifies the token contents and signature.
func (c *GitHubOIDCClient) verifyToken(ctx context.Context, audience []string, rawIDToken string) (*oidc.IDToken, error) {
	// Verify the token.
	verifier, err := c.verifierFunc(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: creating verifier: %w", errVerify, err)
	}

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("verify: could not verify token: %w", err)
	}

	// Verify the audience received is the one we requested.
	if slices.Compare(audience, idToken.Audience) != 0 {
		return nil, fmt.Errorf("%w: audience not equal %q != %q", errVerify, audience, idToken.Audience)
	}

	return idToken, nil
}

func (c *GitHubOIDCClient) decodeToken(token *oidc.IDToken) (*Token, error) {
	var t Token
	if err := token.Claims(&t); err != nil {
		return nil, fmt.Errorf("%w: getting claims: %w", errToken, err)
	}

	return &t, nil
}

func (c *GitHubOIDCClient) verifyClaims(token *Token) error {
	if token.JobWorkflowRef == "" {
		return fmt.Errorf("%w: job workflow ref is empty", errClaims)
	}
	if token.RunnerEnvironment == "" {
		return fmt.Errorf("%w: runner environment is empty", errClaims)
	}
	return nil
}
