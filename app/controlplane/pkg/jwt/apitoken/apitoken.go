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
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
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

type GenerateJWTOptions struct {
	OrgID       *uuid.UUID
	OrgName     *string
	KeyID       uuid.UUID
	KeyName     string
	ProjectID   *uuid.UUID
	ProjectName *string
	ExpiresAt   *time.Time
	Scope       *string
}

// GenerateJWT creates a new JWT token for the given organization and keyID
func (ra *Builder) GenerateJWT(opts *GenerateJWTOptions) (string, error) {
	if opts == nil {
		return "", errors.New("options are required")
	}

	if opts.KeyID == uuid.Nil {
		return "", errors.New("keyID is required")
	}

	if opts.KeyName == "" {
		return "", errors.New("keyName is required")
	}

	claims := CustomClaims{
		KeyName: opts.KeyName,
		RegisteredClaims: jwt.RegisteredClaims{
			// Key identifier so we can check its revocation status
			ID:       opts.KeyID.String(),
			Issuer:   ra.issuer,
			Audience: jwt.ClaimStrings{Audience},
		},
	}

	if opts.OrgID != nil {
		claims.OrgID = opts.OrgID.String()
		claims.OrgName = *opts.OrgName
	}

	if opts.Scope != nil {
		claims.Scope = *opts.Scope
	}

	if opts.ProjectID != nil {
		claims.ProjectID = opts.ProjectID.String()
		claims.ProjectName = *opts.ProjectName
	}

	// optional expiration value, i.e 30 days
	if opts.ExpiresAt != nil {
		claims.ExpiresAt = jwt.NewNumericDate(*opts.ExpiresAt)
	}

	resultToken := jwt.NewWithClaims(SigningMethod, claims)
	return resultToken.SignedString([]byte(ra.hmacSecret))
}

type CustomClaims struct {
	OrgID       string `json:"org_id"`
	OrgName     string `json:"org_name"`
	KeyName     string `json:"token_name"`
	ProjectID   string `json:"project_id,omitempty"`
	ProjectName string `json:"project_name,omitempty"`
	Scope       string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}
