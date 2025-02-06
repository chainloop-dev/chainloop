//
// Copyright 2024 The Chainloop Authors.
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
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	attestation2 "github.com/chainloop-dev/chainloop/app/controlplane/pkg/data/ent/attestation"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/pagination"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	creds "github.com/chainloop-dev/chainloop/pkg/credentials/mocks"
	v2 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	v1 "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/encoding/protojson"
)

func (s *workflowRunIntegrationTestSuite) TestList() {
	// Create a finished run
	finishedRun, err := s.WorkflowRun.Create(context.Background(),
		&biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg2.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
		})
	s.NoError(err)
	err = s.WorkflowRun.MarkAsFinished(context.Background(), finishedRun.ID.String(), biz.WorkflowRunSuccess, "")
	s.NoError(err)

	testCases := []struct {
		name    string
		filters *biz.RunListFilters
		want    []*biz.WorkflowRun
		wantErr bool
	}{
		{
			name:    "no filters",
			filters: &biz.RunListFilters{},
			want:    []*biz.WorkflowRun{s.runOrg2, s.runOrg2Public, finishedRun},
		},
		{
			name:    "filter by workflow",
			filters: &biz.RunListFilters{WorkflowID: &s.workflowOrg2.ID},
			want:    []*biz.WorkflowRun{s.runOrg2, finishedRun},
		},
		{
			name:    "filter by status, no result",
			filters: &biz.RunListFilters{Status: biz.WorkflowRunCancelled},
			want:    []*biz.WorkflowRun{},
		},
		{
			name:    "filter by status, 2 results",
			filters: &biz.RunListFilters{Status: biz.WorkflowRunInitialized},
			want:    []*biz.WorkflowRun{s.runOrg2, s.runOrg2Public},
		},
		{
			name:    "filter by finished state and workflow with results",
			filters: &biz.RunListFilters{Status: biz.WorkflowRunSuccess, WorkflowID: &s.workflowOrg2.ID},
			want:    []*biz.WorkflowRun{finishedRun},
		},
		{
			name:    "can not filter by workflow and version",
			filters: &biz.RunListFilters{VersionID: &s.version2.ID, WorkflowID: &s.workflowOrg2.ID},
			wantErr: true,
		},
		{
			name:    "filter by version no results",
			filters: &biz.RunListFilters{VersionID: &s.casBackend.ID}, // providing a random ID
			want:    []*biz.WorkflowRun{},
		},
		{
			name:    "filter by version with results",
			filters: &biz.RunListFilters{VersionID: &s.version1.ID},
			want:    []*biz.WorkflowRun{s.runOrg2},
		},
		{
			name:    "filter by version with results",
			filters: &biz.RunListFilters{VersionID: &s.version2.ID},
			want:    []*biz.WorkflowRun{s.runOrg2Public},
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			got, _, err := s.WorkflowRun.List(context.Background(), s.org2.ID, tc.filters, &pagination.CursorOptions{Limit: 10})
			if tc.wantErr {
				s.Error(err)
				return
			}

			s.NoError(err)
			s.Len(got, len(tc.want))
			gotIDs := make([]uuid.UUID, len(got))
			for _, g := range got {
				gotIDs = append(gotIDs, g.ID)
			}

			wantIDs := make([]uuid.UUID, len(tc.want))
			for _, w := range tc.want {
				wantIDs = append(wantIDs, w.ID)
			}

			s.ElementsMatch(wantIDs, gotIDs)
		})
	}
}

