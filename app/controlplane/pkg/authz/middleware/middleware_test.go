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

package middleware

import (
	"context"
	"io"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz/middleware/mocks"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var emptyHandler = func(_ context.Context, _ interface{}) (interface{}, error) { return nil, nil }

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

func TestWithAuthMiddleware(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))

	testCases := []struct {
		name           string
		hasUser        bool
		userRole       string
		hasToken       bool
		operationName  string
		hasPermissions bool
		wantErr        bool
	}{
		{
			name:    "neither an user nor a token is set",
			wantErr: true,
		},
		{
			name:          "there is a token but the operation is not in the allow list",
			hasToken:      true,
			operationName: "non-allowedlisted",
			wantErr:       true,
		},
		{
			name:          "token + operation in allowlist but no permissions",
			hasToken:      true,
			operationName: "/controlplane.v1.WorkflowContractService/List",
			wantErr:       true,
		},
		{
			name:           "token + operation in allowlist + permissions",
			hasToken:       true,
			operationName:  "/controlplane.v1.WorkflowContractService/List",
			hasPermissions: true,
		},
		{
			name:           "there is user that's admin so it should be allowed even if it has no permissions",
			hasUser:        true,
			userRole:       "role:org:admin",
			operationName:  "/controlplane.v1.WorkflowContractService/List",
			hasPermissions: false,
		},
		{
			name:           "same for owner",
			hasUser:        true,
			userRole:       "role:org:owner",
			operationName:  "/controlplane.v1.WorkflowContractService/List",
			hasPermissions: false,
		},
		{
			name:           "but viewer requires permissions",
			hasUser:        true,
			userRole:       "role:org:viewer",
			operationName:  "/controlplane.v1.WorkflowContractService/List",
			hasPermissions: false,
			wantErr:        true,
		},
		{
			name:           "but viewer requires permissions, and now it has it",
			hasUser:        true,
			userRole:       "role:org:viewer",
			operationName:  "/controlplane.v1.WorkflowContractService/List",
			hasPermissions: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set request context
			ctx := context.Background()
			var subject string
			// User and token
			if tc.hasUser {
				subject = tc.userRole
				ctx = usercontext.WithAuthzSubject(ctx, subject)
			}

			if tc.hasToken {
				s := authz.SubjectAPIToken{ID: "deadbeef"}
				subject = s.String()
				ctx = usercontext.WithAuthzSubject(ctx, subject)
			}

			// Request information
			ctx = transport.NewServerContext(ctx, &mockTransport{operation: tc.operationName})

			e := mocks.NewEnforcer(t)
			e.On("Enforce", subject, mock.Anything).Maybe().Return(tc.hasPermissions, nil)

			m := WithAuthzMiddleware(e, logger)
			_, err := m(emptyHandler)(ctx, nil)

			if tc.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.IsForbidden(err))
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestPoliciesLookup(t *testing.T) {
	testCases := []struct {
		name      string
		operation string
		wantErr   bool
	}{
		{
			name:    "empty operation",
			wantErr: true,
		},
		{
			name:      "operation not found",
			operation: "non-existing-operation",
			wantErr:   true,
		},
		{
			name:      "operation found in first pass",
			operation: "/controlplane.v1.WorkflowContractService/List",
		},
		{
			name:      "operation found with regexp",
			operation: "/controlplane.v1.OrgMetricsService/List",
		},
		{
			name:      "operation found with regexp 2",
			operation: "/controlplane.v1.OrgMetricsService/boom",
		},
		{
			name:      "operation found with regexp, error wrong prefix",
			operation: "/boom/controlplane.v1.OrgMetricsService",
			wantErr:   true,
		},
		{
			name:      "operation found with regexp, error not found",
			operation: "/controlplane.v1.OrgMetricsService",
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := policiesLookup(tc.operation)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
