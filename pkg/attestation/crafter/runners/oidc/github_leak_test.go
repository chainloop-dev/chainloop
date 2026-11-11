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

package oidc

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A distinctive, JWT-shaped placeholder standing in for the federated identity
// token. It is not a real credential; the tests only check it never shows up in logs.
const fakeFederatedToken = "eyJhbGciOiJSUzI1NiJ9." +
	"eyJzdWIiOiJ0ZXN0LXN1YmplY3QiLCJqb2Jfd29ya2Zsb3dfcmVmIjoicmVmIn0." +
	"ZmFrZS1zaWduYXR1cmUtbm90LWEtcmVhbC10b2tlbg"

// TestTokenDoesNotLeakRawFederatedToken makes sure the GitHub OIDC runner never
// writes the raw federated token to its debug logs. The token flows through
// Token() -> decodePayload (the raw token-endpoint response body) and verifyToken
// (rawIDToken); logging either at debug level would expose a usable, short-lived
// identity token in CI job logs.
func TestTokenDoesNotLeakRawFederatedToken(t *testing.T) {
	// GitHub's token endpoint returns the JWT wrapped as {"value": "<jwt>"}.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"value": %q}`, fakeFederatedToken)
	}))
	defer server.Close()

	t.Setenv(RequestURLEnvKey, server.URL)
	t.Setenv(RequestTokenEnvKey, "request-bearer-token")

	// Capture everything written at debug level into a buffer.
	var logs strings.Builder
	prevGlobal := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	defer zerolog.SetGlobalLevel(prevGlobal)
	logger := zerolog.New(&logs).Level(zerolog.TraceLevel)

	client, err := NewGitHubClient(&logger)
	require.NoError(t, err)

	// Stub out signature verification so the test stays offline and deterministic.
	// The token is already logged before the verifier is ever invoked, so this does
	// not hide the behaviour under test; it only removes the network dependency.
	client.verifierFunc = func(_ context.Context) (*oidc.IDTokenVerifier, error) {
		return nil, fmt.Errorf("verification stubbed out for test")
	}

	// Verification is expected to fail; we only care about what got logged on the way.
	_, _ = client.Token(context.Background())

	// Collect every debug line that leaked the token, for actionable failure output.
	var leaking []string
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		if line != "" && strings.Contains(line, fakeFederatedToken) {
			leaking = append(leaking, line)
		}
	}

	assert.Emptyf(t, leaking,
		"the raw federated OIDC token was written to debug logs in %d message(s):\n%s",
		len(leaking), strings.Join(leaking, "\n"))
}

// TestVerifyClaimsDoesNotLeakRawToken covers the other raw-token path: verifyClaims
// stringifies the whole *Token, whose exported RawToken field holds the raw JWT
// (set in Token() before verifyClaims runs). Logging the struct would expose it.
func TestVerifyClaimsDoesNotLeakRawToken(t *testing.T) {
	var logs strings.Builder
	prevGlobal := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	defer zerolog.SetGlobalLevel(prevGlobal)
	logger := zerolog.New(&logs).Level(zerolog.TraceLevel)

	c := &GitHubOIDCClient{logger: &logger}
	token := &Token{
		JobWorkflowRef:    "octo-org/octo-repo/.github/workflows/ci.yml@refs/heads/main",
		RunnerEnvironment: "github-hosted",
		RawToken:          fakeFederatedToken,
	}

	require.NoError(t, c.verifyClaims(token))

	assert.NotContainsf(t, logs.String(), fakeFederatedToken,
		"verifyClaims wrote the raw federated token to debug logs via Token.RawToken")
}
