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

package user

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuilder(t *testing.T) {
	testCases := []struct {
		name             string
		issuer           string
		encryptionString string
		wantError        bool
	}{
		{"valid", "issuer", "my-key", false},
		{"invalid passphrase", "issuer", "", true},
		{"missing issuer", "", "passphrase", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			opts := make([]NewOpt, 0)
			if tc.issuer != "" {
				opts = append(opts, WithIssuer(tc.issuer))
			}

			if tc.encryptionString != "" {
				opts = append(opts, WithKeySecret(tc.encryptionString))
			}

			b, err := NewBuilder(opts...)
			if tc.wantError {
				assert.Error(err)
				return
			}

			assert.NoError(err)

			if tc.issuer != "" {
				assert.Equal(tc.issuer, b.issuer)
			}

			if tc.encryptionString != "" {
				assert.Equal(b.hmacSecret, tc.encryptionString)
			}
		})
	}
}

func TestGenerateJWT(t *testing.T) {
	const hmacSecret = "my-secret"

	b, err := NewBuilder(
		WithIssuer("my-issuer"),
		WithKeySecret(hmacSecret),
		WithExpiration(10*time.Second),
	)
	require.NoError(t, err)

	token, err := b.GenerateJWT("user-id")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify signature and check claims
	claims := &CustomClaims{}
	tokenInfo, err := jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (interface{}, error) {
		return []byte(hmacSecret), nil
	})

	require.NoError(t, err)
	assert.True(t, tokenInfo.Valid)
	assert.Equal(t, "user-id", claims.UserID)
	assert.Equal(t, "my-issuer", claims.Issuer)
	assert.Contains(t, claims.Audience, Audience)
	assert.WithinDuration(t, time.Now(), claims.ExpiresAt.Time, 10*time.Second)
}
