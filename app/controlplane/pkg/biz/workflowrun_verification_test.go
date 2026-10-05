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
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	repoM "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	ca2 "github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	v1 "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// keylessSigningUseCase returns a signing use case backed by an ephemeral CA,
// so keyless signing is enabled.
func keylessSigningUseCase(t *testing.T) *biz.SigningUseCase {
	t.Helper()
	ca, err := NewTestCA()
	require.NoError(t, err)
	return &biz.SigningUseCase{CAs: &ca2.CertificateAuthorities{CAs: []ca2.CertificateAuthority{ca}, SignerCA: ca}}
}

type testBundleKind int

const (
	// signed with a keyless certificate that carries the certificate
	bundleWithCert testBundleKind = iota
	// signed, but without verification material, as cosign key or SignServer signers do
	bundleWithoutMaterial
	// a raw DSSE envelope instead of a Sigstore bundle
	bundleRawEnvelope
)

// newSignedTestBundle signs the test attestation with a certificate issued by
// the signing use case to the given organization.
func newSignedTestBundle(t *testing.T, signing *biz.SigningUseCase, orgID string, kind testBundleKind) []byte {
	t.Helper()

	raw, err := os.ReadFile("testdata/attestations/bundle.json")
	require.NoError(t, err)
	env, err := attestation.DSSEEnvelopeFromBundleBytes(raw)
	require.NoError(t, err)
	payload, err := base64.StdEncoding.DecodeString(env.Payload)
	require.NoError(t, err)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: "ephemeral certificate"}}, key)
	require.NoError(t, err)
	chain, err := signing.CreateSigningCert(context.Background(), orgID, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrDER}))
	require.NoError(t, err)

	digest := sha256.Sum256(dsse.PAE(env.PayloadType, payload))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	require.NoError(t, err)

	signed := &dsse.Envelope{
		PayloadType: env.PayloadType,
		Payload:     env.Payload,
		Signatures:  []dsse.Signature{{Sig: base64.StdEncoding.EncodeToString(sig)}},
	}

	if kind == bundleRawEnvelope {
		out, err := json.Marshal(signed)
		require.NoError(t, err)
		return out
	}

	bundle, err := attestation.BundleFromDSSEEnvelope(signed)
	require.NoError(t, err)

	if kind == bundleWithCert {
		block, _ := pem.Decode([]byte(chain[0]))
		require.NotNil(t, block)
		bundle.VerificationMaterial.Content = &protobundle.VerificationMaterial_Certificate{
			Certificate: &v1.X509Certificate{RawBytes: block.Bytes},
		}
	}

	out, err := protojson.Marshal(bundle)
	require.NoError(t, err)
	return out
}

func TestValidateAttestationContractEnforcesKeylessVerification(t *testing.T) {
	orgID := uuid.New()
	otherOrgID := uuid.New()
	signing := keylessSigningUseCase(t)

	cases := []struct {
		name   string
		bundle []byte
		// signature check is expected to reject the attestation
		wantRejected bool
	}{
		{
			name:   "keyless certificate issued to the run organization",
			bundle: newSignedTestBundle(t, signing, orgID.String(), bundleWithCert),
		},
		{
			name:         "keyless certificate issued to another organization",
			bundle:       newSignedTestBundle(t, signing, otherOrgID.String(), bundleWithCert),
			wantRejected: true,
		},
		{
			name:         "signed without verification material",
			bundle:       newSignedTestBundle(t, signing, orgID.String(), bundleWithoutMaterial),
			wantRejected: true,
		},
		{
			name:         "raw DSSE envelope",
			bundle:       newSignedTestBundle(t, signing, orgID.String(), bundleRawEnvelope),
			wantRejected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoM.NewWorkflowRunRepo(t)
			uc, err := biz.NewWorkflowRunUseCase(&biz.WorkflowRunUseCaseOpts{WfrRepo: repo, SigningUC: signing})
			require.NoError(t, err)

			runID := uuid.New()
			// the run has no contract revision, so an attestation that passes
			// the signature check is stopped at the contract check instead
			repo.On("FindByID", mock.Anything, runID).Return(&biz.WorkflowRun{
				ID: runID, Workflow: &biz.Workflow{OrgID: orgID},
			}, nil)

			err = uc.ValidateAttestationContract(context.Background(), runID.String(), tc.bundle)
			require.Error(t, err)
			assert.True(t, biz.IsErrValidation(err), "unexpected error type: %v", err)
			if tc.wantRejected {
				assert.Contains(t, err.Error(), "attestation verification failed")
				return
			}
			assert.Contains(t, err.Error(), "no contract revision")
		})
	}
}

func TestVerifyRunKeyless(t *testing.T) {
	orgID := uuid.New()
	signing := keylessSigningUseCase(t)

	cases := []struct {
		name       string
		signing    *biz.SigningUseCase
		bundle     []byte
		wantNil    bool
		wantResult bool
		wantReason string
	}{
		{
			name:       "keyless certificate issued to the run organization",
			signing:    signing,
			bundle:     newSignedTestBundle(t, signing, orgID.String(), bundleWithCert),
			wantResult: true,
		},
		{
			name:       "keyless certificate issued to another organization",
			signing:    signing,
			bundle:     newSignedTestBundle(t, signing, uuid.NewString(), bundleWithCert),
			wantReason: "organization mismatch",
		},
		{
			name:       "no verification material with keyless signing enabled",
			signing:    signing,
			bundle:     newSignedTestBundle(t, signing, orgID.String(), bundleWithoutMaterial),
			wantReason: "no verification material",
		},
		{
			name:    "keyless signing not configured",
			signing: &biz.SigningUseCase{},
			bundle:  newSignedTestBundle(t, signing, orgID.String(), bundleWithoutMaterial),
			wantNil: true,
		},
		{
			name:    "run without attestation",
			signing: signing,
			wantNil: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, err := biz.NewWorkflowRunUseCase(&biz.WorkflowRunUseCaseOpts{SigningUC: tc.signing})
			require.NoError(t, err)

			got, err := uc.VerifyRun(context.Background(), &biz.WorkflowRun{
				Workflow:    &biz.Workflow{OrgID: orgID},
				Attestation: &biz.Attestation{Bundle: tc.bundle},
			})
			require.NoError(t, err)
			if tc.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tc.wantResult, got.Result)
			assert.Contains(t, got.FailureReason, tc.wantReason)
		})
	}
}

func TestSigningUseCaseKeylessEnabled(t *testing.T) {
	cases := []struct {
		name string
		uc   *biz.SigningUseCase
		want bool
	}{
		{name: "certificate authorities configured", uc: keylessSigningUseCase(t), want: true},
		{name: "no certificate authorities", uc: &biz.SigningUseCase{}, want: false},
		{name: "nil use case", uc: nil, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.uc.KeylessEnabled())
		})
	}
}
