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
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signedTestToken(t *testing.T, expiresIn time.Duration) string {
	t.Helper()
	claims := jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn))}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("test"))
	require.NoError(t, err)
	return token
}

func TestCASTokenSource(t *testing.T) {
	fresh := signedTestToken(t, time.Minute)

	testCases := []struct {
		name        string
		initial     string
		fetchErr    error
		wantToken   string
		wantFetches int
		wantErr     bool
	}{
		{
			name:        "valid token is reused",
			initial:     signedTestToken(t, time.Minute),
			wantFetches: 0,
		},
		{
			name:        "expired token is refreshed",
			initial:     signedTestToken(t, -time.Second),
			wantToken:   fresh,
			wantFetches: 1,
		},
		{
			name:        "token close to expiry is refreshed",
			initial:     signedTestToken(t, casTokenRefreshMargin/2),
			wantToken:   fresh,
			wantFetches: 1,
		},
		{
			name:        "unparseable token is refreshed",
			initial:     "not-a-jwt",
			wantToken:   fresh,
			wantFetches: 1,
		},
		{
			name:        "refresh error is returned",
			initial:     signedTestToken(t, -time.Second),
			fetchErr:    errors.New("boom"),
			wantFetches: 1,
			wantErr:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fetches := 0
			src := newCASTokenSource(tc.initial, func(context.Context) (string, error) {
				fetches++
				if tc.fetchErr != nil {
					return "", tc.fetchErr
				}
				return fresh, nil
			})

			got, err := src.Token(context.Background())
			assert.Equal(t, tc.wantFetches, fetches)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			want := tc.wantToken
			if want == "" {
				want = tc.initial
			}
			assert.Equal(t, want, got)

			// A second call reuses the token that is now cached.
			_, err = src.Token(context.Background())
			require.NoError(t, err)
			assert.Equal(t, tc.wantFetches, fetches)
		})
	}
}
