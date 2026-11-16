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

package biz_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"strings"
	"testing"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	ca2 "github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/attestation/sandboxidentity"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	fulcioca "github.com/sigstore/fulcio/pkg/ca"
	"github.com/sigstore/fulcio/pkg/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fixedSubjectCA is a CA that ignores the principal of the control plane, as a remote CA that
// builds the subject from the request does.
type fixedSubjectCA struct {
	*TestCA
	subject pkix.Name
}

type fixedPrincipal struct{ subject pkix.Name }

func (p fixedPrincipal) Name(_ context.Context) string { return "fixed" }

func (p fixedPrincipal) Embed(_ context.Context, cert *x509.Certificate) error {
	cert.Subject = p.subject
	return nil
}

func (e fixedSubjectCA) CreateCertificateFromCSR(ctx context.Context, _ identity.Principal, csr *x509.CertificateRequest) (*fulcioca.CodeSigningCertificate, error) {
	return e.ca.CreateCertificate(ctx, fixedPrincipal{e.subject}, csr.PublicKey)
}

func signingUseCaseWithCA(ca ca2.CertificateAuthority) *biz.SigningUseCase {
	return &biz.SigningUseCase{CAs: &ca2.CertificateAuthorities{CAs: []ca2.CertificateAuthority{ca}, SignerCA: ca}}
}

func parseChain(t *testing.T, chain []string) []*x509.Certificate {
	t.Helper()

	certs := make([]*x509.Certificate, 0, len(chain))
	for _, c := range chain {
		parsed, err := cryptoutils.UnmarshalCertificatesFromPEM([]byte(c))
		require.NoError(t, err)
		certs = append(certs, parsed...)
	}

	return certs
}

func TestCreateSigningCertSandboxIdentityMark(t *testing.T) {
	const orgID = "myorgid"

	t.Run("the certificate has the mark only when it is requested", func(t *testing.T) {
		uc := keylessSigningUseCase(t)
		_, csr, err := createCSR()
		require.NoError(t, err)

		marked, err := uc.CreateSigningCert(context.TODO(), orgID, csr, biz.WithSandboxIdentityMark())
		require.NoError(t, err)
		leaf := parseChain(t, marked)[0]
		assert.True(t, sandboxidentity.HasMark(leaf))
		assert.Equal(t, []string{orgID}, leaf.Subject.Organization)

		plain, err := uc.CreateSigningCert(context.TODO(), orgID, csr)
		require.NoError(t, err)
		assert.False(t, sandboxidentity.HasMark(parseChain(t, plain)[0]))
	})

	t.Run("a CA that cannot write the mark gives no certificate", func(t *testing.T) {
		testCA, err := NewTestCA()
		require.NoError(t, err)
		uc := signingUseCaseWithCA(fixedSubjectCA{TestCA: testCA, subject: pkix.Name{CommonName: "from the request"}})
		_, csr, err := createCSR()
		require.NoError(t, err)

		_, err = uc.CreateSigningCert(context.TODO(), orgID, csr, biz.WithSandboxIdentityMark())
		require.Error(t, err)
		assert.True(t, biz.IsErrNotImplemented(err))
	})

	t.Run("a mark that was not requested gives no certificate", func(t *testing.T) {
		testCA, err := NewTestCA()
		require.NoError(t, err)
		subject := pkix.Name{Organization: []string{orgID}, OrganizationalUnit: []string{sandboxidentity.CertificateMark}}
		uc := signingUseCaseWithCA(fixedSubjectCA{TestCA: testCA, subject: subject})
		_, csr, err := createCSR()
		require.NoError(t, err)

		_, err = uc.CreateSigningCert(context.TODO(), orgID, csr)
		require.Error(t, err)
	})
}

