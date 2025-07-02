//
// Copyright 2024 The Chainloop Authors.
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
	"testing"

	"github.com/golang-jwt/jwt/v4"

	v1 "github.com/chainloop-dev/chainloop/pkg/attestation/crafter/api/attestation/v1"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  *ParsedToken
	}{
		{
			name:  "user account",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYmMwYjIxOTktY2E4NS00MmFiLWE4NTctMDQyZTljMTA5ZDQzIiwiaXNzIjoiY3AuY2hhaW5sb29wIiwiYXVkIjpbInVzZXItYXV0aC5jaGFpbmxvb3AiXSwiZXhwIjoxNzE1OTM1MjUwfQ.ounYshGtagtYQsVIzNeE0ztVYRXrmjFSpdmaTF4QvyY",
			want: &ParsedToken{
				ID:        "bc0b2199-ca85-42ab-a857-042e9c109d43",
				TokenType: v1.Attestation_Auth_AUTH_TYPE_USER,
			},
		},
		{
			name:  "api token",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJvcmdfaWQiOiJkZGRiODIwMS1lYWI2LTRlNjEtOTIwMS1mMTJiNDdjMDE4OTIiLCJpc3MiOiJjcC5jaGFpbmxvb3AiLCJhdWQiOlsiYXBpLXRva2VuLWF1dGguY2hhaW5sb29wIl0sImp0aSI6IjRiMGYwZGQ0LTQ1MzgtNDI2OS05MmE5LWFiNWIwZmNlMDI1OCJ9.yMgsoe4CcqYoNp0xtrvvSGj1Y74HeqxoxS5sw8pdnQ8",
			want: &ParsedToken{
				ID:        "4b0f0dd4-4538-4269-92a9-ab5b0fce0258",
				TokenType: v1.Attestation_Auth_AUTH_TYPE_API_TOKEN,
				OrgID:     "dddb8201-eab6-4e61-9201-f12b47c01892",
			},
		},
		{
			name:  "federated token",
			token: "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImRldi1rZXkifQ.eyJpc3MiOiJodHRwczovL2NoYWlubG9vcC5naXRsYWIuY29tIiwic3ViIjoicHJvamVjdF9wYXRoOmNoYWlubG9vcC9wcm9qZWN0OnJlZl90eXBlOmJyYW5jaDpyZWY6bWFpbiIsImF1ZCI6ImNoYWlubG9vcCIsImV4cCI6MTczMDAwMDAwMCwibmJmIjoxNzI5OTk2NDAwLCJpYXQiOjE3Mjk5OTY0MDAsImp0aSI6ImpvYi05ODc2IiwicmVmIjoibWFpbiIsInJlZl90eXBlIjoiYnJhbmNoIiwicHJvamVjdF9pZCI6IjQyNDIiLCJwcm9qZWN0X3BhdGgiOiJjaGFpbmxvb3AvcHJvamVjdCIsIm5hbWVzcGFjZV9pZCI6IjQyNDMiLCJuYW1lc3BhY2VfcGF0aCI6ImNoYWlubG9vcCIsInVzZXJfbG9naW4iOiJnaXRsYWItY2ktdG9rZW4iLCJ1c2VyX2VtYWlsIjoiY2lAdXNlci5jb20iLCJ1c2VyX2FjY2Vzc19sZXZlbCI6ImRldmVsb3BlciIsInBpcGVsaW5lX2lkIjoiMTAxIiwicGlwZWxpbmVfc291cmNlIjoicHVzaCIsImpvYl9pZCI6IjIwMiIsInJlZl9wcm90ZWN0ZWQiOnRydWUsImVudmlyb25tZW50IjoicHJvZHVjdGlvbiIsImVudmlyb25tZW50X3Byb3RlY3RlZCI6dHJ1ZSwiZGVwbG95bWVudF90aWVyIjoicHJvZHVjdGlvbiJ9.LkNvVGVzdFNpZ25hdHVyZUNoYWluTG9vcA",
			want: &ParsedToken{
				ID:        "https://chainloop.gitlab.com",
				TokenType: v1.Attestation_Auth_AUTH_TYPE_FEDERATED,
			},
		},
		{
			name:  "old api token (without orgID)",
			token: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJjcC5jaGFpbmxvb3AiLCJhdWQiOlsiYXBpLXRva2VuLWF1dGguY2hhaW5sb29wIl0sImp0aSI6ImQ0ZTBlZTVlLTk3MTMtNDFkMi05ZmVhLTBiZGIxNDAzMzA4MSJ9.IOd3JIHPwfo9ihU20kvRwLIQJcQtTvp-ajlGqlCD4Es",
			want: &ParsedToken{
				ID:        "d4e0ee5e-9713-41d2-9fea-0bdb14033081",
				TokenType: v1.Attestation_Auth_AUTH_TYPE_API_TOKEN,
			},
		},
		{
			name:  "totally random token",
			token: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE3MTYxOTE2ODQsImV4cCI6MTc0NzcyNzY4NCwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsIkdpdmVuTmFtZSI6IkpvaG5ueSIsIlN1cm5hbWUiOiJSb2NrZXQiLCJFbWFpbCI6Impyb2NrZXRAZXhhbXBsZS5jb20iLCJSb2xlIjpbIk1hbmFnZXIiLCJQcm9qZWN0IEFkbWluaXN0cmF0b3IiXX0.5UnBivwkQCG4qWgi-gWkJ-Dsd7-A9G_EVEvswODc7Kk",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.token)
			assert.NoError(t, err)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}

			assert.Equal(t, tt.want.ID, got.ID)
			assert.Equal(t, tt.want.TokenType, got.TokenType)
			assert.Equal(t, tt.want.OrgID, got.OrgID)
		})
	}
}

