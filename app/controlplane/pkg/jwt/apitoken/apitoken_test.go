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
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBuilder(t *testing.T) {
	testCases := []struct {
		name             string
		issuer           string
		encryptionString string
		wantError        bool
	}{
		{"valid", "issuer", "my-key", false},
		{"invalid passphrase", "issuer", "", true},
		{"missing issuer", "", "passphrase", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			opts := make([]NewOpt, 0)
			if tc.issuer != "" {
				opts = append(opts, WithIssuer(tc.issuer))
			}

			if tc.encryptionString != "" {
				opts = append(opts, WithKeySecret(tc.encryptionString))
			}

			b, err := NewBuilder(opts...)
			if tc.wantError {
				assert.Error(err)
				return
			}

			assert.NoError(err)

			if tc.issuer != "" {
				assert.Equal(tc.issuer, b.issuer)
			}

			if tc.encryptionString != "" {
				assert.Equal(b.hmacSecret, tc.encryptionString)
			}
		})
	}
}

const (
	testKeyName = "key-name"
	claimJTI    = "jti"
)

func TestGenerateJWT(t *testing.T) {
	const hmacSecret = "my-secret"
	org := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	project := uuid.MustParse("223e4567-e89b-12d3-a456-426614174000")
	workflow := uuid.MustParse("323e4567-e89b-12d3-a456-426614174000")
	product := uuid.MustParse("423e4567-e89b-12d3-a456-426614174000")
	keyID := uuid.MustParse("523e4567-e89b-12d3-a456-426614174000")
	orgScope, projectScope := authz.ResourceTypeOrganization, authz.ResourceTypeProject
	productScope, instanceScope := authz.ResourceTypeProduct, authz.ResourceTypeInstance

	testCases := []struct {
		name    string
		opts    *GenerateJWTOptions
		wantErr bool
	}{
		{
			name: "organization token",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID,
				ExpiresAt: toPtr(time.Now().Add(1 * time.Hour)), Scope: &orgScope, ScopeID: &org},
		},
		{
			name: "no expiration",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID,
				Scope: &orgScope, ScopeID: &org},
		},
		{
			name: "project token",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID,
				ProjectID: &project, ProjectName: toPtr("project-name"), ExpiresAt: toPtr(time.Now().Add(1 * time.Hour)),
				Scope: &projectScope, ScopeID: &project},
		},
		{
			name: "workflow-pinned token",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID,
				ProjectID: &project, ProjectName: toPtr("project-name"), WorkflowID: &workflow, WorkflowName: toPtr("workflow-name"),
				ExpiresAt: toPtr(time.Now().Add(1 * time.Hour)), Scope: &projectScope, ScopeID: &project},
		},
		{
			name: "product token",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID,
				Scope: &productScope, ScopeID: &product},
		},
		{
			name: "instance token",
			opts: &GenerateJWTOptions{KeyName: testKeyName, KeyID: keyID, ExpiresAt: toPtr(time.Now().Add(1 * time.Hour)),
				Scope: &instanceScope},
		},
		{
			name: "missing keyID",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName,
				Scope: &orgScope, ScopeID: &org},
			wantErr: true,
		},
		{
			name: "missing keyName",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyID: keyID,
				Scope: &orgScope, ScopeID: &org},
			wantErr: true,
		},
		{
			name:    "a scope id without a scope",
			opts:    &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID, ScopeID: &org},
			wantErr: true,
		},
		{
			name:    "missing scope",
			opts:    &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID},
			wantErr: true,
		},
		{
			// An empty scope would sign a token without the scope claims, like an older token
			name: "an empty scope",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID,
				Scope: toPtr(authz.ResourceType("")), ScopeID: &org},
			wantErr: true,
		},
		{
			name: "an organization scope naming another organization",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID,
				Scope: &orgScope, ScopeID: &product},
			wantErr: true,
		},
		{
			// The older instance-admin value would read as a token minted before the scope was signed
			name:    "the older instance-admin value as the scope",
			opts:    &GenerateJWTOptions{KeyName: testKeyName, KeyID: keyID, Scope: toPtr(authz.ResourceType(authz.ScopeInstanceAdmin))},
			wantErr: true,
		},
		{
			name: "an instance scope naming an organization",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID,
				Scope: &instanceScope},
			wantErr: true,
		},
		{
			name: "a project scope naming another project",
			opts: &GenerateJWTOptions{OrgID: &org, OrgName: toPtr("org-name"), KeyName: testKeyName, KeyID: keyID,
				ProjectID: &project, ProjectName: toPtr("project-name"), Scope: &projectScope, ScopeID: &product},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := NewBuilder(WithIssuer("my-issuer"), WithKeySecret(hmacSecret))
			require.NoError(t, err)

			token, err := b.GenerateJWT(tc.opts)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, token)

			claims := &CustomClaims{}
			tokenInfo, err := jwt.ParseWithClaims(token, claims, func(_ *jwt.Token) (interface{}, error) {
				return []byte(hmacSecret), nil
			})

			require.NoError(t, err)
			assert.True(t, tokenInfo.Valid)

			if tc.opts.OrgID != nil {
				assert.Equal(t, tc.opts.OrgID.String(), claims.OrgID)
			} else {
				assert.Empty(t, claims.OrgID)
			}
			if tc.opts.OrgName != nil {
				assert.Equal(t, *tc.opts.OrgName, claims.OrgName)
			} else {
				assert.Empty(t, claims.OrgName)
			}

			assert.Equal(t, tc.opts.KeyID.String(), claims.ID)
			assert.Equal(t, tc.opts.KeyName, claims.KeyName)

			if tc.opts.ProjectID != nil {
				assert.Equal(t, tc.opts.ProjectID.String(), claims.ProjectID)
				assert.Equal(t, *tc.opts.ProjectName, claims.ProjectName)
			} else {
				assert.Empty(t, claims.ProjectID)
				assert.Empty(t, claims.ProjectName)
			}

			if tc.opts.WorkflowID != nil {
				assert.Equal(t, tc.opts.WorkflowID.String(), claims.WorkflowID)
				assert.Equal(t, *tc.opts.WorkflowName, claims.WorkflowName)
			} else {
				assert.Empty(t, claims.WorkflowID)
				assert.Empty(t, claims.WorkflowName)
			}

			assert.Equal(t, string(*tc.opts.Scope), claims.Scope)
			if tc.opts.ScopeID != nil {
				assert.Equal(t, tc.opts.ScopeID.String(), claims.ScopeID)
			} else {
				assert.Empty(t, claims.ScopeID)
			}

			if tc.opts.ExpiresAt != nil {
				assert.True(t, claims.ExpiresAt.After(time.Now()))
			} else {
				assert.Nil(t, claims.ExpiresAt)
			}
		})
	}
}

