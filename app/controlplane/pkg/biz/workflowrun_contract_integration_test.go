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
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	creds "github.com/chainloop-dev/chainloop/pkg/credentials/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// The testdata attestations carry these contract material names:
//
//	full.json        -> image, skynet-sbom, skynet2-sbom
//	empty.json       -> (none)
//	with-string.json -> build-ref
const (
	attestationFull  = "testdata/attestations/full.json"
	attestationEmpty = "testdata/attestations/empty.json"
)

// The contract is what the organization agreed the run had to produce. It has
// always been checked by the CLI before pushing, but the CLI also signs the
// bundle, so a client that skips the check still produces a valid attestation.
// These tests pin the control plane down as the authority.
type workflowRunContractIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org        *biz.Organization
	casBackend *biz.CASBackend
}

func (s *workflowRunContractIntegrationTestSuite) SetupTest() {
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On("SaveCredentials", mock.Anything, mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).
		Return("stored-OCI-secret", nil).Maybe()

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	ctx := context.Background()
	var err error
	s.org, err = s.Organization.Create(ctx, "contract-enforcement-org")
	require.NoError(s.T(), err)

	s.casBackend, err = s.CASBackend.CreateOrUpdate(ctx, s.org.ID, "repo", "username", "pass", backendType, true)
	require.NoError(s.T(), err)
}

// newRunWithContract creates a workflow bound to a contract carrying the given
// materials block and returns a run pinned to that contract revision.
func (s *workflowRunContractIntegrationTestSuite) newRunWithContract(name, materialsYAML string) *biz.WorkflowRun {
	s.T().Helper()
	ctx := context.Background()

	rawContract := fmt.Sprintf(`apiVersion: chainloop.dev/v1
kind: Contract
metadata:
  name: %s
spec:
%s`, name, materialsYAML)

	contract, err := s.WorkflowContract.Create(ctx, &biz.WorkflowContractCreateOpts{
		OrgID: s.org.ID, Name: name, RawSchema: []byte(rawContract),
	})
	require.NoError(s.T(), err)

	wf, err := s.Workflow.Create(ctx, &biz.WorkflowCreateOpts{
		Name: name, OrgID: s.org.ID, Project: "test-project", ContractID: contract.ID.String(),
	})
	require.NoError(s.T(), err)

	contractVersion, err := s.WorkflowContract.Describe(ctx, s.org.ID, wf.ContractID.String(), 0)
	require.NoError(s.T(), err)

	run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
		WorkflowID: wf.ID.String(), ContractRevision: contractVersion, CASBackendID: s.casBackend.ID,
	})
	require.NoError(s.T(), err)

	return run
}

func (s *workflowRunContractIntegrationTestSuite) TestSaveAttestationEnforcesContract() {
	testCases := []struct {
		name string
		// the spec's materials block, already indented under spec:
		materials   string
		attestation string
		wantErr     string
	}{
		{
			name:        "contract with no materials accepts any attestation",
			materials:   "  materials: []",
			attestation: attestationFull,
		},
		{
			name: "every required material present",
			materials: `  materials:
    - type: CONTAINER_IMAGE
      name: image
    - type: SBOM_CYCLONEDX_JSON
      name: skynet-sbom`,
			attestation: attestationFull,
		},
		{
			name: "required material missing is rejected",
			materials: `  materials:
    - type: CONTAINER_IMAGE
      name: image
    - type: SARIF
      name: static-analysis`,
			attestation: attestationFull,
			wantErr:     "some materials have not been crafted yet: static-analysis",
		},
		{
			name: "attestation with no materials at all is rejected",
			materials: `  materials:
    - type: CONTAINER_IMAGE
      name: image`,
			attestation: attestationEmpty,
			wantErr:     "some materials have not been crafted yet: image",
		},
		{
			name: "optional material missing is accepted",
			materials: `  materials:
    - type: SARIF
      name: static-analysis
      optional: true`,
			attestation: attestationEmpty,
		},
		{
			name: "choke group satisfied by one member",
			materials: `  materials:
    - type: SBOM_SPDX_JSON
      name: spdx-sbom
      group: sbom
    - type: SBOM_CYCLONEDX_JSON
      name: skynet-sbom
      group: sbom`,
			attestation: attestationFull,
		},
		{
			name: "choke group with no member crafted is rejected",
			materials: `  materials:
    - type: SBOM_SPDX_JSON
      name: spdx-sbom
      group: sbom
    - type: SBOM_CYCLONEDX_JSON
      name: cyclonedx-sbom
      group: sbom`,
			attestation: attestationFull,
			wantErr:     `at least one material from group "sbom" is required: spdx-sbom, cyclonedx-sbom`,
		},
		{
			// Materials the contract never declared are legitimate: runtime
			// evidence and exploded archives both produce undeclared names.
			name: "materials beyond the contract are accepted",
			materials: `  materials:
    - type: CONTAINER_IMAGE
      name: image`,
			attestation: attestationFull,
		},
	}

	for i, tc := range testCases {
		s.Run(tc.name, func() {
			ctx := context.Background()
			run := s.newRunWithContract(fmt.Sprintf("contract-%d", i), tc.materials)
			bundleBytes := testhelpers.BundleBytesFromEnvelope(s.T(), tc.attestation)

			digest, err := s.WorkflowRun.SaveAttestation(ctx, run.ID.String(), bundleBytes)

			if tc.wantErr == "" {
				require.NoError(s.T(), err)
				require.NotNil(s.T(), digest)
				return
			}

			require.Error(s.T(), err)
			s.True(biz.IsErrValidation(err), "expected a validation error, got %T: %v", err, err)
			s.ErrorContains(err, tc.wantErr)
			// The revision the run was pinned to is named so the operator knows
			// which contract the attestation was measured against.
			s.ErrorContains(err, fmt.Sprintf("contract revision %d", run.ContractRevisionUsed))
		})
	}
}

