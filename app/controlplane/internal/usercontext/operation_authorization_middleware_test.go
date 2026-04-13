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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext/entities"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTransport implements transport.Transporter for testing middleware operation matching.
type fakeTransport struct {
	operation string
	header    headerCarrier
}

type headerCarrier http.Header

func (h headerCarrier) Get(key string) string { return http.Header(h).Get(key) }
func (h headerCarrier) Set(key, value string) { http.Header(h).Set(key, value) }
func (h headerCarrier) Add(key, value string) { http.Header(h).Add(key, value) }
func (h headerCarrier) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
func (h headerCarrier) Values(key string) []string { return http.Header(h).Values(key) }

func (f *fakeTransport) Kind() transport.Kind            { return transport.KindGRPC }
func (f *fakeTransport) Endpoint() string                { return "" }
func (f *fakeTransport) Operation() string               { return f.operation }
func (f *fakeTransport) RequestHeader() transport.Header { return f.header }
func (f *fakeTransport) ReplyHeader() transport.Header   { return nil }

func ctxWithOperation(ctx context.Context, op string) context.Context {
	return transport.NewServerContext(ctx, &fakeTransport{operation: op, header: headerCarrier{}})
}

func TestWithOperationAuthorizationMiddleware(t *testing.T) {
	logHelper := log.NewHelper(log.DefaultLogger)

	t.Run("disabled config is passthrough", func(t *testing.T) {
		m := WithOperationAuthorizationMiddleware(nil, logHelper)
		result, err := m(passHandler)(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", result)

		m = WithOperationAuthorizationMiddleware(&conf.OperationAuthorizationProvider{Enabled: false}, logHelper)
		result, err = m(passHandler)(context.Background(), nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", result)
	})

	t.Run("non-target operation is passthrough", func(t *testing.T) {
		var callCount atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			callCount.Add(1)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		cfg := &conf.OperationAuthorizationProvider{Enabled: true, Url: srv.URL}
		m := WithOperationAuthorizationMiddleware(cfg, logHelper)

		ctx := ctxWithOperation(context.Background(), "/controlplane.v1.WorkflowService/List")
		ctx = entities.WithCurrentUser(ctx, &entities.User{ID: "user-1"})

		result, err := m(passHandler)(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", result)
		assert.Equal(t, int32(0), callCount.Load())
	})

	t.Run("target operation allowed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req operationAuthRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "/controlplane.v1.OrganizationService/Create", req.Operation)
			assert.Equal(t, "user-1", req.UserID)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(operationAuthResponse{Allowed: true}) //nolint:errcheck
		}))
		defer srv.Close()

		cfg := &conf.OperationAuthorizationProvider{Enabled: true, Url: srv.URL}
		m := WithOperationAuthorizationMiddleware(cfg, logHelper)

		ctx := ctxWithOperation(context.Background(), "/controlplane.v1.OrganizationService/Create")
		ctx = entities.WithCurrentUser(ctx, &entities.User{ID: "user-1"})

		result, err := m(passHandler)(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", result)
	})

	t.Run("target operation denied", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(operationAuthResponse{Allowed: false, Reason: "org limit reached"}) //nolint:errcheck
		}))
		defer srv.Close()

		cfg := &conf.OperationAuthorizationProvider{Enabled: true, Url: srv.URL}
		m := WithOperationAuthorizationMiddleware(cfg, logHelper)

		ctx := ctxWithOperation(context.Background(), "/controlplane.v1.OrganizationService/Create")
		ctx = entities.WithCurrentUser(ctx, &entities.User{ID: "user-1"})

		result, err := m(passHandler)(ctx, nil)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "org limit reached")
	})

	t.Run("provider unreachable is fail-closed", func(t *testing.T) {
		cfg := &conf.OperationAuthorizationProvider{Enabled: true, Url: "http://127.0.0.1:1"}
		m := WithOperationAuthorizationMiddleware(cfg, logHelper)

		ctx := ctxWithOperation(context.Background(), "/controlplane.v1.OrganizationService/Create")
		ctx = entities.WithCurrentUser(ctx, &entities.User{ID: "user-1"})

		result, err := m(passHandler)(ctx, nil)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unable to verify operation authorization")
	})

	t.Run("provider returns 500 is fail-closed", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &conf.OperationAuthorizationProvider{Enabled: true, Url: srv.URL}
		m := WithOperationAuthorizationMiddleware(cfg, logHelper)

		ctx := ctxWithOperation(context.Background(), "/controlplane.v1.OrganizationService/Create")
		ctx = entities.WithCurrentUser(ctx, &entities.User{ID: "user-2"})

		result, err := m(passHandler)(ctx, nil)
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "unable to verify operation authorization")
	})

	t.Run("organization ID is forwarded", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req operationAuthRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "org-123", req.OrganizationID)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(operationAuthResponse{Allowed: true}) //nolint:errcheck
		}))
		defer srv.Close()

		cfg := &conf.OperationAuthorizationProvider{Enabled: true, Url: srv.URL}
		m := WithOperationAuthorizationMiddleware(cfg, logHelper)

		ctx := ctxWithOperation(context.Background(), "/controlplane.v1.OrganizationService/Create")
		ctx = entities.WithCurrentUser(ctx, &entities.User{ID: "user-1"})
		ctx = entities.WithCurrentOrg(ctx, &entities.Org{ID: "org-123"})

		result, err := m(passHandler)(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", result)
	})

	t.Run("bearer token is forwarded", func(t *testing.T) {
		var gotAuth string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(operationAuthResponse{Allowed: true}) //nolint:errcheck
		}))
		defer srv.Close()

		cfg := &conf.OperationAuthorizationProvider{Enabled: true, Url: srv.URL}
		m := WithOperationAuthorizationMiddleware(cfg, logHelper)

		ft := &fakeTransport{
			operation: "/controlplane.v1.OrganizationService/Create",
			header:    headerCarrier(http.Header{"Authorization": []string{"Bearer test-token-123"}}),
		}
		ctx := transport.NewServerContext(context.Background(), ft)
		ctx = entities.WithCurrentUser(ctx, &entities.User{ID: "user-1"})

		result, err := m(passHandler)(ctx, nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", result)
		assert.Equal(t, "Bearer test-token-123", gotAuth)
	})
}
