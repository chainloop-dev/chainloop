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

package usercontext

import (
	"context"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockTransport is a gRPC transport.
type mockTransport struct {
	operation string
}

// Kind returns the transport kind.
func (tr *mockTransport) Kind() transport.Kind {
	return transport.KindGRPC
}

// Endpoint returns the transport endpoint.
func (tr *mockTransport) Endpoint() string {
	return ""
}

// Operation returns the transport operation.
func (tr *mockTransport) Operation() string {
	return tr.operation
}

// RequestHeader returns the request header.
func (tr *mockTransport) RequestHeader() transport.Header {
	return nil
}

// ReplyHeader returns the reply header.
func (tr *mockTransport) ReplyHeader() transport.Header {
	return nil
}

func TestCheckUserInAllowList(t *testing.T) {
	testCases := []struct {
		name                string
		selectedRoutes      []string
		runningRoute        string
		hasRestrictedAccess bool
		isAPIToken          bool
		wantErr             bool
		customErrMsg        string
	}{
		{
			name: "empty allow list",
		},
		{
			name:                "is an API token so allow-list gets skipped",
			isAPIToken:          true,
			wantErr:             false,
			hasRestrictedAccess: true,
		},
		{
			name:                "user not in allow list",
			wantErr:             true,
			hasRestrictedAccess: true,
		},
		{
			name:                "user in allow list",
			hasRestrictedAccess: false,
		},
		{
			name:                "user not allowed to access the route",
			runningRoute:        "/foo/bar",
			selectedRoutes:      []string{"/foo/bar"},
			wantErr:             true,
			hasRestrictedAccess: true,
		},
		{
			name:                "route not in selected routes",
			runningRoute:        "/foo/bar",
			selectedRoutes:      []string{"/bar/foo", "/request-access", "/forbidden/route"},
			hasRestrictedAccess: true,
		},
		{
			name:                "return custom error message",
			wantErr:             true,
			customErrMsg:        "custom error message",
			hasRestrictedAccess: true,
		},
		{
			name:                "context missing, no user loaded",
			wantErr:             true,
			hasRestrictedAccess: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowList := &conf.Auth_AllowList{
				CustomMessage:  tc.customErrMsg,
				SelectedRoutes: tc.selectedRoutes,
			}

			ctx := context.Background()
			usecase := bizMocks.NewUserOrgFinder(t)
			usecase.On("FindByID", mock.Anything, "124").
				Return(&biz.User{HasRestrictedAccess: biz.ToPtr(tc.hasRestrictedAccess)}, nil).
				Maybe()

			m := CheckUserHasAccess(allowList, usecase)
			if tc.isAPIToken {
				ctx = entities.WithCurrentAPIToken(ctx, &entities.APIToken{ID: "124"})
			} else {
				ctx = entities.WithCurrentUser(ctx, &entities.User{ID: "124"})
			}

			if tc.runningRoute != "" {
				ctx = transport.NewServerContext(ctx, &mockTransport{operation: tc.runningRoute})
			}

			_, err := m(emptyHandler)(ctx, nil)

			if tc.wantErr {
				assert.True(t, v1.IsAllowListErrorNotInList(err))
				if tc.customErrMsg != "" {
					assert.Contains(t, err.Error(), tc.customErrMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