func TestGetScope(t *testing.T) {
	org, project, product, other := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	workflow := uuid.New()

	testCases := []struct {
		name     string
		claims   CustomClaims
		wantKind authz.ResourceType
		wantID   *uuid.UUID
		wantErr  bool
	}{
		{name: "signed organization scope", claims: CustomClaims{OrgID: org.String(), Scope: string(authz.ResourceTypeOrganization), ScopeID: org.String()}, wantKind: authz.ResourceTypeOrganization, wantID: &org},
		{name: "signed project scope", claims: CustomClaims{OrgID: org.String(), ProjectID: project.String(), Scope: string(authz.ResourceTypeProject), ScopeID: project.String()}, wantKind: authz.ResourceTypeProject, wantID: &project},
		{name: "signed workflow-pinned scope", claims: CustomClaims{OrgID: org.String(), ProjectID: project.String(), WorkflowID: workflow.String(), Scope: string(authz.ResourceTypeProject), ScopeID: project.String()}, wantKind: authz.ResourceTypeProject, wantID: &project},
		{name: "signed product scope", claims: CustomClaims{OrgID: org.String(), Scope: string(authz.ResourceTypeProduct), ScopeID: product.String()}, wantKind: authz.ResourceTypeProduct, wantID: &product},
		{name: "signed instance scope", claims: CustomClaims{Scope: string(authz.ResourceTypeInstance)}, wantKind: authz.ResourceTypeInstance},
		{name: "legacy instance-admin token", claims: CustomClaims{Scope: authz.ScopeInstanceAdmin}, wantKind: authz.ResourceTypeInstance},
		{name: "legacy project token", claims: CustomClaims{OrgID: org.String(), ProjectID: project.String()}, wantKind: authz.ResourceTypeProject, wantID: &project},
		{name: "legacy workflow-pinned token", claims: CustomClaims{OrgID: org.String(), ProjectID: project.String(), WorkflowID: workflow.String()}, wantKind: authz.ResourceTypeProject, wantID: &project},
		{name: "legacy organization token", claims: CustomClaims{OrgID: org.String()}, wantKind: authz.ResourceTypeOrganization, wantID: &org},

		// Malformed scope claims
		{name: "an instance scope naming a resource", claims: CustomClaims{Scope: string(authz.ResourceTypeInstance), ScopeID: org.String()}, wantErr: true},
		{name: "a resource scope without an id", claims: CustomClaims{OrgID: org.String(), ProjectID: project.String(), Scope: string(authz.ResourceTypeProject)}, wantErr: true},
		{name: "a scope id that is not a uuid", claims: CustomClaims{OrgID: org.String(), ProjectID: project.String(), Scope: string(authz.ResourceTypeProject), ScopeID: "nope"}, wantErr: true},
		{name: "a scope no token has", claims: CustomClaims{OrgID: org.String(), Scope: "group", ScopeID: org.String()}, wantErr: true},
		{name: "legacy claims naming nothing", claims: CustomClaims{}, wantErr: true},
		{name: "a legacy project claim that is not a uuid", claims: CustomClaims{OrgID: org.String(), ProjectID: "nope"}, wantErr: true},

		// Claims that contradict themselves: every reader must see the same scope
		{name: "an instance scope naming an organization", claims: CustomClaims{OrgID: org.String(), Scope: string(authz.ResourceTypeInstance)}, wantErr: true},
		{name: "a legacy instance-admin claim naming an organization", claims: CustomClaims{OrgID: org.String(), Scope: authz.ScopeInstanceAdmin}, wantErr: true},
		{name: "an organization scope naming another organization", claims: CustomClaims{OrgID: org.String(), Scope: string(authz.ResourceTypeOrganization), ScopeID: other.String()}, wantErr: true},
		{name: "an organization scope with a project claim", claims: CustomClaims{OrgID: org.String(), ProjectID: project.String(), Scope: string(authz.ResourceTypeOrganization), ScopeID: org.String()}, wantErr: true},
		{name: "a project scope naming another project", claims: CustomClaims{OrgID: org.String(), ProjectID: project.String(), Scope: string(authz.ResourceTypeProject), ScopeID: other.String()}, wantErr: true},
		{name: "a project scope without an organization", claims: CustomClaims{ProjectID: project.String(), Scope: string(authz.ResourceTypeProject), ScopeID: project.String()}, wantErr: true},
		{name: "a product scope with a project claim", claims: CustomClaims{OrgID: org.String(), ProjectID: project.String(), Scope: string(authz.ResourceTypeProduct), ScopeID: product.String()}, wantErr: true},
		{name: "a product scope without an organization", claims: CustomClaims{Scope: string(authz.ResourceTypeProduct), ScopeID: product.String()}, wantErr: true},
		{name: "a workflow claim without a project claim", claims: CustomClaims{OrgID: org.String(), WorkflowID: workflow.String(), Scope: string(authz.ResourceTypeOrganization), ScopeID: org.String()}, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			kind, id, err := tc.claims.GetScope()
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantKind, kind)
			assert.Equal(t, tc.wantID, id)
		})
	}
}

