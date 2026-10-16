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

package service

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"testing"
	"time"

	pb "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/usercontext"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/usercontext/entities"
	"github.com/chainloop-dev/chainloop/pkg/attestation/sandboxidentity"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	fulcioca "github.com/sigstore/fulcio/pkg/ca"
	"github.com/sigstore/fulcio/pkg/ca/ephemeralca"
	"github.com/sigstore/fulcio/pkg/identity"
	"github.com/sigstore/sigstore/pkg/cryptoutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type ephemeralTestCA struct {
	ca *ephemeralca.EphemeralCA
}

func (e ephemeralTestCA) CreateCertificateFromCSR(ctx context.Context, principal identity.Principal, csr *x509.CertificateRequest) (*fulcioca.CodeSigningCertificate, error) {
	return e.ca.CreateCertificate(ctx, principal, csr.PublicKey)
}

func (e ephemeralTestCA) GetRootChain(ctx context.Context) ([]*x509.Certificate, error) {
	tb, err := e.ca.TrustBundle(ctx)
	if err != nil {
		return nil, err
	}

	return tb[0], nil
}

func (e ephemeralTestCA) GetName() string {
	return "test"
}

func newKeylessSigningUseCase(t *testing.T) *biz.SigningUseCase {
	t.Helper()

	eca, err := ephemeralca.NewEphemeralCA()
	require.NoError(t, err)
	authority := ephemeralTestCA{ca: eca}

	return &biz.SigningUseCase{CAs: &ca.CertificateAuthorities{CAs: []ca.CertificateAuthority{authority}, SignerCA: authority}}
}

func newCSR(t *testing.T) (*ecdsa.PrivateKey, []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "ephemeral"}}, key)
	require.NoError(t, err)

	return key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der})
}

func parsePEMChain(t *testing.T, chain []string) []*x509.Certificate {
	t.Helper()

	certs := make([]*x509.Certificate, 0, len(chain))
	for _, c := range chain {
		parsed, err := cryptoutils.UnmarshalCertificatesFromPEM([]byte(c))
		require.NoError(t, err)
		certs = append(certs, parsed...)
	}

	return certs
}

// apiTokenContext is the context of a call with an API token, as the API token middleware sets it.
func apiTokenContext(orgID string, tokenID uuid.UUID) context.Context {
	ctx := entities.WithCurrentAPIToken(context.Background(), &entities.APIToken{ID: tokenID.String()})
	ctx = usercontext.WithAuthzSubject(ctx, (&authz.SubjectAPIToken{ID: tokenID.String()}).String())

	return usercontext.WithRobotAccount(ctx, &usercontext.RobotAccount{OrgID: orgID})
}

func TestGenerateSigningCertSandboxIdentityIssuer(t *testing.T) {
	const orgID = "my-org"
	tokenID := uuid.New()
	_, csr := newCSR(t)

	newSvc := func(t *testing.T, policies []*authz.Policy) *SigningService {
		return &SigningService{service: newTestServiceWithTokenPolicies(t, policies), signing: newKeylessSigningUseCase(t)}
	}

	t.Run("a token with the permission gets a marked certificate", func(t *testing.T) {
		svc := newSvc(t, []*authz.Policy{authz.PolicyArtifactUpload, authz.PolicySandboxIdentityIssue})

		resp, err := svc.GenerateSigningCert(apiTokenContext(orgID, tokenID), &pb.GenerateSigningCertRequest{CertificateSigningRequest: csr, SandboxIdentityIssuer: true})
		require.NoError(t, err)
		assert.True(t, sandboxidentity.HasMark(parsePEMChain(t, resp.GetChain().GetCertificates())[0]))
	})

	t.Run("a token without the permission is denied", func(t *testing.T) {
		svc := newSvc(t, []*authz.Policy{authz.PolicyArtifactUpload})

		_, err := svc.GenerateSigningCert(apiTokenContext(orgID, tokenID), &pb.GenerateSigningCertRequest{CertificateSigningRequest: csr, SandboxIdentityIssuer: true})
		require.Error(t, err)
		assert.True(t, kerrors.IsForbidden(err))
	})

	t.Run("a caller without an authorization subject is denied", func(t *testing.T) {
		svc := newSvc(t, []*authz.Policy{authz.PolicySandboxIdentityIssue})
		ctx := usercontext.WithRobotAccount(context.Background(), &usercontext.RobotAccount{OrgID: orgID})

		_, err := svc.GenerateSigningCert(ctx, &pb.GenerateSigningCertRequest{CertificateSigningRequest: csr, SandboxIdentityIssuer: true})
		require.Error(t, err)
		assert.True(t, kerrors.IsForbidden(err))
	})

	t.Run("a request without the flag gets a certificate without the mark", func(t *testing.T) {
		svc := newSvc(t, []*authz.Policy{authz.PolicyArtifactUpload})

		resp, err := svc.GenerateSigningCert(apiTokenContext(orgID, tokenID), &pb.GenerateSigningCertRequest{CertificateSigningRequest: csr})
		require.NoError(t, err)
		assert.False(t, sandboxidentity.HasMark(parsePEMChain(t, resp.GetChain().GetCertificates())[0]))
	})
}

