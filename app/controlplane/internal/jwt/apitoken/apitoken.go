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

package apitoken

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var SigningMethod = jwt.SigningMethodHS256

const Audience = "api-token-auth.chainloop"

type Builder struct {
	issuer     string
	hmacSecret string
}

type NewOpt func(b *Builder)

func WithIssuer(issuer string) NewOpt {
	return func(b *Builder) {
		b.issuer = issuer
	}
}

func WithKeySecret(hmacSecret string) NewOpt {
	return func(b *Builder) {
		b.hmacSecret = hmacSecret
	}
}

// NewBuilder creates a new APIToken JWT builder
// It supports expiration and revocation
// Currently we use a simple hmac encryption method meant to be continuously rotated
// TODO: additional/alternative encryption method, i.e DSE asymmetric, see CAS robot account for reference
func NewBuilder(opts ...NewOpt) (*Builder, error) {
	b := &Builder{}
	for _, opt := range opts {
		opt(b)
	}

	if b.issuer == "" {
		return nil, errors.New("issuer is required")
	}

	if b.hmacSecret == "" {
		return nil, errors.New("hmac secret is required")
	}

	return b, nil
}

// it can both expire and being revoked, revocation is performed by checking the keyID against our revocation list
func (ra *Builder) GenerateJWT(keyID string, expiresAt *time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		// Key identifier so we can check it's revocation status
		ID:       keyID,
		Issuer:   ra.issuer,
		Audience: jwt.ClaimStrings{Audience},
	}

	// optional expiration value, i.e 30 days
	if expiresAt != nil {
		claims.ExpiresAt = jwt.NewNumericDate(*expiresAt)
	}

	resultToken := jwt.NewWithClaims(SigningMethod, claims)
	return resultToken.SignedString([]byte(ra.hmacSecret))
}