func TestClaimsFromMap(t *testing.T) {
	testCases := []struct {
		name    string
		in      jwt.MapClaims
		want    *CustomClaims
		wantErr bool
	}{
		{
			name: "every claim is read",
			in: jwt.MapClaims{
				claimJTI: "id", "aud": []any{Audience}, "org_id": "o", "org_name": "on", "token_name": "t",
				"project_id": "p", "workflow_id": "w", "scope": "project", "scope_id": "s",
			},
			want: &CustomClaims{
				OrgID: "o", OrgName: "on", KeyName: "t", ProjectID: "p", WorkflowID: "w",
				Scope: string(authz.ResourceTypeProject), ScopeID: "s",
				RegisteredClaims: jwt.RegisteredClaims{ID: "id", Audience: jwt.ClaimStrings{Audience}},
			},
		},
		{
			name: "a single audience string is read",
			in:   jwt.MapClaims{claimJTI: "id", "aud": Audience},
			want: &CustomClaims{RegisteredClaims: jwt.RegisteredClaims{ID: "id", Audience: jwt.ClaimStrings{Audience}}},
		},
		{
			// The API entry point used to drop such a claim silently and skip its cross-check
			name:    "a claim of the wrong type is refused",
			in:      jwt.MapClaims{claimJTI: "id", "project_id": 42},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ClaimsFromMap(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func toPtr[T any](t T) *T {
	return &t
}