func TestAttestationInitVerifySandboxIdentity(t *testing.T) {
	orgID := uuid.NewString()
	tokenID := uuid.New()
	wf := &biz.Workflow{ID: uuid.New(), ProjectID: uuid.New()}
	signing := newKeylessSigningUseCase(t)

	key, csr := newCSR(t)
	chain, err := signing.CreateSigningCert(context.Background(), orgID, csr, biz.WithSandboxIdentityMark())
	require.NoError(t, err)

	raw, err := sandboxidentity.Sign(key, parsePEMChain(t, chain), &sandboxidentity.Claims{
		RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now())},
		OrganizationID:   orgID, ProjectID: wf.ProjectID.String(), WorkflowID: wf.ID.String(),
		SandboxRun: "sandbox-run-1", ChainloopTokenID: tokenID.String(),
	})
	require.NoError(t, err)

	svc := &AttestationService{service: newTestServiceWithTokenPolicies(t, nil), signingUseCase: signing}
	sandbox := schemaapi.CraftingSchema_Runner_CHAINLOOP_SANDBOX

	t.Run("a valid identity from the named token", func(t *testing.T) {
		id, result := svc.verifySandboxIdentity(apiTokenContext(orgID, tokenID), raw, sandbox, orgID, wf)
		require.NotNil(t, id)
		assert.True(t, result.GetVerified())
		assert.Equal(t, tokenID, id.ChainloopTokenID)
		assert.Equal(t, "sandbox-run-1", id.SandboxRun)
	})

	testCases := []struct {
		name    string
		ctx     context.Context
		runner  schemaapi.CraftingSchema_Runner_RunnerType
		wf      *biz.Workflow
		svc     *AttestationService
		wantMsg string
	}{
		{name: "another runner", ctx: apiTokenContext(orgID, tokenID), runner: schemaapi.CraftingSchema_Runner_GITHUB_ACTION, wantMsg: "CHAINLOOP_SANDBOX"},
		{name: "a caller without an API token", ctx: context.Background(), runner: sandbox, wantMsg: "API token"},
		{name: "another caller token", ctx: apiTokenContext(orgID, uuid.New()), runner: sandbox, wantMsg: "another Chainloop token"},
		{name: "another workflow", ctx: apiTokenContext(orgID, tokenID), runner: sandbox, wf: &biz.Workflow{ID: uuid.New(), ProjectID: wf.ProjectID}, wantMsg: "another workflow"},
		{
			name: "no keyless signing", ctx: apiTokenContext(orgID, tokenID), runner: sandbox,
			svc: &AttestationService{service: newTestServiceWithTokenPolicies(t, nil), signingUseCase: &biz.SigningUseCase{}}, wantMsg: "keyless",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			target, workflow := svc, wf
			if tc.svc != nil {
				target = tc.svc
			}

			if tc.wf != nil {
				workflow = tc.wf
			}

			id, result := target.verifySandboxIdentity(tc.ctx, raw, tc.runner, orgID, workflow)
			assert.Nil(t, id)
			assert.False(t, result.GetVerified())
			assert.Contains(t, result.GetReason(), tc.wantMsg)
		})
	}
}
