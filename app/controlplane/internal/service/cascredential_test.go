//
// Copyright 2026 The Chainloop Authors.
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

package service

import (
	"strings"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	casJWT "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	"github.com/stretchr/testify/assert"
)

func TestResolveSourceInternal(t *testing.T) {
	testCases := []struct {
		name      string
		requested bool
		token     *entities.APIToken
		want      bool
	}{
		{
			name:      "not requested, no token (user auth)",
			requested: false,
			token:     nil,
			want:      false,
		},
		{
			name:      "not requested, regular API token",
			requested: false,
			token:     &entities.APIToken{},
			want:      false,
		},
		{
			name:      "not requested, system API token",
			requested: false,
			token:     &entities.APIToken{IsSystem: true},
			want:      false,
		},
		{
			name:      "requested by system API token",
			requested: true,
			token:     &entities.APIToken{IsSystem: true},
			want:      true,
		},
		{
			// best-effort: a non-system token requesting it is ignored, not rejected,
			// to tolerate outdated systems whose tokens are not yet marked as system
			name:      "requested by regular API token falls back to false",
			requested: true,
			token:     &entities.APIToken{},
			want:      false,
		},
		{
			name:      "requested by user auth falls back to false",
			requested: true,
			token:     nil,
			want:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveSourceInternal(tc.requested, tc.token)
			assert.Equal(t, tc.want, got)
		})
	}
}

// A token restricted to artifacts only downloads, and only the digests in its list. A token
// without the restriction, and a user, are left alone.
func TestRestrictToAllowedDigest(t *testing.T) {
	const (
		allowed = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		other   = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	)

	restricted := &entities.APIToken{AllowedDigests: []string{allowed}}

	testCases := []struct {
		name           string
		token          *entities.APIToken
		digest         string
		role           casJWT.Role
		wantRestricted bool
		wantErr        bool
	}{
		{name: "a user", digest: other, role: casJWT.Uploader},
		{name: "a token without the restriction", token: &entities.APIToken{}, digest: other, role: casJWT.Uploader},
		{name: "a token without the restriction and no digest", token: &entities.APIToken{}, role: casJWT.Downloader},
		{name: "an allowed digest", token: restricted, digest: allowed, role: casJWT.Downloader, wantRestricted: true},
		{name: "another digest", token: restricted, digest: other, role: casJWT.Downloader, wantRestricted: true, wantErr: true},
		{name: "no digest", token: restricted, role: casJWT.Downloader, wantRestricted: true, wantErr: true},
		{name: "an upload of an allowed digest", token: restricted, digest: allowed, role: casJWT.Uploader, wantRestricted: true, wantErr: true},
		{name: "no role", token: restricted, digest: allowed, wantRestricted: true, wantErr: true},
		{name: "a digest that differs in case", token: restricted, digest: strings.ToUpper(allowed), role: casJWT.Downloader, wantRestricted: true, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gotRestricted, err := restrictToAllowedDigest(tc.token, tc.digest, tc.role)
			assert.Equal(t, tc.wantRestricted, gotRestricted)
			if tc.wantErr {
				assert.ErrorIs(t, err, errDigestNotAllowed)
				return
			}
			assert.NoError(t, err)
		})
	}
}
