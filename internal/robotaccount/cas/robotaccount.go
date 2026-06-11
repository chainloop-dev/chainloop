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
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	kratosjwt "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
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
	// Managed providers (e.g. AWS-S3-ACCESS-POINT) require it to scope
	// per-tenant STS sessions; the non-managed providers ignore it but
	// it is still carried for audit traceability.
	OrgID string `json:"org-id"`
	// SourceInternal is true when the token was minted for the control plane's
	// own CAS client (e.g. attestation storage, policy material reads).
	// The CAS skips audit event emission for this traffic so it doesn't
	// pollute per-org usage numbers. The zero value (false) means client traffic.
	SourceInternal bool `json:"source-internal,omitempty"`
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

// GenerateOpt tweaks optional claims of the minted token
type GenerateOpt func(c *Claims)

// WithSourceInternal flags the token as minted for the control plane's own
// CAS client, so the CAS can tell internal traffic apart from client traffic
func WithSourceInternal() GenerateOpt {
	return func(c *Claims) {
		c.SourceInternal = true
	}
}

// GenerateJWT mints a CAS token. All fields are required, including
// orgID — managed providers (e.g. AWS-S3-ACCESS-POINT) need it to scope
// per-tenant STS sessions and other providers still record it for
// audit. The token always carries the CAS audience and a short expiry
// window.
func (ra *Builder) GenerateJWT(backendType, secretID, audience string, role Role, maxBytes int64, orgID string, opts ...GenerateOpt) (string, error) {
	if backendType == "" {
		return "", fmt.Errorf("backend type is required")
	}

	if secretID == "" {
		return "", fmt.Errorf("secret id is required")
	}

	if audience == "" {
		return "", fmt.Errorf("audience is required")
	}

	if orgID == "" {
		return "", fmt.Errorf("org id is required")
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

	for _, opt := range opts {
		opt(claims)
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

// InfoFromAuth extracts the JWT claims from the context, note that the JWT verification has happened in the middleware
func InfoFromAuth(ctx context.Context) (*Claims, error) {
	rawClaims, ok := kratosjwt.FromContext(ctx)
	if !ok {
		return nil, kerrors.Unauthorized("cas", "missing authentication information")
	}

	claims, ok := rawClaims.(*Claims)
	if !ok {
		return nil, kerrors.Unauthorized("cas", "invalid authentication information")
	}

	if claims.StoredSecretID == "" {
		return nil, kerrors.Unauthorized("cas", "missing secret reference")
	}

	if claims.BackendType == "" {
		return nil, kerrors.Unauthorized("cas", "missing backend type")
	}

	if claims.Role != Uploader && claims.Role != Downloader {
		return nil, kerrors.Unauthorized("cas", "invalid role")
	}

	return claims, nil
}
