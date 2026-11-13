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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"net"
	"testing"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/sandboxidentity"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func newIdentityTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test root"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign, SubjectKeyId: []byte("root"),
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return cert, key
}

// trustedRootConn serves the CA as the trusted root of a fake control plane.
func trustedRootConn(t *testing.T, root *x509.Certificate) *grpc.ClientConn {
	t.Helper()

	rootPEM, err := cryptoutils.MarshalCertificateToPEM(root)
	require.NoError(t, err)
	keyID := sha256.Sum256(root.SubjectKeyId)

	svc := &fakeSigningService{trustedRoot: &pb.GetTrustedRootResponse{
		Keys: map[string]*pb.CertificateChain{hex.EncodeToString(keyID[:]): {Certificates: []string{string(rootPEM)}}},
	}}

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

func TestApplySandboxIdentity(t *testing.T) {
	const commit = "ea3a974660a030f0314aa1505a64792bf2633b33"
	wf := &pb.WorkflowItem{Id: uuid.NewString(), ProjectId: uuid.NewString()}
	root, rootKey := newIdentityTestCA(t)

	// A certificate with the mark, as the control plane CA gives it to the platform
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafDER, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"org-id"}, OrganizationalUnit: []string{sandboxidentity.CertificateMark}},
		NotBefore:    time.Now().Add(-time.Minute), NotAfter: time.Now().Add(10 * time.Minute),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}, root, &key.PublicKey, rootKey)
	require.NoError(t, err)
	leaf, err := x509.ParseCertificate(leafDER)
	require.NoError(t, err)

	sign := func(t *testing.T, workflowID string) string {
		t.Helper()
		raw, err := sandboxidentity.Sign(key, []*x509.Certificate{leaf}, &sandboxidentity.Claims{
			RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now())},
			OrganizationID:   "org-id", ProjectID: wf.GetProjectId(), WorkflowID: workflowID, ChainloopTokenID: uuid.NewString(),
			Data: &sandboxidentity.Data{CommitVerification: &sandboxidentity.CommitVerification{
				Commit:       commit,
				Verification: &commitverification.CommitVerification{Attempted: true, Status: commitverification.VerificationStatusVerified},
			}},
		})
		require.NoError(t, err)
		return raw
	}

	verified := &pb.AttestationServiceInitResponse_SandboxIdentityResult{Verified: true}

	testCases := []struct {
		name     string
		raw      string
		cpResult *pb.AttestationServiceInitResponse_SandboxIdentityResult
		conn     *grpc.ClientConn
		wantAuth bool
	}{
		{name: "verified by the control plane and the CLI", raw: sign(t, wf.GetId()), cpResult: verified, conn: trustedRootConn(t, root), wantAuth: true},
		{name: "a control plane that does not know the identity", raw: sign(t, wf.GetId()), conn: trustedRootConn(t, root)},
		{name: "rejected by the control plane", raw: sign(t, wf.GetId()), cpResult: &pb.AttestationServiceInitResponse_SandboxIdentityResult{Reason: "another token"}, conn: trustedRootConn(t, root)},
		{name: "an identity for another workflow", raw: sign(t, uuid.NewString()), cpResult: verified, conn: trustedRootConn(t, root)},
		{name: "another trusted root", raw: sign(t, wf.GetId()), cpResult: verified, conn: newSigningServiceConn(t, true)},
		{name: "no trusted root", raw: sign(t, wf.GetId()), cpResult: verified, conn: newSigningServiceConn(t, false)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			action := &AttestationInit{ActionsOpts: &ActionsOpts{CPConnection: tc.conn, Logger: zerolog.Nop()}}
			runner := runners.NewChainloopSandbox()

			action.applySandboxIdentity(context.Background(), runner, tc.raw, tc.cpResult, wf)

			assert.Equal(t, tc.wantAuth, runner.IsAuthenticated())
			if tc.wantAuth {
				assert.NotNil(t, runner.VerifyCommitSignature(context.Background(), commit))
			} else {
				assert.Nil(t, runner.VerifyCommitSignature(context.Background(), commit))
			}
		})
	}
}
