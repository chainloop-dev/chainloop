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

package usercontext

import (
	"context"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type suspensionMockTransport struct {
	operation string
}

func (tr *suspensionMockTransport) Kind() transport.Kind            { return transport.KindGRPC }
func (tr *suspensionMockTransport) Endpoint() string                { return "" }
func (tr *suspensionMockTransport) Operation() string               { return tr.operation }
func (tr *suspensionMockTransport) RequestHeader() transport.Header { return nil }
func (tr *suspensionMockTransport) ReplyHeader() transport.Header   { return nil }

var passHandler = func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil }

func TestWithSuspensionMiddleware(t *testing.T) {
	suspendedOrg := &entities.Org{ID: "org-1", Name: "test", Suspended: true}
	activeOrg := &entities.Org{ID: "org-1", Name: "test", Suspended: false}

	tests := []struct {
		name      string
		org       *entities.Org
		operation string
		wantErr   bool
		wantMsg   string
	}{
		{
			name:    "no org context passes through",
			org:     nil,
			wantErr: false,
		},
		{
			name:      "non-suspended org passes through",
			org:       activeOrg,
			operation: "/controlplane.v1.WorkflowService/Create",
			wantErr:   false,
		},
		{
			name:      "suspended org allows read operation",
			org:       suspendedOrg,
			operation: "/controlplane.v1.ReferrerService/DiscoverPrivate",
			wantErr:   false,
		},
		{
			name:      "suspended org allows list operation",
			org:       suspendedOrg,
			operation: "/controlplane.v1.WorkflowService/List",
			wantErr:   false,
		},
		{
			name:      "suspended org allows read policy operation",
			org:       suspendedOrg,
			operation: "/controlplane.v1.ContextService/Current",
			wantErr:   false,
		},
		{
			name:      "suspended org allows exempt empty-policy read",
			org:       suspendedOrg,
			operation: "/controlplane.v1.CASCredentialsService/Get",
			wantErr:   false,
		},
		{
			name:      "suspended org allows exempt navigation operation",
			org:       suspendedOrg,
			operation: "/controlplane.v1.UserService/ListMemberships",
			wantErr:   false,
		},
		{
			name:      "suspended org allows exempt group read",
			org:       suspendedOrg,
			operation: "/controlplane.v1.GroupService/ListMembers",
			wantErr:   false,
		},
		{
			name:      "suspended org allows self-service org delete",
			org:       suspendedOrg,
			operation: "/controlplane.v1.OrganizationService/Delete",
			wantErr:   false,
		},
		{
			name:      "suspended org allows self-service leave org",
			org:       suspendedOrg,
			operation: "/controlplane.v1.UserService/DeleteMembership",
			wantErr:   false,
		},
		{
			name:      "suspended org allows self-service delete account",
			org:       suspendedOrg,
			operation: "/controlplane.v1.AuthService/DeleteAccount",
			wantErr:   false,
		},
		{
			name:      "suspended org blocks empty-policy write",
			org:       suspendedOrg,
			operation: "/controlplane.v1.GroupService/AddMember",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks another empty-policy write",
			org:       suspendedOrg,
			operation: "/controlplane.v1.GroupService/RemoveMember",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks empty-policy org create",
			org:       suspendedOrg,
			operation: "/controlplane.v1.OrganizationService/Create",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks unmapped write",
			org:       suspendedOrg,
			operation: "/controlplane.v1.CASBackendService/Update",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks another unmapped write",
			org:       suspendedOrg,
			operation: "/controlplane.v1.OrgInvitationService/Revoke",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks attestation write",
			org:       suspendedOrg,
			operation: "/controlplane.v1.AttestationService/Init",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks attestation state write",
			org:       suspendedOrg,
			operation: "/controlplane.v1.AttestationStateService/Save",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks signing write",
			org:       suspendedOrg,
			operation: "/controlplane.v1.SigningService/GenerateSigningCert",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks mapped write policy",
			org:       suspendedOrg,
			operation: "/controlplane.v1.WorkflowService/Create",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks mapped update policy",
			org:       suspendedOrg,
			operation: "/controlplane.v1.CASBackendService/Revalidate",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks OrganizationService/DeleteMembership despite Delete being exempt",
			org:       suspendedOrg,
			operation: "/controlplane.v1.OrganizationService/DeleteMembership",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:      "suspended org blocks unknown future endpoint",
			org:       suspendedOrg,
			operation: "/controlplane.v1.NewService/SomeWrite",
			wantErr:   true,
			wantMsg:   "suspended",
		},
		{
			name:    "suspended org with no transport context is blocked",
			org:     suspendedOrg,
			wantErr: true,
			wantMsg: "suspended",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			if tt.org != nil {
				ctx = entities.WithCurrentOrg(ctx, tt.org)
			}

			if tt.operation != "" {
				ctx = transport.NewServerContext(ctx, &suspensionMockTransport{operation: tt.operation})
			}

			m := WithSuspensionMiddleware()
			result, err := m(passHandler)(ctx, nil)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantMsg)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "ok", result)
			}
		})
	}
}
