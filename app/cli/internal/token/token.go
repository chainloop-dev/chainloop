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
	apiTokenAudience       = "api-token-auth.chainloop"
	federatedTokenAudience = "chainloop"
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
		if isGitLabFederatedToken(claims) {
			pToken.TokenType = v1.Attestation_Auth_AUTH_TYPE_FEDERATED
			if issuer, ok := claims["iss"].(string); ok {
				pToken.ID = issuer
			}
		}
	default:
		return nil, nil
	}

	return pToken, nil
}

// Checks if the claims contain at least 10 custom GitLab ID token claims.
// Reference: https://docs.gitlab.com/ci/secrets/id_token_authentication/
func isGitLabFederatedToken(claims jwt.MapClaims) bool {
	gitlabClaims := []string{
		"namespace_id",
		"namespace_path",
		"project_id",
		"project_path",
		"user_id",
		"user_login",
		"user_email",
		"user_access_level",
		"user_identities",
		"pipeline_id",
		"pipeline_source",
		"job_id",
		"ref",
		"ref_type",
		"ref_path",
		"ref_protected",
		"groups_direct",
		"environment",
		"environment_protected",
		"deployment_tier",
		"deployment_action",
		"runner_id",
		"runner_environment",
		"sha",
		"ci_config_ref_uri",
		"ci_config_sha",
		"project_visibility",
	}

	requiredClaims := 10

	// Count how many GitLab-specific claims are present
	found := 0
	for _, claim := range gitlabClaims {
		if _, exists := claims[claim]; exists {
			found++
			if found >= requiredClaims {
				return true
			}
		}
	}

	return found >= requiredClaims
}
