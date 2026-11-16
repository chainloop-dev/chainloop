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
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	repoM "github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/mocks"
	"github.com/chainloop-dev/chainloop/pkg/attestation"
	"github.com/chainloop-dev/chainloop/pkg/attestation/crafter/runners/commitverification"
	"github.com/chainloop-dev/chainloop/pkg/attestation/sandboxidentity"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	v1 "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// testBundleHead is the git.head commit of testdata/attestations/bundle.json
const testBundleHead = "25f40d9f4b911969c901377eb247f934dde21b9f"

// newSignedSandboxBundle signs the test attestation as one of the sandbox runner. It records an
// authenticated runner, and a verification with the given status and algorithm for the git.head commit.
func newSignedSandboxBundle(t *testing.T, signing *biz.SigningUseCase, orgID, verificationStatus, algorithm string) []byte {
	t.Helper()

	raw, err := os.ReadFile("testdata/attestations/bundle.json")
	require.NoError(t, err)
	env, err := attestation.DSSEEnvelopeFromBundleBytes(raw)
	require.NoError(t, err)
	payload, err := base64.StdEncoding.DecodeString(env.Payload)
	require.NoError(t, err)

	var statement map[string]any
	require.NoError(t, json.Unmarshal(payload, &statement))

	predicate := statement["predicate"].(map[string]any)
	predicate["runnerType"] = schemaapi.CraftingSchema_Runner_CHAINLOOP_SANDBOX.String()
	predicate["runnerAuthenticated"] = true

	for _, s := range statement["subject"].([]any) {
		subject := s.(map[string]any)
		if subject["name"] == "git.head" && verificationStatus != "" {
			annotations := subject["annotations"].(map[string]any)
			annotations["author.verification_status"] = verificationStatus
			if algorithm != "" {
				annotations["signature.algorithm"] = algorithm
			}
		}
	}

	payload, err = json.Marshal(statement)
	require.NoError(t, err)

	key, csr, err := createCSR()
	require.NoError(t, err)
	chain, err := signing.CreateSigningCert(context.Background(), orgID, csr)
	require.NoError(t, err)

	digest := sha256.Sum256(dsse.PAE(env.PayloadType, payload))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	require.NoError(t, err)

	bundle, err := attestation.BundleFromDSSEEnvelope(&dsse.Envelope{
		PayloadType: env.PayloadType,
		Payload:     base64.StdEncoding.EncodeToString(payload),
		Signatures:  []dsse.Signature{{Sig: base64.StdEncoding.EncodeToString(sig)}},
	})
	require.NoError(t, err)

	block, _ := pem.Decode([]byte(chain[0]))
	require.NotNil(t, block)
	bundle.VerificationMaterial.Content = &protobundle.VerificationMaterial_Certificate{Certificate: &v1.X509Certificate{RawBytes: block.Bytes}}

	out, err := protojson.Marshal(bundle)
	require.NoError(t, err)

	return out
}

// The control plane enforces the sandbox identity on the store path, for any CLI (R-006).
func TestValidateAttestationContractEnforcesSandboxIdentity(t *testing.T) {
	const (
		verified     = "verified"
		commitClaims = "commit verification"
	)

	orgID := uuid.New()
	signing := keylessSigningUseCase(t)

	holding := func(commit string) *biz.SandboxIdentity {
		return &biz.SandboxIdentity{ChainloopTokenID: uuid.New(), Data: &sandboxidentity.Data{CommitVerification: &sandboxidentity.CommitVerification{
			Commit:       commit,
			Verification: &commitverification.CommitVerification{Attempted: true, Status: commitverification.VerificationStatusVerified, SignatureAlgorithm: "PGP"},
		}}}
	}

	cases := []struct {
		name      string
		identity  *biz.SandboxIdentity
		status    string
		algorithm string
		// empty when the attestation passes the sandbox check and stops at the contract check
		wantErr string
	}{
		{name: "an authenticated runner without an identity", wantErr: "authenticated sandbox runner"},
		{name: "a commit verification that the identity holds", identity: holding(testBundleHead), status: verified, algorithm: "PGP"},
		{name: "a commit verification with another signature algorithm", identity: holding(testBundleHead), status: verified, algorithm: "SSH", wantErr: commitClaims},
		{name: "a commit verification for another commit", identity: holding("0123456789abcdef0123456789abcdef01234567"), status: verified, wantErr: commitClaims},
		{name: "a commit verification with another status", identity: holding(testBundleHead), status: "unverified", wantErr: commitClaims},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := repoM.NewWorkflowRunRepo(t)
			uc, err := biz.NewWorkflowRunUseCase(&biz.WorkflowRunUseCaseOpts{WfrRepo: repo, SigningUC: signing})
			require.NoError(t, err)

			runID := uuid.New()
			run := &biz.WorkflowRun{ID: runID, Workflow: &biz.Workflow{OrgID: orgID}, RunnerType: schemaapi.CraftingSchema_Runner_CHAINLOOP_SANDBOX.String(), SandboxIdentity: tc.identity}
			repo.On("FindByID", mock.Anything, runID).Return(run, nil)

			err = uc.ValidateAttestationContract(context.Background(), runID.String(), newSignedSandboxBundle(t, signing, orgID.String(), tc.status, tc.algorithm))
			require.Error(t, err)
			assert.True(t, biz.IsErrValidation(err), "unexpected error type: %v", err)

			if tc.wantErr == "" {
				assert.Contains(t, err.Error(), "no contract revision")
				return
			}

			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
