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

package server

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	robotaccount "github.com/chainloop-dev/bedrock/internal/robotaccount/cas"
	jwtMiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/grpc-ecosystem/go-grpc-middleware/util/metautils"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestJWTAuthFunc(t *testing.T) {
	const defaultPrivateKeyPath = "./testdata/test-key.ec.pem"
	const defaultPublicKeyPath = "./testdata/test-key.ec.pub"

	testCases := []struct {
		name                  string
		audience              string
		publicKeyOverride     string
		expiration            time.Duration
		signingMethodOverride *jwt.SigningMethodECDSA
		// regular error message
		wantRegularErr    string
		wantExpirationErr bool
	}{
		{
			name:     "valid token no expiration",
			audience: robotaccount.JWTAudience,
		},
		{
			name:       "valid token no with expiration",
			audience:   robotaccount.JWTAudience,
			expiration: 10 * time.Minute,
		},
		{
			name:           "invalid audience",
			audience:       "invalid audience",
			wantRegularErr: "invalid audience",
		},
		{
			name:              "expired token",
			audience:          robotaccount.JWTAudience,
			expiration:        -10 * time.Minute,
			wantExpirationErr: true,
		},
		{
			name:     "wrong signing method",
			audience: robotaccount.JWTAudience,
			// This signing method is not the same one it was used during crafting the token
			signingMethodOverride: jwt.SigningMethodES384,
			wantRegularErr:        "Wrong signing method",
		},
		{
			name:     "different public key signing method",
			audience: robotaccount.JWTAudience,
			// This signing method is not the same one it was used during crafting the token
			wantRegularErr:    "verification error",
			publicKeyOverride: "./testdata/test-key-2.ec.pub",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate a token
			opts := []robotaccount.NewOpt{
				robotaccount.WithIssuer("my-issuer"),
				robotaccount.WithPrivateKey(defaultPrivateKeyPath),
			}

			if tc.expiration != 0 {
				opts = append(opts, robotaccount.WithExpiration(tc.expiration))
			}

			b, err := robotaccount.NewBuilder(opts...)
			require.NoError(t, err)
			token, err := b.GenerateJWT("secret-id", tc.audience, robotaccount.Downloader)
			require.NoError(t, err)

			// add bearer token to context
			md := metadata.Pairs("authorization", fmt.Sprintf("bearer %s", token))
			ctx := metautils.NiceMD(md).ToIncoming(context.TODO())

			// Perform a check
			publicKeyPath := defaultPublicKeyPath
			if tc.publicKeyOverride != "" {
				publicKeyPath = tc.publicKeyOverride
			}

			signingMethod := robotaccount.SigningMethod
			if tc.signingMethodOverride != nil {
				signingMethod = tc.signingMethodOverride
			}

			ctx, err = jwtAuthFunc(loadTestPublicKey(publicKeyPath), signingMethod)(ctx)

			switch {
			case tc.wantExpirationErr:
				assert.ErrorAs(t, err, &jwtMiddleware.ErrTokenExpired)
			case tc.wantRegularErr != "":
				assert.ErrorContains(t, err, tc.wantRegularErr)
			default:
				assert.NoError(t, err)
				// Validate and extract the claims
				claims := infoFromAuth(ctx, t)
				assert.NoError(t, claims.Valid())
				assert.Equal(t, "secret-id", claims.StoredSecretID)
				assert.Equal(t, robotaccount.Downloader, claims.Role)
				assert.Equal(t, "my-issuer", claims.Issuer)
				assert.Contains(t, claims.Audience, "artifact-cas.chainloop")
				if tc.expiration != 0 {
					assert.WithinDuration(t, time.Now(), claims.ExpiresAt.Time, tc.expiration)
				}
			}
		})
	}
}

func loadTestPublicKey(path string) jwt.Keyfunc {
	rawKey, _ := os.ReadFile(path)
	return func(token *jwt.Token) (interface{}, error) {
		return jwt.ParseECPublicKeyFromPEM(rawKey)
	}
}

func infoFromAuth(ctx context.Context, t *testing.T) *robotaccount.Claims {
	rawClaims, ok := jwtMiddleware.FromContext(ctx)
	if !ok {
		require.Fail(t, "no claims found in context")
	}

	claims, ok := rawClaims.(*robotaccount.Claims)
	if !ok {
		require.Fail(t, "invalid claims")
	}

	return claims
}
