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

package orcarouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatSuccess(t *testing.T) {
	var gotAuth, gotPath, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		var req ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotModel = req.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello there"}}]}`))
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	answer, err := client.Chat(context.Background(), Credential{Key: "sk-orca-test"}, ChatRequest{
		Model:    "orcarouter/auto",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "hello there", answer)
	assert.Equal(t, "/chat/completions", gotPath)
	assert.Equal(t, "Bearer sk-orca-test", gotAuth)
	assert.Equal(t, "orcarouter/auto", gotModel)
}

func TestChatRejectedKeyNeedsReauth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"revoked"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := client.Chat(context.Background(), Credential{Key: "sk-orca-revoked"}, ChatRequest{
		Model:    "orcarouter/auto",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	assert.ErrorIs(t, err, ErrNeedsReauth)
	assert.NotContains(t, err.Error(), "sk-orca-revoked", "the key must never appear in an error")
}

func TestChatRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{}`, http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := client.Chat(context.Background(), Credential{Key: "sk-orca-test"}, ChatRequest{
		Model:    "orcarouter/auto",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429")
}

func TestChatModelNotAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":{"code":"model_access_denied"}}`, http.StatusForbidden)
	}))
	defer srv.Close()

	client := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	_, err := client.Chat(context.Background(), Credential{Key: "sk-orca-test"}, ChatRequest{
		Model:    "orcarouter/auto",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	require.Error(t, err)
	// A forbidden model is not a rejected credential: a new login would not
	// help, so the caller must not mark the key as needing reauth.
	assert.NotErrorIs(t, err, ErrNeedsReauth)
	assert.Contains(t, err.Error(), "orcarouter/auto")
	assert.Contains(t, err.Error(), "403")
	assert.NotContains(t, err.Error(), "sk-orca-test")
}

func TestChatValidation(t *testing.T) {
	client := &Client{}
	_, err := client.Chat(context.Background(), Credential{}, ChatRequest{Model: "m", Messages: []Message{{Role: "user"}}})
	assert.ErrorIs(t, err, ErrNoCredential)

	_, err = client.Chat(context.Background(), Credential{Key: "sk-orca-x"}, ChatRequest{Messages: []Message{{Role: "user"}}})
	require.Error(t, err)

	_, err = client.Chat(context.Background(), Credential{Key: "sk-orca-x"}, ChatRequest{Model: "m"})
	require.Error(t, err)
}
