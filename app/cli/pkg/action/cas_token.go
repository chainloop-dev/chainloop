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

package action

import (
	"context"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// casTokenRefreshMargin is the minimum validity a cached CAS token must have
// left to be reused. It leaves time for the request to reach the CAS.
const casTokenRefreshMargin = 10 * time.Second

// casTokenSource caches the short-lived CAS token and requests a new one from
// the control plane when the cached token is close to expiry. Crafting a
// material (e.g. redacting a big AI coding session) can take longer than the
// token lifetime, so the token must be fresh when the upload starts.
type casTokenSource struct {
	mu    sync.Mutex
	token string
	fetch func(ctx context.Context) (string, error)
}

func newCASTokenSource(initial string, fetch func(ctx context.Context) (string, error)) *casTokenSource {
	return &casTokenSource{token: initial, fetch: fetch}
}

// Token returns the cached token, or a new one if the cached token expires
// within casTokenRefreshMargin
func (s *casTokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tokenValidFor(s.token) > casTokenRefreshMargin {
		return s.token, nil
	}

	token, err := s.fetch(ctx)
	if err != nil {
		return "", err
	}

	s.token = token
	return token, nil
}

// tokenValidFor returns how long the token stays valid. The signature is not
// verified: the CAS verifies it, and the CLI only needs the expiration.
// A token that cannot be parsed or has no expiration counts as expired.
func tokenValidFor(token string) time.Duration {
	claims := &jwt.RegisteredClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil || claims.ExpiresAt == nil {
		return 0
	}

	return time.Until(claims.ExpiresAt.Time)
}