func (s *workflowRunIntegrationTestSuite) TestSaveAttestation() {
	assert := assert.New(s.T())
	ctx := context.Background()

	validEnvelope, envelopeBytes := testEnvelope(s.T(), "testdata/attestations/full.json")
	h, _, err := v2.SHA256(bytes.NewReader(envelopeBytes))
	require.NoError(s.T(), err)

	s.T().Run("non existing workflowRun", func(t *testing.T) {
		_, err := s.WorkflowRun.SaveAttestation(ctx, uuid.NewString(), envelopeBytes, nil)
		assert.Error(err)
		assert.True(biz.IsNotFound(err))
	})

	s.T().Run("valid workflowRun", func(_ *testing.T) {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
		})
		assert.NoError(err)

		d, err := s.WorkflowRun.SaveAttestation(ctx, run.ID.String(), envelopeBytes, nil)
		assert.NoError(err)
		wantDigest := h.String()
		assert.Equal(wantDigest, d)

		// Retrieve attestation ref from storage and compare
		r, err := s.WorkflowRun.GetByIDInOrgOrPublic(ctx, s.org.ID, run.ID.String())
		assert.NoError(err)
		assert.Equal(wantDigest, r.Attestation.Digest)
		assert.Equal(&biz.Attestation{Envelope: validEnvelope, Digest: wantDigest}, r.Attestation)
	})

	_, bundleBytes := testBundle(s.T(), "testdata/attestations/bundle.json")
	bundleHash, _, err := v2.SHA256(bytes.NewReader(bundleBytes))
	require.NoError(s.T(), err)

	s.T().Run("saves the bundle", func(_ *testing.T) {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
		})
		assert.NoError(err)

		d, err := s.WorkflowRun.SaveAttestation(ctx, run.ID.String(), envelopeBytes, bundleBytes)
		assert.NoError(err)
		wantDigest := bundleHash.String()
		assert.Equal(wantDigest, d)
		exists, err := s.Data.DB.Attestation.Query().Where(attestation2.WorkflowrunID(run.ID)).Exist(ctx)
		assert.NoError(err)
		assert.True(exists)
	})
}

func (s *workflowRunIntegrationTestSuite) TestGetByIDInOrgOrPublic() {
	assert := assert.New(s.T())
	ctx := context.Background()
	testCases := []struct {
		name    string
		orgID   string
		runID   string
		wantErr bool
	}{
		{
			name:    "non existing workflowRun",
			orgID:   s.org.ID,
			runID:   uuid.NewString(),
			wantErr: true,
		},
		{
			name:  "existing workflowRun in org1",
			orgID: s.org.ID,
			runID: s.runOrg1.ID.String(),
		},
		{
			name:    "can't access workflowRun from other org",
			orgID:   s.org.ID,
			runID:   s.runOrg2.ID.String(),
			wantErr: true,
		},
		{
			name:  "can access workflowRun from other org if public",
			orgID: s.org.ID,
			runID: s.runOrg2Public.ID.String(),
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			run, err := s.WorkflowRun.GetByIDInOrgOrPublic(ctx, tc.orgID, tc.runID)
			if tc.wantErr {
				assert.Error(err)
				assert.True(biz.IsNotFound(err))
			} else {
				assert.NoError(err)
				assert.Equal(tc.runID, run.ID.String())
			}
		})
	}
}

func (s *workflowRunIntegrationTestSuite) TestGetByDigestInOrgOrPublic() {
	assert := assert.New(s.T())
	ctx := context.Background()
	testCases := []struct {
		name           string
		orgID          string
		digest         string
		errTypeChecker func(err error) bool
	}{
		{
			name:           "non existing workflowRun",
			orgID:          s.org.ID,
			digest:         "sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
			errTypeChecker: biz.IsNotFound,
		},
		{
			name:           "invalid digest",
			orgID:          s.org.ID,
			digest:         "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c",
			errTypeChecker: biz.IsErrValidation,
		},
		{
			name:   "existing workflowRun in org1",
			orgID:  s.org.ID,
			digest: s.digestAtt1,
		},
		{
			name:           "can't access workflowRun from other org",
			orgID:          s.org.ID,
			digest:         s.digestAttOrg2,
			errTypeChecker: biz.IsNotFound,
		},
		{
			name:   "can access workflowRun from other org if public",
			orgID:  s.org.ID,
			digest: s.digestAttPublic,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			run, err := s.WorkflowRun.GetByDigestInOrgOrPublic(ctx, tc.orgID, tc.digest)
			if tc.errTypeChecker != nil {
				assert.Error(err)
				assert.True(tc.errTypeChecker(err))
			} else {
				assert.NoError(err)
				assert.Equal(tc.digest, run.Attestation.Digest)
			}
		})
	}
}

