//
// Copyright 2024-2026 The Chainloop Authors.
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
	"strconv"

	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/golang-jwt/jwt/v5"
)

const (
	UserAudience = "user-auth.chainloop"
	APIAudience  = "api-token-auth.chainloop"
)

type ParsedToken struct {
	ID        string
	OrgID     string
	TokenType v1.Attestation_Auth_AuthType
	// CINamespaceID identifies the CI namespace a federated token was minted for: the
	// GitHub organization that owns the repository, or the GitLab namespace that owns
	// the project. Empty for every other token type, and for a provider that emits
	// neither claim. It exists for telemetry and must stay out of the attestation auth
	// metadata, which is keyed on ID.
	CINamespaceID string
}

const (
	userAudience = "user-auth.chainloop"
	//nolint:gosec
	apiTokenAudience       = "api-token-auth.chainloop"
	federatedTokenAudience = "chainloop"

	// githubOwnerIDClaim is GitHub Actions' ID of the organization owning the repository
	// the workflow runs from.
	githubOwnerIDClaim = "repository_owner_id"
	// gitlabNamespaceIDClaim is GitLab's ID of the namespace owning the project the job
	// runs from.
	gitlabNamespaceIDClaim = "namespace_id"
)

// Parse the token and return the type of token. At the moment in Chainloop we have 3 types of tokens:
// 1. User account token
// 2. API token
// 3. Federated token
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

	// Supports both string and array formats per JWT RFC 7519
	// Takes first array element when multiple audiences exist
	var audience string
	switch aud := claims["aud"].(type) {
	case string:
		audience = aud
	case []interface{}:
		if len(aud) > 0 {
			audience, _ = aud[0].(string)
		}
	default:
		return nil, nil
	}

	if audience == "" {
		return nil, nil
	}

	pToken := &ParsedToken{}

	// Determines token type and id based on audience:
	// 1. API Tokens:
	//    - Type: AUTH_TYPE_API_TOKEN
	//    - ID: 'jti' claim (JWT ID)
	//    - OrgID: 'org_id' claim
	// 2. User Tokens:
	//    - Type: AUTH_TYPE_USER
	//    - ID: 'user_id' claim
	// 3. Federated Tokens:
	//    - Type: AUTH_TYPE_FEDERATED
	//    - ID: 'iss' claim (issuer URL)
	switch audience {
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
	case federatedTokenAudience:
		pToken.TokenType = v1.Attestation_Auth_AUTH_TYPE_FEDERATED
		if issuer, ok := claims["iss"].(string); ok {
			pToken.ID = issuer
		} else {
			return nil, nil
		}
		pToken.CINamespaceID = ciNamespaceID(claims)
	default:
		return nil, nil
	}

	// Avoid returning partially filled tokens that would create invalid auth metadata
	// in crafting state (e.g. type=UNSPECIFIED or empty ID).
	if pToken.TokenType == v1.Attestation_Auth_AUTH_TYPE_UNSPECIFIED || pToken.ID == "" {
		return nil, nil
	}

	return pToken, nil
}

// ciNamespaceID returns the provider's identifier for the namespace that owns the
// repository a federated token was minted for. Both providers expose it as a numeric id
// that survives a rename, unlike the matching name claims, and both document it as a
// string, so a JSON number is accepted too rather than silently dropping the namespace.
// Returns an empty string when the provider emits neither claim.
func ciNamespaceID(claims jwt.MapClaims) string {
	for _, claim := range []string{githubOwnerIDClaim, gitlabNamespaceIDClaim} {
		switch v := claims[claim].(type) {
		case string:
			if v != "" {
				return v
			}
		case float64:
			return strconv.FormatInt(int64(v), 10)
		}
	}

	return ""
}
