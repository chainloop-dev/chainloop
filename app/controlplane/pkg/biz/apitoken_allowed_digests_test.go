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

package biz

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// allowedDigestsTokenRepo records the options of the token it creates; the embedded interface
// panics on anything else.
type allowedDigestsTokenRepo struct {
	APITokenRepo
	created *APITokenCreateOpts
}

func (r *allowedDigestsTokenRepo) Create(_ context.Context, opts *APITokenCreateOpts) (*APIToken, error) {
	r.created = opts
	t := &APIToken{ID: uuid.New(), Name: opts.Name, Scope: opts.Scope, ScopeID: opts.ScopeID, Policies: opts.Policies, AllowedDigests: opts.AllowedDigests}
	if opts.OrganizationID != nil {
		t.OrganizationID = *opts.OrganizationID
	}

	return t, nil
}

// A token can be restricted to a list of sha256 digests. The list is stored deduplicated and
// sorted. Create refuses an empty list, a digest that is not sha256, and a token that can upload.
func TestCreateAPITokenWithAllowedDigests(t *testing.T) {
	const (
		digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		// the refusal of a digest that does not parse
		errInvalidDigest = "invalid digest"
	)

	orgID := uuid.New()
	org := &Organization{ID: orgID.String(), Name: "acme"}
	downloadOnly := APITokenWithPolicies([]*authz.Policy{authz.PolicyArtifactDownload})

	testCases := []struct {
		name string
		opts []APITokenCreateOpt
		// want is the stored list; nil means no restriction
		want    []string
		wantErr string
	}{
		{name: "no restriction", opts: []APITokenCreateOpt{downloadOnly}},
		{name: "one digest", opts: []APITokenCreateOpt{downloadOnly, APITokenWithAllowedDigests(digestA)}, want: []string{digestA}},
		{
			name: "digests are deduplicated and sorted",
			opts: []APITokenCreateOpt{downloadOnly, APITokenWithAllowedDigests(digestB, digestA, digestB)},
			want: []string{digestA, digestB},
		},
		{name: "an empty list", opts: []APITokenCreateOpt{downloadOnly, APITokenWithAllowedDigests()}, wantErr: "at least one digest"},
		{name: "an empty digest", opts: []APITokenCreateOpt{downloadOnly, APITokenWithAllowedDigests(digestA, "")}, wantErr: errInvalidDigest},
		{name: "a digest that is not one", opts: []APITokenCreateOpt{downloadOnly, APITokenWithAllowedDigests("sha256:nothex")}, wantErr: errInvalidDigest},
		{name: "a digest without its algorithm", opts: []APITokenCreateOpt{downloadOnly, APITokenWithAllowedDigests(digestA[len("sha256:"):])}, wantErr: errInvalidDigest},
		{
			name:    "a sha512 digest",
			opts:    []APITokenCreateOpt{downloadOnly, APITokenWithAllowedDigests("sha512:" + digestA[len("sha256:"):] + digestA[len("sha256:"):])},
			wantErr: "only sha256",
		},
		{
			name:    "the default policies allow an upload",
			opts:    []APITokenCreateOpt{APITokenWithAllowedDigests(digestA)},
			wantErr: "upload",
		},
		{
			name: "explicit policies with the upload policy",
			opts: []APITokenCreateOpt{
				APITokenWithPolicies([]*authz.Policy{authz.PolicyArtifactDownload, {Resource: authz.ResourceCASArtifact, Action: authz.ActionCreate}}),
				APITokenWithAllowedDigests(digestA),
			},
			wantErr: "upload",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &allowedDigestsTokenRepo{}
			auditorUC, _ := newRecordingAuditor()
			uc, err := NewAPITokenUseCase(repo, &APITokenJWTConfig{SymmetricHmacKey: "allowed-digests-hmac-key"}, nil,
				&OrganizationUseCase{orgRepo: &auditOrgRepo{org: org}}, auditorUC, nil)
			require.NoError(t, err)

			orgIDStr := orgID.String()
			token, err := uc.Create(ctxWithAPITokenActor(context.Background()), "ci", nil, nil, &orgIDStr, tc.opts...)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.True(t, IsErrValidation(err), "got %v", err)
				assert.ErrorContains(t, err, tc.wantErr)
				assert.Nil(t, repo.created, "a refused token is not stored")
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, repo.created.AllowedDigests)
			assert.Equal(t, tc.want, token.AllowedDigests)
		})
	}
}
