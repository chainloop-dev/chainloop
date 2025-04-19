//
// Copyright 2023 The Chainloop Authors.
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

	"bytes"
	"io"

	"github.com/coreos/go-oidc/v3/oidc"
)

var defaultActionsProviderURL = "https://token.actions.githubusercontent.com"
var defaultAudience = []string{"nobody"}

const (
	requestTokenEnvKey = "ACTIONS_ID_TOKEN_REQUEST_TOKEN"
	requestURLEnvKey   = "ACTIONS_ID_TOKEN_REQUEST_URL"
)

type GitHubOIDCClient struct {
	requestURL   *url.URL
	verifierFunc func(context.Context) (*oidc.IDTokenVerifier, error)
	bearerToken  string
}

// NewOIDCGitHubClient returns new GitHub OIDC provider client.
func NewOIDCGitHubClient(_ context.Context) (Client, error) {
	var c GitHubOIDCClient

	// Get the request URL and token from env vars
	requestURL := os.Getenv(requestURLEnvKey)
	if requestURL == "" {
		return nil, fmt.Errorf("url: %s environment variable not set; does your workflow have `id-token: write` scope?", requestURLEnvKey)
	}

	parsedURL, err := url.ParseRequestURI(requestURL)
	if err != nil {
		return nil, fmt.Errorf(
			"%w: invalid request URL %q: %w; does your workflow have `id-token: write` scope?",
			errURLError,
			requestURL, err,
		)
	}

	c = GitHubOIDCClient{
		requestURL:  parsedURL,
		bearerToken: os.Getenv(requestTokenEnvKey),
	}
	c.verifierFunc = func(ctx context.Context) (*oidc.IDTokenVerifier, error) {
		provider, err := oidc.NewProvider(ctx, defaultActionsProviderURL)
		if err != nil {
			return nil, err
		}
		return provider.Verifier(&oidc.Config{
			SkipClientIDCheck: true,
		}), nil
	}
	return &c, nil
}

// Token requests an OIDC token from GitHub's provider, verifies it, and returns the token.
func (c *GitHubOIDCClient) Token(ctx context.Context) (*Token, error) {
	tokenBytes, err := c.requestToken(ctx, defaultAudience)
	if err != nil {
		return nil, err
	}

	tokenPayload, err := c.decodePayload(tokenBytes)
	if err != nil {
		return nil, err
	}

	t, err := c.verifyToken(ctx, defaultAudience, tokenPayload)
	if err != nil {
		return nil, err
	}

	token, err := c.decodeToken(t)
	if err != nil {
		return nil, err
	}

	if err := c.verifyClaims(token); err != nil {
		return nil, err
	}

	token.RawToken = tokenPayload

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
	if !compareStringSlice(audience, idToken.Audience) {
		return nil, fmt.Errorf("%w: audience not equal %q != %q", errVerify, audience, idToken.Audience)
	}

	return idToken, nil
}

func (c *GitHubOIDCClient) decodeToken(token *oidc.IDToken) (*Token, error) {
	var t Token
	t.Issuer = token.Issuer
	t.Audience = token.Audience
	t.Expiry = token.Expiry

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
