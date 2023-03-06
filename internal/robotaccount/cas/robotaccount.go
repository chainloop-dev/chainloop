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
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

var SigningMethod = jwt.SigningMethodES512

type Builder struct {
	pk         *ecdsa.PrivateKey
	issuer     string
	expiration *time.Duration
}

type Claims struct {
	jwt.RegisteredClaims
	Role           Role   `json:"role"`      // either downloader or uploader
	StoredSecretID string `json:"secret-id"` // path to the OCI secret in the vault
}

type Role string

const (
	Downloader Role = "downloader"
	Uploader   Role = "uploader"
)

type NewOpt func(b *Builder) error

func WithIssuer(issuer string) NewOpt {
	return func(b *Builder) error {
		b.issuer = issuer
		return nil
	}
}

func WithExpiration(d time.Duration) NewOpt {
	return func(b *Builder) error {
		if d == 0 {
			return errors.New("expiration needs to be set")
		}

		b.expiration = &d
		return nil
	}
}

func WithPrivateKey(path string) NewOpt {
	return func(b *Builder) error {
		rawKey, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		ecdsaKey, err := jwt.ParseECPrivateKeyFromPEM(rawKey)
		if err != nil {
			return fmt.Errorf("unable to parse ECDSA private key: %w", err)
		}

		b.pk = ecdsaKey
		return nil
	}
}

func NewBuilder(opts ...NewOpt) (*Builder, error) {
	b := &Builder{}
	for _, opt := range opts {
		if err := opt(b); err != nil {
			return nil, err
		}
	}

	if b.issuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}

	if b.pk == nil {
		return nil, fmt.Errorf("private key is required")
	}

	return b, nil
}

func (ra *Builder) GenerateJWT(secretID, audience string, role Role) (string, error) {
	claims := &Claims{
		Role:           role,
		StoredSecretID: secretID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   ra.issuer,
			Audience: jwt.ClaimStrings{audience},
		},
	}

	if ra.expiration != nil {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(*ra.expiration))
	}

	resultToken := jwt.NewWithClaims(SigningMethod, claims)
	return resultToken.SignedString(ra.pk)
}

const JWTAudience = "artifact-cas.chainloop"

// Additional validation checks
func (c *Claims) Valid() error {
	// Default validation checks
	// expiration, not before, issuer, subject
	if err := c.RegisteredClaims.Valid(); err != nil {
		return err
	}

	if valid := c.VerifyAudience(JWTAudience, true); !valid {
		return jwt.NewValidationError("invalid audience", jwt.ValidationErrorAudience)
	}

	return nil
}

func (c *Claims) CheckRole(r Role) error {
	if r != Downloader && r != Uploader {
		return errors.New("invalid role")
	}

	if c.Role != r {
		return fmt.Errorf("invalid role, got=%s, want=%s", c.Role, r)
	}

	return nil
}
