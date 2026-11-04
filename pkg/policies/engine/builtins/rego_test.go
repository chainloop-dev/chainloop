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

package builtins

import (
	"context"
	"net"
	"sync"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const containerImageKind = "CONTAINER_IMAGE"

// fakeReferrerService records the discover requests it receives.
type fakeReferrerService struct {
	v1.UnimplementedReferrerServiceServer
	mu       sync.Mutex
	requests []*v1.ReferrerServiceDiscoverPrivateRequest
}

func (f *fakeReferrerService) DiscoverPrivate(_ context.Context, req *v1.ReferrerServiceDiscoverPrivateRequest) (*v1.ReferrerServiceDiscoverPrivateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, req)

	return &v1.ReferrerServiceDiscoverPrivateResponse{
		Result: &v1.ReferrerItem{Digest: req.GetDigest(), Kind: containerImageKind},
	}, nil
}

func (f *fakeReferrerService) lastRequest() *v1.ReferrerServiceDiscoverPrivateRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.requests) == 0 {
		return nil
	}

	return f.requests[len(f.requests)-1]
}

func newFakeReferrerConn(t *testing.T) (*grpc.ClientConn, *fakeReferrerService) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	fake := &fakeReferrerService{}
	v1.RegisterReferrerServiceServer(srv, fake)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return conn, fake
}

func TestDiscoverBuiltins(t *testing.T) {
	conn, fake := newFakeReferrerConn(t)
	require.NoError(t, RegisterDiscoverBuiltin(conn))

	testCases := []struct {
		name               string
		query              string
		wantKind           string
		wantCurrentOrgOnly bool
	}{
		{
			name:  "discover searches every organization",
			query: `chainloop.discover("sha256:deadbeef", "")`,
		},
		{
			name:     "discover forwards the kind",
			query:    `chainloop.discover("sha256:deadbeef", "CONTAINER_IMAGE")`,
			wantKind: containerImageKind,
		},
		{
			name:               "discover_current_org scopes to the current organization",
			query:              `chainloop.discover_current_org("sha256:deadbeef", "")`,
			wantCurrentOrgOnly: true,
		},
		{
			name:               "discover_current_org forwards the kind",
			query:              `chainloop.discover_current_org("sha256:deadbeef", "CONTAINER_IMAGE")`,
			wantKind:           containerImageKind,
			wantCurrentOrgOnly: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rs, err := rego.New(rego.Query(tc.query)).Eval(context.Background())
			require.NoError(t, err)
			require.Len(t, rs, 1)
			require.Len(t, rs[0].Expressions, 1)

			got, ok := rs[0].Expressions[0].Value.(map[string]any)
			require.True(t, ok, "unexpected result type %T", rs[0].Expressions[0].Value)
			assert.Equal(t, "sha256:deadbeef", got["digest"])

			req := fake.lastRequest()
			require.NotNil(t, req)
			assert.Equal(t, "sha256:deadbeef", req.GetDigest())
			assert.Equal(t, tc.wantKind, req.GetKind())
			assert.Equal(t, tc.wantCurrentOrgOnly, req.GetCurrentOrgOnly())
		})
	}
}
