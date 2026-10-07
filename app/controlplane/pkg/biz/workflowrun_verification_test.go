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
	"io"
	"os"
	"testing"

	conf "github.com/chainloop-dev/chainloop/app/controlplane/internal/conf/controlplane/config/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	repoM "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	ca2 "github.com/chainloop-dev/chainloop/app/controlplane/pkg/ca"
	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	v1 "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// keylessSigningUseCase returns a signing use case backed by an ephemeral CA,
// so keyless signing is enabled, and verification is forced (the default).
func keylessSigningUseCase(t *testing.T) *biz.SigningUseCase {
	t.Helper()
	ca, err := NewTestCA()
	require.NoError(t, err)
	return &biz.SigningUseCase{CAs: &ca2.CertificateAuthorities{CAs: []ca2.CertificateAuthority{ca}, SignerCA: ca}, ForceVerification: true}
}

// withoutForcedVerification returns a copy of the signing use case, with the
// same certificate authorities, that does not force verification.
func withoutForcedVerification(uc *biz.SigningUseCase) *biz.SigningUseCase {
	optOut := *uc
	optOut.ForceVerification = false
	return &optOut
}

type testBundleKind int

const (
	// signed with a keyless certificate that carries the certificate
	bundleWithCert testBundleKind = iota
	// signed, but without verification material, as cosign key or SignServer signers do
	bundleWithoutMaterial
	// a raw DSSE envelope instead of a Sigstore bundle
	bundleRawEnvelope
	// carries a valid keyless certificate, but the signature does not match the payload
	bundleWithCertTamperedSignature
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

	signedPayload := payload
	if kind == bundleWithCertTamperedSignature {
		// sign other content, so the signature does not match the payload in the envelope
		signedPayload = append([]byte("tampered"), payload...)
	}

	digest := sha256.Sum256(dsse.PAE(env.PayloadType, signedPayload))
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

	if kind == bundleWithCert || kind == bundleWithCertTamperedSignature {
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

	optOut := withoutForcedVerification(signing)

	cases := []struct {
		name   string
		bundle []byte
		// verification not forced
		optOut bool
		// signature check is expected to reject the attestation
		wantRejected bool
	}{
		{
			name:   "keyless certificate issued to the run organization",
			bundle: newSignedTestBundle(t, signing, orgID.String(), bundleWithCert),
		},
		{
			name:   "opt-out: signed without verification material is accepted",
			bundle: newSignedTestBundle(t, signing, orgID.String(), bundleWithoutMaterial),
			optOut: true,
		},
		{
			name:   "opt-out: keyless certificate issued to another organization is accepted",
			bundle: newSignedTestBundle(t, signing, otherOrgID.String(), bundleWithCert),
			optOut: true,
		},
		{
			name:         "opt-out: keyless certificate with a tampered signature is still rejected",
			bundle:       newSignedTestBundle(t, signing, orgID.String(), bundleWithCertTamperedSignature),
			optOut:       true,
			wantRejected: true,
		},
		{
			name:         "keyless certificate issued to another organization",
			bundle:       newSignedTestBundle(t, signing, otherOrgID.String(), bundleWithCert),
			wantRejected: true,
		},
		{
			name:         "keyless certificate issued to the run organization, with a tampered signature",
			bundle:       newSignedTestBundle(t, signing, orgID.String(), bundleWithCertTamperedSignature),
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
			signingUC := signing
			if tc.optOut {
				signingUC = optOut
			}

			repo := repoM.NewWorkflowRunRepo(t)
			uc, err := biz.NewWorkflowRunUseCase(&biz.WorkflowRunUseCaseOpts{WfrRepo: repo, SigningUC: signingUC})
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
		digest     string
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
			name:       "keyless certificate issued to the run organization, with a tampered signature",
			signing:    signing,
			bundle:     newSignedTestBundle(t, signing, orgID.String(), bundleWithCertTamperedSignature),
			wantReason: "validating the DSSE envelope",
		},
		{
			name:       "attestation digest recorded, but its bundle could not be retrieved",
			signing:    signing,
			digest:     "sha256:0f9b2a1c",
			wantReason: "could not be retrieved",
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
		{
			name:    "opt-out: no verification material means verification does not apply",
			signing: withoutForcedVerification(signing),
			bundle:  newSignedTestBundle(t, signing, orgID.String(), bundleWithoutMaterial),
			wantNil: true,
		},
		{
			name:       "opt-out: keyless certificate with a tampered signature is not verified",
			signing:    withoutForcedVerification(signing),
			bundle:     newSignedTestBundle(t, signing, orgID.String(), bundleWithCertTamperedSignature),
			wantReason: "validating the DSSE envelope",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, err := biz.NewWorkflowRunUseCase(&biz.WorkflowRunUseCaseOpts{SigningUC: tc.signing})
			require.NoError(t, err)

			got, err := uc.VerifyRun(context.Background(), &biz.WorkflowRun{
				Workflow:    &biz.Workflow{OrgID: orgID},
				Attestation: &biz.Attestation{Bundle: tc.bundle, Digest: tc.digest},
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

func TestSigningUseCaseVerificationEnforced(t *testing.T) {
	cases := []struct {
		name        string
		uc          *biz.SigningUseCase
		wantKeyless bool
		wantForced  bool
	}{
		{name: "certificate authorities configured, verification forced", uc: keylessSigningUseCase(t), wantKeyless: true, wantForced: true},
		{name: "certificate authorities configured, opt-out", uc: withoutForcedVerification(keylessSigningUseCase(t)), wantKeyless: true},
		{name: "no certificate authorities", uc: &biz.SigningUseCase{ForceVerification: true}},
		{name: "nil use case", uc: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantKeyless, tc.uc.KeylessEnabled())
			assert.Equal(t, tc.wantForced, tc.uc.VerificationEnforced())
		})
	}
}

func TestNewChainloopSigningUseCaseForceVerification(t *testing.T) {
	cases := []struct {
		name   string
		config *conf.Bootstrap
		want   bool
	}{
		{name: "defaults to true when unset", config: &conf.Bootstrap{}, want: true},
		{name: "defaults to true when the attestations section has no value", config: &conf.Bootstrap{Attestations: &conf.Attestations{}}, want: true},
		{name: "explicitly enabled", config: &conf.Bootstrap{Attestations: &conf.Attestations{ForceVerification: proto.Bool(true)}}, want: true},
		{name: "explicitly disabled", config: &conf.Bootstrap{Attestations: &conf.Attestations{ForceVerification: proto.Bool(false)}}, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uc, err := biz.NewChainloopSigningUseCase(tc.config, log.NewStdLogger(io.Discard))
			require.NoError(t, err)
			assert.Equal(t, tc.want, uc.ForceVerification)
		})
	}
}