func TestIsGitLabFederatedToken(t *testing.T) {
	tests := []struct {
		name   string
		claims jwt.MapClaims
		want   bool
	}{
		{
			name:   "empty claims",
			claims: jwt.MapClaims{},
			want:   false,
		},
		{
			name: "exactly 10 gitlab",
			claims: jwt.MapClaims{
				"namespace_id":   "4243",
				"namespace_path": "chainloop",
				"project_id":     "4242",
				"project_path":   "chainloop/project",
				"user_id":        "123",
				"user_login":     "gitlab-ci-token",
				"user_email":     "ci@gitlab.com",
				"pipeline_id":    "101",
				"job_id":         "202",
				"ref":            "main",
			},
			want: true,
		},
		{
			name: "9 gitlab claims",
			claims: jwt.MapClaims{
				"namespace_id":   "4243",
				"namespace_path": "chainloop",
				"project_id":     "4242",
				"project_path":   "chainloop/project",
				"user_id":        "123",
				"user_login":     "gitlab-ci-token",
				"pipeline_id":    "101",
				"job_id":         "202",
				"ref":            "main",
			},
			want: false,
		},
		{
			name: "all gitlab claims",
			claims: jwt.MapClaims{
				"namespace_id":          "4243",
				"namespace_path":        "chainloop",
				"project_id":            "4242",
				"project_path":          "chainloop/project",
				"user_id":               "123",
				"user_login":            "gitlab-ci-token",
				"user_email":            "ci@gitlab.com",
				"user_access_level":     "developer",
				"pipeline_id":           "101",
				"pipeline_source":       "push",
				"job_id":                "202",
				"ref":                   "main",
				"ref_type":              "branch",
				"ref_protected":         true,
				"groups_direct":         []string{"group1"},
				"environment":           "production",
				"environment_protected": true,
				"deployment_tier":       "production",
				"deployment_action":     "deploy",
				"runner_id":             "runner-1",
				"runner_environment":    "production",
				"sha":                   "abc123",
				"ci_config_ref_uri":     "https://gitlab.com",
				"ci_config_sha":         "config-abc123",
				"project_visibility":    "public",
			},
			want: true,
		},
		{
			name: "10 gitlab claims mixed with non-gitlab claims",
			claims: jwt.MapClaims{
				"namespace_id":   "4243",
				"namespace_path": "chainloop",
				"project_id":     "4242",
				"project_path":   "chainloop/project",
				"user_id":        "123",
				"user_login":     "gitlab-ci-token",
				"user_email":     "ci@gitlab.com",
				"pipeline_id":    "101",
				"job_id":         "202",
				"ref":            "main",
				"custom_claim_1": "value1",
				"custom_claim_2": "value2",
				"custom_claim_3": "value3",
			},
			want: true,
		},
		{
			name: "9 gitlab claims mixed with non-gitlab claims",
			claims: jwt.MapClaims{
				"namespace_id":   "4243",
				"namespace_path": "chainloop",
				"project_id":     "4242",
				"project_path":   "chainloop/project",
				"user_id":        "123",
				"user_login":     "gitlab-ci-token",
				"pipeline_id":    "101",
				"job_id":         "202",
				"ref":            "main",
				"custom_claim_1": "value1",
				"custom_claim_2": "value2",
				"custom_claim_3": "value3",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGitLabFederatedToken(tt.claims)
			assert.Equal(t, tt.want, got)
		})
	}
}
