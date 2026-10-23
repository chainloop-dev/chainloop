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

package sandboxidentity

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"testing"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

// fakeSigningClient issues certificates from a test CA, as the signing service of the control
// plane does. markDenied simulates a control plane that ignores or refuses the mark.
type fakeSigningClient struct {
	pb.SigningServiceClient
	t          *testing.T
	ca         *testCA
	markDenied bool
	err        error
	gotIssuer  bool
	// org is the organization that the CA writes in the subject. Empty means testOrg.
	org string
}

func (c *fakeSigningClient) GenerateSigningCert(_ context.Context, req *pb.GenerateSigningCertRequest, _ ...grpc.CallOption) (*pb.GenerateSigningCertResponse, error) {
	c.gotIssuer = req.GetSandboxIdentityIssuer()
	if c.err != nil {
		return nil, c.err
	}

	csr, err := cryptoutils.ParseCSR(req.GetCertificateSigningRequest())
	require.NoError(c.t, err)

	org := testOrg
	if c.org != "" {
		org = c.org
	}

	subject := pkix.Name{Organization: []string{org}}
	if req.GetSandboxIdentityIssuer() && !c.markDenied {
		subject.OrganizationalUnit = []string{CertificateMark}
	}

	der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: subject,
		NotBefore: time.Now().Add(-time.Second), NotAfter: time.Now().Add(10 * time.Minute),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
	}, c.ca.cert, csr.PublicKey, c.ca.key)
	require.NoError(c.t, err)

	leaf, err := cryptoutils.MarshalCertificateToPEM(&x509.Certificate{Raw: der})
	require.NoError(c.t, err)
	root, err := cryptoutils.MarshalCertificateToPEM(c.ca.cert)
	require.NoError(c.t, err)

	return &pb.GenerateSigningCertResponse{Chain: &pb.CertificateChain{Certificates: []string{string(leaf), string(root)}}}, nil
}

func TestIssuer(t *testing.T) {
	ca := newTestCA(t)

	t.Run("issues an identity that verifies", func(t *testing.T) {
		client := &fakeSigningClient{t: t, ca: ca}
		claims := validClaims(time.Time{})
		claims.IssuedAt = nil

		raw, err := NewIssuer(client).Issue(context.Background(), claims)
		require.NoError(t, err)
		assert.True(t, client.gotIssuer)

		id, err := Verify(raw, ca.trustedRoot(), validExpected())
		require.NoError(t, err)
		assert.NotNil(t, id.CommitVerification(testCommit))
		assert.Nil(t, claims.IssuedAt, "the claims of the caller do not change")
	})

	t.Run("a certificate without the mark gives no identity", func(t *testing.T) {
		_, err := NewIssuer(&fakeSigningClient{t: t, ca: ca, markDenied: true}).Issue(context.Background(), validClaims(time.Time{}))
		require.Error(t, err)
	})

	t.Run("a certificate for another organization gives no identity", func(t *testing.T) {
		_, err := NewIssuer(&fakeSigningClient{t: t, ca: ca, org: "another-org"}).Issue(context.Background(), validClaims(time.Time{}))
		require.ErrorContains(t, err, "not for the organization of the identity")
	})

	t.Run("a control plane error gives no identity", func(t *testing.T) {
		_, err := NewIssuer(&fakeSigningClient{t: t, ca: ca, err: errors.New("denied")}).Issue(context.Background(), validClaims(time.Time{}))
		require.Error(t, err)
	})

	t.Run("invalid claims give no identity", func(t *testing.T) {
		claims := validClaims(time.Time{})
		claims.ChainloopTokenID = ""

		_, err := NewIssuer(&fakeSigningClient{t: t, ca: ca}).Issue(context.Background(), claims)
		require.Error(t, err)
	})
}
