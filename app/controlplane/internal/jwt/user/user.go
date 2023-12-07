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
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const Audience = "user-auth.chainloop"

type Builder struct {
	issuer     string
	hmacSecret string
	expiration time.Duration
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

func WithExpiration(d time.Duration) NewOpt {
	return func(b *Builder) {
		b.expiration = d
	}
}

var defaultExpiration = 24 * time.Hour
var SigningMethod = jwt.SigningMethodHS256

func NewBuilder(opts ...NewOpt) (*Builder, error) {
	b := &Builder{
		expiration: defaultExpiration,
	}

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

func (ra *Builder) GenerateJWT(userID string) (string, error) {
	claims := CustomClaims{
		userID,
		jwt.RegisteredClaims{
			Issuer:    ra.issuer,
			Audience:  jwt.ClaimStrings{Audience},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ra.expiration)),
		},
	}

	resultToken := jwt.NewWithClaims(SigningMethod, claims)
	return resultToken.SignedString([]byte(ra.hmacSecret))
}

type KeyFunc func(token *jwt.Token) (interface{}, error)

type CustomClaims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}
