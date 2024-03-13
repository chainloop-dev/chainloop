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
		name            string
		OrgID, ID       string
		input           *biz.WorkflowContractUpdateOpts
		inputSchema     *schemav1.CraftingSchema
		wantErrMsg      string
		wantRevision    int
		wantName        string
		wantDescription string
	}{
		{
			name:       "non-updates",
			wantErrMsg: "no updates",
		},
		{
			name:       "non-existing contract",
			wantName:   "non-existing",
			OrgID:      s.org.ID,
			input:      &biz.WorkflowContractUpdateOpts{},
			ID:         uuid.NewString(),
			wantErrMsg: "not found",
		},
		{
			name:       "existing contract invalid name",
			input:      &biz.WorkflowContractUpdateOpts{Name: "invalid name"},
			OrgID:      s.org.ID,
			ID:         s.contractOrg1.ID.String(),
			wantErrMsg: "RFC 1123",
		},
		{
			name:         "existing contract valid name, does not bump revision",
			input:        &biz.WorkflowContractUpdateOpts{Name: "valid-name"},
			wantName:     "valid-name",
			OrgID:        s.org.ID,
			ID:           s.contractOrg1.ID.String(),
			wantRevision: 1,
		},
		{
			name:         "updating schema bumps revision",
			OrgID:        s.org.ID,
			ID:           s.contractOrg1.ID.String(),
			wantName:     "valid-name",
			input:        &biz.WorkflowContractUpdateOpts{Schema: &schemav1.CraftingSchema{SchemaVersion: "v123"}},
			wantRevision: 2,
		},
		{
			name:         "updating with same schema DOES NOT bump revision",
			OrgID:        s.org.ID,
			ID:           s.contractOrg1.ID.String(),
			input:        &biz.WorkflowContractUpdateOpts{Schema: &schemav1.CraftingSchema{SchemaVersion: "v123"}},
			wantName:     "valid-name",
			wantRevision: 2,
		},
		{
			name:            "you can update the description",
			OrgID:           s.org.ID,
			ID:              s.contractOrg1.ID.String(),
			input:           &biz.WorkflowContractUpdateOpts{Description: toPtrS("new description")},
			wantName:        "valid-name",
			wantDescription: "new description",
			wantRevision:    2,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			contract, err := s.WorkflowContract.Update(ctx, tc.OrgID, tc.ID, tc.input)
			if tc.wantErrMsg != "" {
				s.ErrorContains(err, tc.wantErrMsg)
				return
			}
			require.NoError(s.T(), err)

			if tc.wantName != "" {
				s.Equal(tc.wantName, contract.Contract.Name)
			}

			if tc.wantDescription != "" {
				s.Equal(tc.wantDescription, contract.Contract.Description)
			}

			s.Equal(tc.wantRevision, contract.Version.Revision)
		})
	}
}

func (s *workflowContractIntegrationTestSuite) TestCreateDuplicatedName() {
	ctx := context.Background()

	const contractName = "name"
	contract, err := s.WorkflowContract.Create(ctx, &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: contractName})
	require.NoError(s.T(), err)

	s.Run("can't create a contract with the same name", func() {
		_, err := s.WorkflowContract.Create(ctx, &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: contractName})
		s.ErrorContains(err, "name already taken")
	})

	s.Run("but if we delete it we can", func() {
		err = s.WorkflowContract.Delete(ctx, s.org.ID, contract.ID.String())
		require.NoError(s.T(), err)

		_, err := s.WorkflowContract.Create(ctx, &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: contractName})
		require.NoError(s.T(), err)
	})
}

func (s *workflowContractIntegrationTestSuite) TestCreate() {
	ctx := context.Background()

	testCases := []struct {
		name            string
		input           *biz.WorkflowContractCreateOpts
		wantErrMsg      string
		wantName        string
		wantDescription string
	}{
		{
			name:       "org missing",
			input:      &biz.WorkflowContractCreateOpts{Name: "name"},
			wantErrMsg: "required",
		},
		{
			name:       "name missing",
			input:      &biz.WorkflowContractCreateOpts{OrgID: s.org.ID},
			wantErrMsg: "required",
		},
		{
			name:       "invalid name",
			input:      &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "this/not/valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name:       "another invalid name",
			input:      &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "this-not Valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name:  "non-existing contract name",
			input: &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "name"},
		},
		{
			name:       "existing contract name",
			input:      &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "name"},
			wantErrMsg: "taken",
		},
		{
			name:     "can create same name in different org",
			input:    &biz.WorkflowContractCreateOpts{OrgID: s.org2.ID, Name: "name"},
			wantName: "name",
		},
		{
			name:  "or ask to generate a random name",
			input: &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "name", AddUniquePrefix: true},
		},
		{
			name:            "you can include a description",
			input:           &biz.WorkflowContractCreateOpts{OrgID: s.org.ID, Name: "name-2", Description: toPtrS("description")},
			wantName:        "name-2",
			wantDescription: "description",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			contract, err := s.WorkflowContract.Create(ctx, tc.input)
			if tc.wantErrMsg != "" {
				s.ErrorContains(err, tc.wantErrMsg)
				return
			}

			require.NoError(s.T(), err)
			s.NotEmpty(contract.ID)
			s.NotEmpty(contract.CreatedAt)

			if tc.wantDescription != "" {
				s.Equal(tc.wantDescription, contract.Description)
			}

			if tc.wantName != "" {
				s.Equal(tc.wantName, contract.Name)
			}
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
