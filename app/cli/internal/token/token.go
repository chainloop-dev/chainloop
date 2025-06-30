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

package token

import (
	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/golang-jwt/jwt/v4"
)

const (
	UserAudience = "user-auth.chainloop"
	APIAudience  = "api-token-auth.chainloop"
)

type ParsedToken struct {
	ID        string
	OrgID     string
	TokenType v1.Attestation_Auth_AuthType
}

const (
	userAudience = "user-auth.chainloop"
	//nolint:gosec
	apiTokenAudience = "api-token-auth.chainloop"
)

// Parse the token and return the type of token. At the moment in Chainloop we have 3 types of tokens:
// 1. User account token
// 2. API token
// Each one of them have an associated audience claim that we use to identify the type of token. If the token is not
// present, nor we cannot match it with one of the expected audience, return nil.
func Parse(token string) (*ParsedToken, error) {
	if token == "" {
		return nil, nil
	}

	// Create a parser without claims validation
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())

	// Parse the token without verification
	t, _, err := parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	// Extract generic claims otherwise, we would have to parse
	// the token again to get the claims for each type
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil
	}

	// Get the audience claim
	val, ok := claims["aud"]
	if !ok || val == nil {
		return nil, nil
	}

	// Ensure audience is an array of interfaces
	// Chainloop only has one audience per token
	aud, ok := val.([]interface{})
	if !ok || len(aud) == 0 {
		return nil, nil
	}

	// Initialize parsedToken
	pToken := &ParsedToken{}

	// Determine the type of token based on the audience.
	switch aud[0].(string) {
	case apiTokenAudience:
		pToken.TokenType = v1.Attestation_Auth_AUTH_TYPE_API_TOKEN
		if tokenID, ok := claims["jti"].(string); ok {
			pToken.ID = tokenID
		}
		if orgID, ok := claims["org_id"].(string); ok {
			pToken.OrgID = orgID
		}
	case userAudience:
		pToken.TokenType = v1.Attestation_Auth_AUTH_TYPE_USER
		if userID, ok := claims["user_id"].(string); ok {
			pToken.ID = userID
		}
	default:
		return nil, nil
	}

	return pToken, nil
}
