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
	"errors"
	"fmt"
	"strings"
)

// Source says how a credential was obtained. Both adapters end at the same
// kind of value, a normal OrcaRouter API key, so nothing downstream branches on
// it beyond the status text.
type Source string

const (
	// SourceAPIKey is a key the user pasted from the console.
	SourceAPIKey Source = "api_key"
	// SourcePKCE is a key minted by an OAuth 2.0 + PKCE login.
	SourcePKCE Source = "pkce"
)

// Credential is one OrcaRouter API key plus what the exchange said about it.
// It is the only shape the provider, the request path, and model discovery
// accept, whatever adapter produced it.
type Credential struct {
	// Key is the API key. It is never logged, printed, or put in a URL.
	Key string
	// Source records which adapter produced it.
	Source Source
	// UserID is the account the exchange reported, when it did.
	UserID string
	// Scope is the scope that was granted, as read back from the response,
	// not the scope that was requested.
	Scope string
	// Generation increases on every successful replacement. A late failure
	// from an older generation must not mark a newer credential unusable.
	Generation int
}

// Redacted returns the credential with the key masked, for status output.
func (c Credential) Redacted() Credential {
	c.Key = MaskKey(c.Key)
	return c
}

// MaskKey shows enough of a key to recognise it and no more. It never panics on
// a short or empty value.
func MaskKey(key string) string {
	const keep = 4
	if key == "" {
		return ""
	}
	if len(key) <= keep*2 {
		return strings.Repeat("*", len(key))
	}
	return key[:keep] + strings.Repeat("*", len(key)-keep*2) + key[len(key)-keep:]
}

// ErrInvalidKey is returned by the API-key adapter for a value that cannot be
// an OrcaRouter key.
var ErrInvalidKey = errors.New("not an OrcaRouter API key")

// APIKeyAdapter turns a pasted key into a Credential. It checks the documented
// prefix and that the value is not empty. It does not call the service: the
// prefix is a format check, not proof that the credential is valid, and there
// is no free validation endpoint.
type APIKeyAdapter struct{}

// Acquire validates a pasted key. The key travels in memory only.
func (APIKeyAdapter) Acquire(key string) (Credential, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return Credential{}, fmt.Errorf("%w: the key is empty", ErrInvalidKey)
	}
	if !strings.HasPrefix(trimmed, KeyPrefix) || len(trimmed) == len(KeyPrefix) {
		return Credential{}, fmt.Errorf("%w: the key must start with %q", ErrInvalidKey, KeyPrefix)
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return Credential{}, fmt.Errorf("%w: the key contains whitespace", ErrInvalidKey)
	}
	return Credential{Key: trimmed, Source: SourceAPIKey}, nil
}
