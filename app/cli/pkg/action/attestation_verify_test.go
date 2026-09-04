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

package action

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// caKeyID is the authority key identifier of testdata/ca.pub, the CA that signed
// the certificate embedded in the valid test bundles.
const caKeyID = "2a522d9652e0933d2a1237c395bc116e012f86dffff13122da59f76e0d2abe27"

func TestAttestationVerifyRun(t *testing.T) {
	testCases := []struct {
		name   string
		bundle string
		// withTrustedRoot serves the test CA as trusted root, otherwise the
		// signing service replies Unimplemented, simulating a control plane
		// with no signing CA configured.
		withTrustedRoot bool
		expectErr       string
		// expectNotVerifiable asserts the error is the sentinel telling the
		// bundle was never verified, as opposed to failing verification.
		expectNotVerifiable bool
	}{
		{
			name:            "valid bundle is verified",
			bundle:          "testdata/bundle_valid.json",
			withTrustedRoot: true,
		},
		{
			name:                "bundle without verification material fails",
			bundle:              "testdata/bundle_valid_nomaterial.json",
			withTrustedRoot:     true,
			expectErr:           "verification material",
			expectNotVerifiable: true,
		},
		{
			name:                "no trusted root configured fails",
			bundle:              "testdata/bundle_valid.json",
			expectErr:           "trusted root",
			expectNotVerifiable: true,
		},
		{
			name:            "tampered bundle fails verification",
			bundle:          "testdata/bundle_invalid.json",
			withTrustedRoot: true,
			expectErr:       "bundle verification failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := &ActionsOpts{
				CPConnection: newSigningServiceConn(t, tc.withTrustedRoot),
				Logger:       zerolog.Nop(),
			}

			err := NewAttestationVerifyAction(opts).Run(context.Background(), tc.bundle)
			if tc.expectErr == "" {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectErr)
			assert.Equal(t, tc.expectNotVerifiable, errors.Is(err, ErrAttestationNotVerifiable))
		})
	}
}

// fakeSigningService serves the test CA as trusted root, or replies Unimplemented
// when none is configured.
type fakeSigningService struct {
	pb.UnimplementedSigningServiceServer
	trustedRoot *pb.GetTrustedRootResponse
}

func (s *fakeSigningService) GetTrustedRoot(context.Context, *pb.GetTrustedRootRequest) (*pb.GetTrustedRootResponse, error) {
	if s.trustedRoot == nil {
		return nil, status.Error(codes.Unimplemented, "trusted root not available")
	}

	return s.trustedRoot, nil
}

func newSigningServiceConn(t *testing.T, withTrustedRoot bool) *grpc.ClientConn {
	t.Helper()

	svc := &fakeSigningService{}
	if withTrustedRoot {
		ca, err := os.ReadFile("testdata/ca.pub")
		require.NoError(t, err)
		svc.trustedRoot = &pb.GetTrustedRootResponse{
			Keys: map[string]*pb.CertificateChain{caKeyID: {Certificates: []string{string(ca)}}},
		}
	}

	l := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	pb.RegisterSigningServiceServer(srv, svc)
	go func() { _ = srv.Serve(l) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return l.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
		srv.Stop()
	})

	return conn
}