func (s *workflowRunIntegrationTestSuite) TestCreate() {
	ctx := context.Background()

	s.T().Run("valid workflowRun", func(t *testing.T) {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
			RunnerType: "runnerType", RunnerRunURL: "runURL",
		})
		s.Require().NoError(err)
		// Load project version
		pv, err := s.ProjectVersion.FindByProjectAndVersion(ctx, s.workflowOrg1.ProjectID.String(), "")
		s.Require().NoError(err)
		s.Equal("runnerType", run.RunnerType)
		s.Equal("runURL", run.RunURL)
		s.Equal(string(biz.WorkflowRunInitialized), run.State)
		s.Equal(pv, run.ProjectVersion)
	})

	s.T().Run("find or create version", func(_ *testing.T) {
		testCases := []struct {
			version string
		}{
			{version: ""},
			{version: "custom"},
		}

		for _, tc := range testCases {
			run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
				WorkflowID: s.workflowOrg1.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
				RunnerType: "runnerType", RunnerRunURL: "runURL", ProjectVersion: tc.version,
			})
			s.Require().NoError(err)
			// Load project version
			s.Equal(tc.version, run.ProjectVersion.Version)
			pv, err := s.ProjectVersion.FindByProjectAndVersion(ctx, s.workflowOrg1.ProjectID.String(), tc.version)
			s.Require().NoError(err)
			s.Equal(pv.ID, run.ProjectVersion.ID)
		}
	})
}

func (s *workflowRunIntegrationTestSuite) TestContractInformation() {
	ctx := context.Background()
	s.Run("if it's the first revision of the contract it matches", func() {
		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
			RunnerType: "runnerType", RunnerRunURL: "runURL",
		})
		s.NoError(err)
		s.Equal(1, run.ContractRevisionUsed)
		s.Equal(1, run.ContractRevisionLatest)
	})

	s.Run("if the contract gets a new revision but it's not used, it shows spread", func() {
		c := &schemav1.CraftingSchema{
			SchemaVersion: "v1",
			Runner:        &schemav1.CraftingSchema_Runner{Type: schemav1.CraftingSchema_Runner_CIRCLECI_BUILD},
		}

		rawContract, err := biz.SchemaToRawContract(c)
		require.NoError(s.T(), err)

		updatedContractRevision, err := s.WorkflowContract.Update(ctx, s.org.ID, s.contractVersion.Contract.Name,
			&biz.WorkflowContractUpdateOpts{RawSchema: rawContract.Raw})
		s.NoError(err)
		// load the previous version of the contract
		updatedContractRevision.Version = s.contractVersion.Version

		run, err := s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), ContractRevision: updatedContractRevision, CASBackendID: s.casBackend.ID,
			RunnerType: "runnerType", RunnerRunURL: "runURL",
		})
		s.NoError(err)
		// Shows that the latest available revision is 2, but the used one is 1
		s.Equal(1, run.ContractRevisionUsed)
		s.Equal(2, run.ContractRevisionLatest)
	})
}

// Run the tests
func TestWorkflowRunUseCase(t *testing.T) {
	suite.Run(t, new(workflowRunIntegrationTestSuite))
}

// Utility struct to hold the test suite
type workflowRunIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	*workflowRunTestData
}

type workflowRunTestData struct {
	org, org2                                      *biz.Organization
	casBackend                                     *biz.CASBackend
	workflowOrg1, workflowOrg2, workflowPublicOrg2 *biz.Workflow
	runOrg1, runOrg2, runOrg2Public                *biz.WorkflowRun
	contractVersion                                *biz.WorkflowContractWithVersion
	digestAtt1, digestAttOrg2, digestAttPublic     string
	version1, version2                             *biz.ProjectVersion
}

func testEnvelope(t *testing.T, path string) (*dsse.Envelope, []byte) {
	attJSON, err := os.ReadFile(path)
	require.NoError(t, err)
	var envelope *dsse.Envelope
	require.NoError(t, json.Unmarshal(attJSON, &envelope))
	return envelope, attJSON
}

