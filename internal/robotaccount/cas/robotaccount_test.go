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

package robotaccount

import (
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckRole tests the CheckRole method
func TestCheckRole(t *testing.T) {
	tests := []struct {
		name      string
		gotRole   Role
		wantRole  Role
		wantError bool
	}{
		{"downloader", Downloader, Downloader, false},
		{"uploader", Uploader, Uploader, false},
		{"invalid", Downloader, "invalid", true},
		{"does not match", Uploader, Downloader, true},
		{"does not match", Downloader, Uploader, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			c := &Claims{
				Role: tc.gotRole,
			}

			err := c.CheckRole(tc.wantRole)
			if tc.wantError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

// TestValid tests the Valid method
func TestValid(t *testing.T) {
	tests := []struct {
		name      string
		gotAud    string
		wantAud   string
		wantError bool
	}{
		{"valid", JWTAudience, JWTAudience, false},
		{"invalid", "invalid", JWTAudience, true},
		{"empty", "", JWTAudience, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			c := &Claims{
				RegisteredClaims: jwt.RegisteredClaims{
					Audience: []string{tc.gotAud},
				},
			}

			err := c.Valid()
			if tc.wantError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestNewBuilder(t *testing.T) {
	testCases := []struct {
		name           string
		issuer         string
		privateKeypath string
		expiration     time.Duration
		wantError      bool
	}{
		{"valid", "issuer", "testdata/test-key.ec.pem", 5 * time.Minute, false},
		{"invalid path", "issuer", "testdata/nonexisting.key", 0, true},
		{"invalid key type", "issuer", "testdata/test-key.rsa.pem", 0, true},
		{"missing issuer", "", "testdata/test-key.ec.pem", 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			opts := make([]NewOpt, 0)
			if tc.issuer != "" {
				opts = append(opts, WithIssuer(tc.issuer))
			}

			if tc.privateKeypath != "" {
				opts = append(opts, WithPrivateKey(tc.privateKeypath))
			}

			if tc.expiration != 0 {
				opts = append(opts, WithExpiration(tc.expiration))
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

			if tc.privateKeypath != "" {
				assert.NotNil(b.pk)
			}

			if tc.expiration != 0 {
				assert.Equal(tc.expiration, *b.expiration)
			}
		})
	}
}

func TestGenerateJWT(t *testing.T) {
	b, err := NewBuilder(
		WithIssuer("my-issuer"),
		WithPrivateKey("testdata/test-key.ec.pem"),
		WithExpiration(5*time.Second),
	)

	require.NoError(t, err)
	token, err := b.GenerateJWT("OCI", "secret-id", JWTAudience, Uploader, 123)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify signature and check claims
	rawKey, err := os.ReadFile("testdata/test-key.ec.pub")
	require.NoError(t, err)

	claims := &Claims{}
	tokenInfo, err := jwt.ParseWithClaims(token, claims, loadPublicKey(rawKey))
	require.NoError(t, err)
	assert.True(t, tokenInfo.Valid)
	assert.Equal(t, "secret-id", claims.StoredSecretID)
	assert.Equal(t, Uploader, claims.Role)
	assert.Equal(t, "my-issuer", claims.Issuer)
	assert.Contains(t, claims.Audience, "artifact-cas.chainloop")
	assert.Equal(t, claims.MaxBytes, int64(123))
	assert.WithinDuration(t, time.Now(), claims.ExpiresAt.Time, 10*time.Second)
}

// load key for verification
func loadPublicKey(rawKey []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		return jwt.ParseECPublicKeyFromPEM(rawKey)
	}
}
