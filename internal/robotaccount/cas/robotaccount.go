//
// Copyright 2023-2026 The Chainloop Authors.
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
	BackendType    string `json:"backend"`   // backend to use, i.e OCI
	MaxBytes       int64  `json:"maxbytes"`  // max bytes to upload
	// OrgID identifies the authenticated org this token was minted for.
	// Required for managed providers (AWS-S3-ACCESS-POINT) that need to
	// scope per-tenant STS sessions; carried as a separate claim from
	// StoredSecretID so the binding can't be tampered with by rewriting
	// just the secret store. Empty for legacy tokens or providers that
	// don't need per-tenant attribution.
	OrgID string `json:"org-id,omitempty"`
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

// GenerateJWT mints a CAS token. orgID is required for tokens that will
// touch managed providers (e.g. AWS-S3-ACCESS-POINT) and otherwise
// optional — pass "" if the targeted backend doesn't need per-tenant
// attribution. The token always carries the CAS audience and a short
// expiry window.
func (ra *Builder) GenerateJWT(backendType, secretID, audience string, role Role, maxBytes int64, orgID string) (string, error) {
	if backendType == "" {
		return "", fmt.Errorf("backend type is required")
	}

	if secretID == "" {
		return "", fmt.Errorf("secret id is required")
	}

	if audience == "" {
		return "", fmt.Errorf("audience is required")
	}

	if role != Downloader && role != Uploader {
		return "", fmt.Errorf("invalid role")
	}

	claims := &Claims{
		Role: role,
		// Credentials to instantiate the backend
		StoredSecretID: secretID,
		// Identifier for the backend, i.e OCI
		BackendType: backendType,
		OrgID:       orgID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   ra.issuer,
			Audience: jwt.ClaimStrings{audience},
		},
	}

	// If there is limit on the size of the upload we store it as claim
	if maxBytes != 0 {
		claims.MaxBytes = maxBytes
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
