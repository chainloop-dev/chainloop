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

package usercontext

import (
	"context"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
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
	const email = "sarah@cyberdyne.io"

	defaultRules := []string{
		"foo@foo.com",
		"sarah@cyberdyne.io",
		// it can also contain domains
		"@cyberdyne.io",
		"@dyson-industries.io",
	}

	testCases := []struct {
		name           string
		rules          []string
		selectedRoutes []string
		runningRoute   string
		email          string
		wantErr        bool
		customErrMsg   string
	}{
		{
			name:  "empty allow list",
			email: email,
		},
		{
			name:    "user not in allow list",
			email:   email,
			rules:   []string{"nothere@cyberdyne.io"},
			wantErr: true,
		},
		{
			name:           "user not allowed to access the route",
			email:          "random@example.com",
			runningRoute:   "/foo/bar",
			selectedRoutes: []string{"/foo/bar"},
			wantErr:        true,
		},
		{
			name:           "route not in selected routes",
			email:          "random@example.com",
			runningRoute:   "/foo/bar",
			selectedRoutes: []string{"/bar/foo", "/request-access", "/forbidden/route"},
		},
		{
			name:         "return custom error message",
			email:        email,
			rules:        []string{"nothere@cyberdyne.io"},
			wantErr:      true,
			customErrMsg: "custom error message",
		},
		{
			name:    "context missing, no user loaded",
			wantErr: true,
		},
		{
			name:  "user in allow list",
			email: email,
		},
		{
			name:  "user in one of the valid domains",
			email: "miguel@dyson-industries.io",
		},
		{
			name:  "user in one of the valid domains",
			email: "john@dyson-industries.io",
		},
		{
			name:  "and can use modifiers",
			email: "john+chainloop@dyson-industries.io",
		},
		{
			name:    "it needs to be an email",
			email:   "dyson-industries.io",
			wantErr: true,
		},
		{
			name:    "domain position is important",
			email:   "dyson-industries.io@john",
			wantErr: true,
		},
		{
			name:    "and can't be typosquated",
			email:   "john@dyson-industriesss.io",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			allowList := &conf.Auth_AllowList{
				Rules:          defaultRules,
				CustomMessage:  tc.customErrMsg,
				SelectedRoutes: tc.selectedRoutes,
			}

			if tc.rules != nil {
				allowList.Rules = tc.rules
			}

			m := CheckUserInAllowList(allowList)
			ctx := context.Background()
			if tc.email != "" {
				ctx = WithCurrentUser(ctx, &User{Email: tc.email, ID: "124"})
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
