//
// Copyright 2024-2025 The Chainloop Authors.
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

package apitoken

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
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
	testCases := []struct {
		name    string
		opts    *GenerateJWTOptions
		wantErr bool
	}{
		{
			name: "no project",
			opts: &GenerateJWTOptions{
				OrgID:     toPtr(uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")),
				OrgName:   toPtr("org-name"),
				KeyName:   "key-name",
				KeyID:     uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				ExpiresAt: toPtr(time.Now().Add(1 * time.Hour)),
			},
		},
		{
			name: "no expiration",
			opts: &GenerateJWTOptions{
				OrgID:   toPtr(uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")),
				OrgName: toPtr("org-name"),
				KeyName: "key-name",
				KeyID:   uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
			},
		},
		{
			name: "with project",
			opts: &GenerateJWTOptions{
				OrgID:       toPtr(uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")),
				OrgName:     toPtr("org-name"),
				KeyName:     "key-name",
				KeyID:       uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				ProjectID:   toPtr(uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")),
				ProjectName: toPtr("project-name"),
				ExpiresAt:   toPtr(time.Now().Add(1 * time.Hour)),
			},
		},
		{
			name: "instance token - no orgID or orgName",
			opts: &GenerateJWTOptions{
				KeyName:   "key-name",
				KeyID:     uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				ExpiresAt: toPtr(time.Now().Add(1 * time.Hour)),
				Scope:     toPtr("INSTANCE_ADMIN"),
			},
		},
		{
			name: "missing keyID",
			opts: &GenerateJWTOptions{
				OrgID:     toPtr(uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")),
				OrgName:   toPtr("org-name"),
				KeyName:   "key-name",
				ExpiresAt: toPtr(time.Now().Add(1 * time.Hour)),
			},
			wantErr: true,
		},
		{
			name: "missing keyName",
			opts: &GenerateJWTOptions{
				OrgID:     toPtr(uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")),
				OrgName:   toPtr("org-name"),
				KeyID:     uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				ExpiresAt: toPtr(time.Now().Add(1 * time.Hour)),
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := NewBuilder(WithIssuer("my-issuer"), WithKeySecret(hmacSecret))
			require.NoError(t, err)

			token, err := b.GenerateJWT(tc.opts)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, token)

			claims := &CustomClaims{}
			tokenInfo, err := jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (interface{}, error) {
				return []byte(hmacSecret), nil
			})

			require.NoError(t, err)
			assert.True(t, tokenInfo.Valid)

			if tc.opts.OrgID != nil {
				assert.Equal(t, tc.opts.OrgID.String(), claims.OrgID)
			} else {
				assert.Empty(t, claims.OrgID)
			}
			if tc.opts.OrgName != nil {
				assert.Equal(t, *tc.opts.OrgName, claims.OrgName)
			} else {
				assert.Empty(t, claims.OrgName)
			}

			assert.Equal(t, tc.opts.KeyID.String(), claims.ID)
			assert.Equal(t, tc.opts.KeyName, claims.KeyName)

			if tc.opts.ProjectID != nil {
				assert.Equal(t, tc.opts.ProjectID.String(), claims.ProjectID)
				assert.Equal(t, *tc.opts.ProjectName, claims.ProjectName)
			} else {
				assert.Empty(t, claims.ProjectID)
				assert.Empty(t, claims.ProjectName)
			}

			if tc.opts.Scope != nil {
				assert.Equal(t, *tc.opts.Scope, claims.Scope)
			} else {
				assert.Empty(t, claims.Scope)
			}

			if tc.opts.ExpiresAt != nil {
				assert.True(t, claims.ExpiresAt.After(time.Now()))
			} else {
				assert.Nil(t, claims.ExpiresAt)
			}
		})
	}
}

func toPtr[T any](t T) *T {
	return &t
}