// A rejected attestation must leave no trace: the evidence is not persisted and
// the run stays without an attestation rather than half-recording one.
func (s *workflowRunContractIntegrationTestSuite) TestRejectedAttestationIsNotPersisted() {
	ctx := context.Background()
	run := s.newRunWithContract("atomic-contract", `  materials:
    - type: SARIF
      name: static-analysis`)

	bundleBytes := testhelpers.BundleBytesFromEnvelope(s.T(), attestationFull)
	_, err := s.WorkflowRun.SaveAttestation(ctx, run.ID.String(), bundleBytes)
	require.Error(s.T(), err)

	stored, err := s.WorkflowRun.GetByIDInOrg(ctx, s.org.ID, run.ID.String())
	require.NoError(s.T(), err)
	// The run always carries an Attestation holder; what matters is that it was
	// left empty, so neither the digest nor the bundle bytes were recorded.
	s.Empty(stored.Attestation.Digest, "a rejected attestation must not record a digest on the run")
	s.Nil(stored.Attestation.Bundle, "a rejected attestation must not persist its bundle")
	s.Nil(stored.Attestation.Envelope, "a rejected attestation must not persist its envelope")
}

// The synchronous CAS path uploads the bundle before SaveAttestation runs, so it
// needs a check it can run first. This must reach the same verdict as the one
// inside SaveAttestation while leaving the run untouched.
func (s *workflowRunContractIntegrationTestSuite) TestValidateAttestationContractIsSideEffectFree() {
	ctx := context.Background()

	s.Run("rejects a violating attestation without touching the run", func() {
		run := s.newRunWithContract("preflight-reject", `  materials:
    - type: SARIF
      name: static-analysis`)
		bundleBytes := testhelpers.BundleBytesFromEnvelope(s.T(), attestationFull)

		err := s.WorkflowRun.ValidateAttestationContract(ctx, run.ID.String(), bundleBytes)
		require.Error(s.T(), err)
		s.True(biz.IsErrValidation(err), "expected a validation error, got %T: %v", err, err)
		s.ErrorContains(err, "some materials have not been crafted yet: static-analysis")

		stored, err := s.WorkflowRun.GetByIDInOrg(ctx, s.org.ID, run.ID.String())
		require.NoError(s.T(), err)
		s.Empty(stored.Attestation.Digest, "the preflight check must not record anything")
	})

	s.Run("accepts a satisfying attestation and still allows the save", func() {
		run := s.newRunWithContract("preflight-accept", `  materials:
    - type: CONTAINER_IMAGE
      name: image`)
		bundleBytes := testhelpers.BundleBytesFromEnvelope(s.T(), attestationFull)

		require.NoError(s.T(), s.WorkflowRun.ValidateAttestationContract(ctx, run.ID.String(), bundleBytes))

		digest, err := s.WorkflowRun.SaveAttestation(ctx, run.ID.String(), bundleBytes)
		require.NoError(s.T(), err)
		s.NotNil(digest)
	})

	s.Run("reports an unknown run rather than passing it through", func() {
		bundleBytes := testhelpers.BundleBytesFromEnvelope(s.T(), attestationFull)
		err := s.WorkflowRun.ValidateAttestationContract(ctx, uuid.NewString(), bundleBytes)
		require.Error(s.T(), err)
		s.True(biz.IsNotFound(err), "expected a not-found error, got %T: %v", err, err)
	})
}

func TestWorkflowRunContractEnforcement(t *testing.T) {
	suite.Run(t, new(workflowRunContractIntegrationTestSuite))
}
