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

package vault

import (
	"context"
	"encoding/json"
	"github.com/go-kratos/kratos/v2/log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeVaultLogin serves Vault's Kubernetes auth login endpoint at mount and records what it was sent.
func fakeVaultLogin(t *testing.T, mount string, got *map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut && r.Method != http.MethodPost || r.URL.Path != "/v1/auth/"+mount+"/login" {
			http.NotFound(w, r)
			return
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(got))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{"client_token": "vault-token-from-login", "renewable": true, "lease_duration": 3600},
		})
	}))
}

func TestKubernetesLogin(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("service-account-jwt\n"), 0o600))

	for name, tc := range map[string]struct{ mount, wantMount string }{
		"default mount": {"", "kubernetes"},
		"custom mount":  {"k8s-prod", "k8s-prod"},
	} {
		t.Run(name, func(t *testing.T) {
			var got map[string]string
			srv := fakeVaultLogin(t, tc.wantMount, &got)
			defer srv.Close()

			cfg := vault.DefaultConfig()
			cfg.Address = srv.URL
			client, err := vault.NewClient(cfg)
			require.NoError(t, err)

			secret, err := kubernetesLogin(context.Background(), client,
				&KubernetesAuthOpts{Role: "chainloop", MountPath: tc.mount, TokenPath: tokenPath})
			require.NoError(t, err)
			assert.Equal(t, map[string]string{"role": "chainloop", "jwt": "service-account-jwt"}, got)
			assert.Equal(t, "vault-token-from-login", client.Token())
			assert.True(t, secret.Auth.Renewable)
		})
	}
}

func TestKubernetesLogin_Errors(t *testing.T) {
	client, err := vault.NewClient(vault.DefaultConfig())
	require.NoError(t, err)

	_, err = kubernetesLogin(context.Background(), client, &KubernetesAuthOpts{TokenPath: "unused"})
	assert.ErrorContains(t, err, "role is required")

	_, err = kubernetesLogin(context.Background(), client,
		&KubernetesAuthOpts{Role: "r", TokenPath: filepath.Join(t.TempDir(), "missing")})
	assert.ErrorContains(t, err, "reading service account token")
}

func TestNewManager_AuthIsExactlyOne(t *testing.T) {
	for name, opts := range map[string]*NewManagerOpts{
		"neither": {Address: "http://127.0.0.1:1"},
		"both":    {Address: "http://127.0.0.1:1", AuthToken: "t", KubernetesAuth: &KubernetesAuthOpts{Role: "r"}},
	} {
		_, err := NewManager(opts)
		assert.ErrorContains(t, err, "exactly one", name)
	}
}

// fakeVaultRelogin issues non-renewable one-second leases and records every JWT it is sent; any other path fails, so a
// client validation against it fails.
func fakeVaultRelogin(t *testing.T, jwts chan<- string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/kubernetes/login" {
			http.Error(w, "no", http.StatusInternalServerError)
			return
		}
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		jwts <- body["jwt"]
		_ = json.NewEncoder(w).Encode(map[string]any{
			"auth": map[string]any{"client_token": "t", "renewable": false, "lease_duration": 1},
		})
	}))
}

// A LEASE THAT ENDS IS REPLACED BY A NEW LOGIN, which re-reads the token file — the kubelet rotates a projected token —
// and the loop stops when its context ends.
func TestKeepLoggedIn_RelogsInWithTheRotatedToken(t *testing.T) {
	jwts := make(chan string, 10)
	srv := fakeVaultRelogin(t, jwts)
	defer srv.Close()
	tokenPath := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("first"), 0o600))

	cfg := vault.DefaultConfig()
	cfg.Address = srv.URL
	client, err := vault.NewClient(cfg)
	require.NoError(t, err)
	opts := &KubernetesAuthOpts{Role: "r", TokenPath: tokenPath}
	login, err := kubernetesLogin(context.Background(), client, opts)
	require.NoError(t, err)
	assert.Equal(t, "first", <-jwts)

	require.NoError(t, os.WriteFile(tokenPath, []byte("rotated"), 0o600))
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() { keepLoggedIn(ctx, client, opts, login, log.NewHelper(log.DefaultLogger)); close(stopped) }()

	select {
	case got := <-jwts:
		assert.Equal(t, "rotated", got, "the re-login read the token file again")
	case <-time.After(5 * time.Second):
		t.Fatal("no re-login after the lease ended")
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("keepLoggedIn did not stop when its context ended")
	}
}

// A MANAGER THAT FAILS VALIDATION LEAVES NOTHING RUNNING: the only login is the initial one.
func TestNewManager_FailedValidationStartsNoRenewer(t *testing.T) {
	jwts := make(chan string, 10)
	srv := fakeVaultRelogin(t, jwts)
	defer srv.Close()
	tokenPath := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenPath, []byte("sa"), 0o600))

	_, err := NewManager(&NewManagerOpts{Address: srv.URL, KubernetesAuth: &KubernetesAuthOpts{Role: "r", TokenPath: tokenPath}})
	require.Error(t, err)
	assert.Len(t, jwts, 1)
	time.Sleep(2 * time.Second) // two lease lengths
	assert.Len(t, jwts, 1, "no re-login: no renewer was started")
}
