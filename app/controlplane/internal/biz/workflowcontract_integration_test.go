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
	"context"
	"testing"

	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *workflowContractIntegrationTestSuite) TestUpdate() {
	ctx := context.Background()

	testCases := []struct {
		name         string
		contractName string
		OrgID, ID    string
		inputSchema  *schemav1.CraftingSchema
		wantErrMsg   string
		wantRevision int
	}{
		{
			name:         "non-existing contract",
			contractName: "non-existing",
			OrgID:        s.org.ID,
			ID:           uuid.NewString(),
			wantErrMsg:   "not found",
		},
		{
			name:         "existing contract invalid name",
			contractName: "invalid name",
			OrgID:        s.org.ID,
			ID:           s.contractOrg1.ID.String(),
			wantErrMsg:   "RFC 1123",
		},
		{
			name:         "existing contract valid name, does not bump revision",
			contractName: "valid-name",
			OrgID:        s.org.ID,
			ID:           s.contractOrg1.ID.String(),
			wantRevision: 1,
		},
		{
			name:         "updating schema bumps revision",
			OrgID:        s.org.ID,
			ID:           s.contractOrg1.ID.String(),
			contractName: "valid-name",
			inputSchema:  &schemav1.CraftingSchema{SchemaVersion: "v123"},
			wantRevision: 2,
		},
		{
			name:         "updating with same schema DOES NOT bump revision",
			OrgID:        s.org.ID,
			ID:           s.contractOrg1.ID.String(),
			contractName: "valid-name",
			inputSchema:  &schemav1.CraftingSchema{SchemaVersion: "v123"},
			wantRevision: 2,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			contract, err := s.WorkflowContract.Update(ctx, tc.OrgID, tc.ID,
				&biz.WorkflowContractUpdateOpts{Name: tc.contractName, Schema: tc.inputSchema},
			)
			if tc.wantErrMsg != "" {
				s.ErrorContains(err, tc.wantErrMsg)
				return
			}

			require.NoError(s.T(), err)
			s.Equal(tc.contractName, contract.Contract.Name)
			s.Equal(tc.wantRevision, contract.Version.Revision)
		})
	}
}

func (s *workflowContractIntegrationTestSuite) TestCreate() {
	ctx := context.Background()

	testCases := []struct {
		name       string
		opts       *biz.WorkflowContractCreateOpts
		wantErrMsg string
	}{
		{
			name:       "org missing",
			opts:       &biz.WorkflowContractCreateOpts{Name: "name"},
			wantErrMsg: "required",
		},
		{
			name:       "name missing",
			opts:       &biz.WorkflowContractCreateOpts{OrgID: s.org.ID},
			wantErrMsg: "required",
		},
		{
			name:       "invalid name",
			opts:       &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "this/not/valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name:       "another invalid name",
			opts:       &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "this-not Valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name: "non-existing contract name",
			opts: &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "name"},
		},
		{
			name:       "existing contract name",
			opts:       &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "name"},
			wantErrMsg: "taken",
		},
		{
			name: "can create same name in different org",
			opts: &biz.WorkflowContractCreateOpts{OrgID: s.org2.ID, Name: "name"},
		},
		{
			name: "or ask to generate a random name",
			opts: &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "name", AddUniquePrefix: true},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			contract, err := s.WorkflowContract.Create(ctx, tc.opts)
			if tc.wantErrMsg != "" {
				s.ErrorContains(err, tc.wantErrMsg)
				return
			}

			require.NoError(s.T(), err)
			s.NotEmpty(contract.ID)
			s.NotEmpty(contract.CreatedAt)
		})
	}
}

// Run the tests
func TestWorkflowContractUseCase(t *testing.T) {
	suite.Run(t, new(workflowContractIntegrationTestSuite))
}

// Utility struct to hold the test suite
type workflowContractIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org, org2 *biz.Organization

	contractOrg1 *biz.WorkflowContract
}

func (s *workflowContractIntegrationTestSuite) SetupTest() {
	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T())

	var err error
	ctx := context.Background()
	s.org, err = s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)
	s.org2, err = s.Organization.CreateWithRandomName(ctx)
	s.NoError(err)

	s.contractOrg1, err = s.WorkflowContract.Create(ctx, &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "a-valid-contract"})
	s.NoError(err)
}
