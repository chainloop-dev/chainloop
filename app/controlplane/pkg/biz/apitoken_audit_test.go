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
	"encoding/json"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/auditor/events"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// auditTokenRepo keeps created tokens in memory; the embedded interface panics on anything else.
type auditTokenRepo struct {
	APITokenRepo
	rows map[uuid.UUID]*APIToken
}

func (r *auditTokenRepo) Create(_ context.Context, opts *APITokenCreateOpts) (*APIToken, error) {
	t := &APIToken{ID: uuid.New(), Name: opts.Name, ProjectID: opts.ProjectID, Scope: opts.Scope, ScopeID: opts.ScopeID, Policies: opts.Policies}
	if opts.OrganizationID != nil {
		t.OrganizationID = *opts.OrganizationID
	}
	r.rows[t.ID] = t
	return t, nil
}

func (r *auditTokenRepo) FindByID(_ context.Context, id uuid.UUID) (*APIToken, error) {
	return r.rows[id], nil
}

func (r *auditTokenRepo) Revoke(_ context.Context, _ *uuid.UUID, _ uuid.UUID) error { return nil }

type auditOrgRepo struct {
	OrganizationRepo
	org *Organization
}

func (r *auditOrgRepo) FindByID(_ context.Context, _ uuid.UUID) (*Organization, error) {
	return r.org, nil
}

// The audit trail must say what a token reaches: the created and revoked events carry the
// scope recorded on the row.
func TestAPITokenAuditEventsCarryTheScope(t *testing.T) {
	orgID, productID := uuid.New(), uuid.New()
	org := &Organization{ID: orgID.String(), Name: "acme"}

	testCases := []struct {
		name        string
		opts        []APITokenCreateOpt
		wantScope   authz.ResourceType
		wantScopeID uuid.UUID
	}{
		{name: "a product-scoped token", opts: []APITokenCreateOpt{APITokenWithScope(authz.ResourceTypeProduct, &productID)}, wantScope: authz.ResourceTypeProduct, wantScopeID: productID},
		{name: "an organization token", wantScope: authz.ResourceTypeOrganization, wantScopeID: orgID},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			auditorUC, published := newRecordingAuditor()
			uc, err := NewAPITokenUseCase(&auditTokenRepo{rows: map[uuid.UUID]*APIToken{}}, &APITokenJWTConfig{SymmetricHmacKey: "audit-hmac-key"}, nil,
				&OrganizationUseCase{orgRepo: &auditOrgRepo{org: org}}, auditorUC, nil)
			require.NoError(t, err)

			ctx := ctxWithAPITokenActor(context.Background())
			orgIDStr := orgID.String()
			token, err := uc.Create(ctx, "ci", nil, nil, &orgIDStr, tc.opts...)
			require.NoError(t, err)
			require.NoError(t, uc.Revoke(ctx, orgIDStr, token.ID.String()))

			require.Len(t, published.published, 2)
			for i, wantAction := range []string{events.APITokenCreatedActionType, events.APITokenRevokedActionType} {
				got := published.published[i].Data
				assert.Equal(t, wantAction, got.ActionType)

				var info struct {
					Scope   *authz.ResourceType `json:"scope"`
					ScopeID *uuid.UUID          `json:"scope_id"`
				}
				require.NoError(t, json.Unmarshal(got.Info, &info))
				require.NotNil(t, info.Scope, wantAction)
				assert.Equal(t, tc.wantScope, *info.Scope, wantAction)
				require.NotNil(t, info.ScopeID, wantAction)
				assert.Equal(t, tc.wantScopeID, *info.ScopeID, wantAction)
			}
		})
	}
}
