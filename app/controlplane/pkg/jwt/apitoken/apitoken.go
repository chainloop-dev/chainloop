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

package apitoken

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/golang-jwt/jwt/v5"
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
	OrgID        *uuid.UUID
	OrgName      *string
	KeyID        uuid.UUID
	KeyName      string
	ProjectID    *uuid.UUID
	ProjectName  *string
	WorkflowID   *uuid.UUID
	WorkflowName *string
	ExpiresAt    *time.Time
	// Scope and ScopeID name the token's scope, as its row records it. Scope is required.
	// ScopeID is unset only for an instance token.
	Scope   *authz.ResourceType
	ScopeID *uuid.UUID
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

	if opts.ProjectID != nil {
		claims.ProjectID = opts.ProjectID.String()
		claims.ProjectName = *opts.ProjectName
	}

	if opts.WorkflowID != nil {
		if opts.WorkflowName == nil {
			return "", errors.New("workflowName is required when workflowID is set")
		}
		claims.WorkflowID = opts.WorkflowID.String()
		claims.WorkflowName = *opts.WorkflowName
	}

	if opts.Scope == nil || *opts.Scope == "" {
		return "", errors.New("scope is required")
	}

	claims.Scope = string(*opts.Scope)
	if opts.ScopeID != nil {
		claims.ScopeID = opts.ScopeID.String()
	}

	// The older instance-admin value would read as a token minted before the scope was signed
	if !claims.HasScopeClaims() {
		return "", fmt.Errorf("invalid scope %q", claims.Scope)
	}

	// Never sign a token whose claims contradict themselves
	if _, _, err := claims.GetScope(); err != nil {
		return "", fmt.Errorf("inconsistent token scope: %w", err)
	}

	// optional expiration value, i.e 30 days
	if opts.ExpiresAt != nil {
		claims.ExpiresAt = jwt.NewNumericDate(*opts.ExpiresAt)
	}

	resultToken := jwt.NewWithClaims(SigningMethod, claims)
	return resultToken.SignedString([]byte(ra.hmacSecret))
}

type CustomClaims struct {
	OrgID        string `json:"org_id"`
	OrgName      string `json:"org_name"`
	KeyName      string `json:"token_name"`
	ProjectID    string `json:"project_id,omitempty"`
	ProjectName  string `json:"project_name,omitempty"`
	WorkflowID   string `json:"workflow_id,omitempty"`
	WorkflowName string `json:"workflow_name,omitempty"`
	// Scope and ScopeID say what the token was granted. Scope is the kind: instance, organization,
	// project or product. ScopeID names the resource, and an instance scope names none.
	//
	// A token minted before the control plane signed its scope has no scope claim. An older
	// instance token has the value "INSTANCE_ADMIN" in it instead. GetScope derives the scope of
	// such a token from the claims that it does carry.
	Scope   string `json:"scope,omitempty"`
	ScopeID string `json:"scope_id,omitempty"`
	jwt.RegisteredClaims
}

// HasScopeClaims reports whether the token was signed with its scope kind in the scope claim. A
// token without it was minted before the control plane signed the scope.
func (c *CustomClaims) HasScopeClaims() bool {
	return c.Scope != "" && c.Scope != authz.ScopeInstanceAdmin
}

// GetScope returns the scope that the claims bind the token to. For a token with the scope and
// scope_id claims, that scope is what they name. For an older token, it is the scope that its
// other claims imply (see legacyScope). An older product token therefore gets its organization as
// its scope. GetScope returns an error for claims that contradict themselves.
func (c *CustomClaims) GetScope() (authz.ResourceType, *uuid.UUID, error) {
	kind, id, err := c.namedScope()
	if err != nil {
		return "", nil, err
	}

	if err := c.agreesWith(kind, id); err != nil {
		return "", nil, err
	}

	return kind, id, nil
}

// namedScope is the scope the scope and scope_id claims name, else the one the claims of an older
// token imply.
func (c *CustomClaims) namedScope() (authz.ResourceType, *uuid.UUID, error) {
	if !c.HasScopeClaims() {
		return c.legacyScope()
	}

	kind := authz.ResourceType(c.Scope)
	switch kind {
	case authz.ResourceTypeInstance:
		if c.ScopeID != "" {
			return "", nil, errors.New("an instance scope names no resource")
		}

		return kind, nil, nil
	case authz.ResourceTypeOrganization, authz.ResourceTypeProject, authz.ResourceTypeProduct:
		id, err := uuid.Parse(c.ScopeID)
		if err != nil {
			return "", nil, fmt.Errorf("invalid scope_id claim: %w", err)
		}

		return kind, &id, nil
	default:
		return "", nil, fmt.Errorf("unknown scope claim %q", c.Scope)
	}
}

// legacyScope returns the scope that the claims of an older token imply. An older token is a token
// minted before the control plane signed the scope. Its scope is the instance for the
// instance-admin value, else its project, else its organization. biz.newTokenScope and the scope backfill
// migration apply the same rule to the row, so the two agree.
func (c *CustomClaims) legacyScope() (authz.ResourceType, *uuid.UUID, error) {
	var kind authz.ResourceType
	var raw string
	switch {
	case c.Scope == authz.ScopeInstanceAdmin:
		return authz.ResourceTypeInstance, nil, nil
	case c.ProjectID != "":
		kind, raw = authz.ResourceTypeProject, c.ProjectID
	case c.OrgID != "":
		kind, raw = authz.ResourceTypeOrganization, c.OrgID
	default:
		return "", nil, errors.New("the claims name no scope")
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		return "", nil, fmt.Errorf("invalid %s claim: %w", kind, err)
	}

	return kind, &id, nil
}

// agreesWith checks that the other claims fit the scope. This makes the control plane and the
// platform read the same scope from the token. The rules are:
//   - An instance token names no organization and no project.
//   - An organization token names its own organization and no project.
//   - A project token names its organization and the project of its scope.
//   - A product token names its organization and no project.
//   - A workflow claim always comes with a project claim.
func (c *CustomClaims) agreesWith(kind authz.ResourceType, id *uuid.UUID) error {
	if c.WorkflowID != "" && c.ProjectID == "" {
		return errors.New("a workflow claim needs a project claim")
	}

	switch kind {
	case authz.ResourceTypeInstance:
		if c.OrgID != "" || c.ProjectID != "" {
			return errors.New("an instance scope names no organization or project")
		}
	case authz.ResourceTypeOrganization:
		if c.OrgID != id.String() || c.ProjectID != "" {
			return errors.New("an organization scope names its own organization and no project")
		}
	case authz.ResourceTypeProject:
		if c.OrgID == "" || c.ProjectID != id.String() {
			return errors.New("a project scope names its organization and its own project")
		}
	case authz.ResourceTypeProduct:
		if c.OrgID == "" || c.ProjectID != "" {
			return errors.New("a product scope names its organization and no project")
		}
	}

	return nil
}

// ClaimsFromMap reads API-token claims that were parsed generically, as the API entry point
// receives them. A claim of the wrong type is an error, not an absent claim.
func ClaimsFromMap(m jwt.MapClaims) (*CustomClaims, error) {
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encoding claims: %w", err)
	}

	claims := &CustomClaims{}
	if err := json.Unmarshal(raw, claims); err != nil {
		return nil, fmt.Errorf("decoding claims: %w", err)
	}

	return claims, nil
}
