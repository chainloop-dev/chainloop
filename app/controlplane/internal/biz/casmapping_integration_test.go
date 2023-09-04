//
// Copyright 2023 The Chainloop Authors.
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
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	creds "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *casMappingIntegrationSuite) TestCreate() {
	validDigest := "sha256:3b0f04c276be095e62f3ac03b9991913c37df1fcd44548e75069adce313aba4d"
	invalidDigest := "sha256:deadbeef"

	testCases := []struct {
		name          string
		digest        string
		casBackendID  uuid.UUID
		workflowRunID uuid.UUID
		wantErr       bool
	}{
		{
			name:          "valid",
			digest:        validDigest,
			casBackendID:  s.casBackend.ID,
			workflowRunID: s.workflowRun.ID,
		},
		{
			name:          "created again with same digest",
			digest:        validDigest,
			casBackendID:  s.casBackend.ID,
			workflowRunID: s.workflowRun.ID,
		},
		{
			name:          "invalid digest format",
			digest:        invalidDigest,
			casBackendID:  s.casBackend.ID,
			workflowRunID: s.workflowRun.ID,
			wantErr:       true,
		},
		{
			name:          "invalid digest missing prefix",
			digest:        "3b0f04c276be095e62f3ac03b9991913c37df1fcd44548e75069adce313aba4d",
			casBackendID:  s.casBackend.ID,
			workflowRunID: s.workflowRun.ID,
			wantErr:       true,
		},
		{
			name:          "non-existing CASBackend",
			digest:        validDigest,
			casBackendID:  uuid.New(),
			workflowRunID: s.workflowRun.ID,
			wantErr:       true,
		},
		{
			name:          "non-existing WorkflowRunID",
			digest:        validDigest,
			casBackendID:  s.casBackend.ID,
			workflowRunID: uuid.New(),
			wantErr:       true,
		},
	}

	want := &biz.CASMapping{
		Digest:        validDigest,
		CASBackendID:  s.casBackend.ID,
		WorkflowRunID: s.workflowRun.ID,
		OrgID:         s.casBackend.OrganizationID,
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			got, err := s.CASMapping.Create(context.TODO(), tc.digest, tc.casBackendID.String(), tc.workflowRunID.String())
			if tc.wantErr {
				s.Error(err)
			} else {
				s.NoError(err)
				if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(biz.CASMapping{}, "CreatedAt", "ID")); diff != "" {
					assert.Failf(s.T(), "mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}

type casMappingIntegrationSuite struct {
	testhelpers.UseCasesEachTestSuite
	casBackend  *biz.CASBackend
	workflowRun *biz.WorkflowRun
}

func (s *casMappingIntegrationSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()

	// RunDB
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On(
		"SaveCredentials", ctx, mock.Anything, mock.Anything,
	).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	// Create casBackend in the database
	org, err := s.Organization.Create(ctx, "testing org 1 with one backend")
	assert.NoError(err)
	s.casBackend, err = s.CASBackend.Create(ctx, org.ID, "my-location", "backend 1 description", biz.CASBackendOCI, nil, true)
	assert.NoError(err)

	// Create workflowRun in the database
	// Workflow
	workflow, err := s.Workflow.Create(ctx, &biz.CreateOpts{Name: "test workflow", OrgID: org.ID})
	assert.NoError(err)

	// Robot account
	robotAccount, err := s.RobotAccount.Create(ctx, "name", org.ID, workflow.ID.String())
	assert.NoError(err)

	// Find contract revision
	contractVersion, err := s.WorkflowContract.Describe(ctx, org.ID, workflow.ContractID.String(), 0)
	assert.NoError(err)

	s.workflowRun, err = s.WorkflowRun.Create(ctx, &biz.WorkflowRunCreateOpts{
		WorkflowID: workflow.ID.String(), RobotaccountID: robotAccount.ID.String(), ContractRevisionUUID: contractVersion.Version.ID, CASBackendID: s.casBackend.ID,
		RunnerType: "runnerType", RunnerRunURL: "runURL",
	})
	assert.NoError(err)
}

func TestCASMappingIntegration(t *testing.T) {
	suite.Run(t, new(casMappingIntegrationSuite))
}