func testBundle(t *testing.T, path string) (*v1.Bundle, []byte) {
	bundleJSON, err := os.ReadFile(path)
	require.NoError(t, err)
	var bundle v1.Bundle
	require.NoError(t, protojson.Unmarshal(bundleJSON, &bundle))
	return &bundle, bundleJSON
}

const (
	version1 = "v1"
	version2 = "v2"
)

// extract this setup to a helper function so it can be used from other test suites
func setupWorkflowRunTestData(t *testing.T, suite *testhelpers.TestingUseCases, s *workflowRunTestData) {
	var err error
	assert := assert.New(t)
	ctx := context.Background()

	s.org, err = suite.Organization.Create(ctx, "testing-org")
	assert.NoError(err)
	s.org2, err = suite.Organization.Create(ctx, "second-org")
	assert.NoError(err)

	// Workflow
	s.workflowOrg1, err = suite.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "test-workflow", OrgID: s.org.ID, Project: "test-project"})
	assert.NoError(err)
	s.workflowOrg2, err = suite.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "test-workflow", OrgID: s.org2.ID, Project: "test-project"})
	assert.NoError(err)
	// Public workflow
	s.workflowPublicOrg2, err = suite.Workflow.Create(ctx, &biz.WorkflowCreateOpts{Name: "test-public-workflow", OrgID: s.org2.ID, Public: true, Project: "test-project"})
	assert.NoError(err)

	// Find contract revision
	s.contractVersion, err = suite.WorkflowContract.Describe(ctx, s.org.ID, s.workflowOrg1.ContractID.String(), 0)
	assert.NoError(err)

	s.casBackend, err = suite.CASBackend.CreateOrUpdate(ctx, s.org.ID, "repo", "username", "pass", backendType, true)
	assert.NoError(err)

	// Let's create 3 runs, one in org1 and 2 in org2 (one public)
	s.runOrg1, err = suite.WorkflowRun.Create(ctx,
		&biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg1.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
			ProjectVersion: version1,
		})
	assert.NoError(err)
	_, envBytes := testEnvelope(t, "testdata/attestations/full.json")
	d, err := suite.WorkflowRun.SaveAttestation(ctx, s.runOrg1.ID.String(), envBytes, nil)
	assert.NoError(err)
	s.digestAtt1 = d.String()

	s.runOrg2, err = suite.WorkflowRun.Create(ctx,
		&biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowOrg2.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
			ProjectVersion: version1,
		})
	assert.NoError(err)
	_, envBytes = testEnvelope(t, "testdata/attestations/empty.json")
	d, err = suite.WorkflowRun.SaveAttestation(ctx, s.runOrg2.ID.String(), envBytes, nil)
	assert.NoError(err)
	s.digestAttOrg2 = d.String()

	s.runOrg2Public, err = suite.WorkflowRun.Create(ctx,
		&biz.WorkflowRunCreateOpts{
			WorkflowID: s.workflowPublicOrg2.ID.String(), ContractRevision: s.contractVersion, CASBackendID: s.casBackend.ID,
			ProjectVersion: version2,
		})
	assert.NoError(err)
	_, envBytes = testEnvelope(t, "testdata/attestations/with-string.json")
	d, err = suite.WorkflowRun.SaveAttestation(ctx, s.runOrg2Public.ID.String(), envBytes, nil)
	assert.NoError(err)
	s.digestAttPublic = d.String()

	s.version1, err = suite.ProjectVersion.FindByProjectAndVersion(ctx, s.workflowOrg2.ProjectID.String(), version1)
	require.NoError(t, err)
	s.version2, err = suite.ProjectVersion.FindByProjectAndVersion(ctx, s.workflowPublicOrg2.ProjectID.String(), version2)
	require.NoError(t, err)
}

func (s *workflowRunIntegrationTestSuite) SetupTest() {
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On("SaveCredentials", context.Background(), mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	s.workflowRunTestData = &workflowRunTestData{}
	setupWorkflowRunTestData(s.T(), s.TestingUseCases, s.workflowRunTestData)
}