func TestVerifySandboxIdentity(t *testing.T) {
	const (
		commit   = "0123456789abcdef0123456789abcdef01234567"
		verified = "verified"
	)
	orgID, projectID, workflowID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	tokenID := uuid.New()
	uc := keylessSigningUseCase(t)

	issue := func(t *testing.T, opts ...biz.SigningCertOption) (*ecdsa.PrivateKey, []*x509.Certificate) {
		t.Helper()
		key, csr, err := createCSR()
		require.NoError(t, err)
		chain, err := uc.CreateSigningCert(context.TODO(), orgID, csr, opts...)
		require.NoError(t, err)
		return key, parseChain(t, chain)
	}

	claims := func() *sandboxidentity.Claims {
		return &sandboxidentity.Claims{
			RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now())},
			OrganizationID:   orgID, ProjectID: projectID, WorkflowID: workflowID,
			SandboxRun: "sandbox-run-1", ChainloopTokenID: tokenID.String(),
			Data: &sandboxidentity.Data{CommitVerification: &sandboxidentity.CommitVerification{
				Commit:       commit,
				Verification: &commitverification.CommitVerification{Attempted: true, Status: commitverification.VerificationStatusVerified},
			}},
		}
	}

	expected := &sandboxidentity.Expected{OrganizationID: orgID, ProjectID: projectID, WorkflowID: workflowID}

	key, chain := issue(t, biz.WithSandboxIdentityMark())
	raw, err := sandboxidentity.Sign(key, chain, claims())
	require.NoError(t, err)

	t.Run("a valid identity from the named token", func(t *testing.T) {
		got, err := uc.VerifySandboxIdentity(context.TODO(), raw, expected, tokenID)
		require.NoError(t, err)
		assert.Equal(t, tokenID, got.ChainloopTokenID)
		assert.Equal(t, "sandbox-run-1", got.SandboxRun)
		assert.True(t, got.HoldsCommitVerification(chainloop.GitHeadVerification{Commit: strings.ToUpper(commit), Status: verified}))
		assert.False(t, got.HoldsCommitVerification(chainloop.GitHeadVerification{Commit: commit, Status: "unverified"}))
		assert.False(t, got.HoldsCommitVerification(chainloop.GitHeadVerification{Commit: "fedcba9876543210fedcba9876543210fedcba98", Status: verified}))
	})

	t.Run("another caller token", func(t *testing.T) {
		_, err := uc.VerifySandboxIdentity(context.TODO(), raw, expected, uuid.New())
		require.Error(t, err)
		assert.True(t, biz.IsErrValidation(err))
	})

	t.Run("another workflow", func(t *testing.T) {
		other := *expected
		other.WorkflowID = uuid.NewString()
		_, err := uc.VerifySandboxIdentity(context.TODO(), raw, &other, tokenID)
		require.Error(t, err)
		assert.True(t, biz.IsErrValidation(err))
	})

	t.Run("a certificate without the permission has no mark", func(t *testing.T) {
		key, chain := issue(t)
		raw, err := sandboxidentity.Sign(key, chain, claims())
		require.NoError(t, err)

		_, err = uc.VerifySandboxIdentity(context.TODO(), raw, expected, tokenID)
		require.Error(t, err)
		assert.True(t, biz.IsErrValidation(err))
	})

	t.Run("a certificate from another CA", func(t *testing.T) {
		key, chain := issue(t, biz.WithSandboxIdentityMark())
		raw, err := sandboxidentity.Sign(key, chain, claims())
		require.NoError(t, err)

		_, err = keylessSigningUseCase(t).VerifySandboxIdentity(context.TODO(), raw, expected, tokenID)
		require.Error(t, err)
		assert.True(t, biz.IsErrValidation(err))
	})

	t.Run("no keyless signing", func(t *testing.T) {
		_, err := (&biz.SigningUseCase{}).VerifySandboxIdentity(context.TODO(), raw, expected, tokenID)
		require.Error(t, err)
		assert.True(t, biz.IsErrNotImplemented(err))
	})
}
