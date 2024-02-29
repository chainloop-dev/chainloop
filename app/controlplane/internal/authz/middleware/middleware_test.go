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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/authz/middleware/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
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

// - it's a token but unknown operation
// - it's a token with known operation but wrong permissions
// - it's a token with known operation and right permissions
func TestWithAuthMiddleware(t *testing.T) {
	u := &usercontext.User{Email: "sarah@cyberdyne.io", ID: "124"}
	token := &usercontext.APIToken{ID: "deadbeef"}
	logger := log.NewHelper(log.NewStdLogger(io.Discard))

	testCases := []struct {
		name           string
		hasUser        bool
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set request context
			ctx := context.Background()
			var subject string
			// User and token
			if tc.hasUser {
				ctx = usercontext.WithCurrentUser(ctx, u)
				subject = "role:admin"
				ctx = usercontext.WithAuthzSubject(ctx, subject)
			}

			if tc.hasToken {
				ctx = usercontext.WithCurrentAPIToken(ctx, token)
				s := authz.SubjectAPIToken{ID: token.ID}
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
