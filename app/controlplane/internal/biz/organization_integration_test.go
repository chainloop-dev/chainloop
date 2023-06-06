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

	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	integrationMocks "github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/chainloop-dev/chainloop/internal/credentials"
	creds "github.com/chainloop-dev/chainloop/internal/credentials/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// We are doing an integration test here because there are some database constraints
// and delete cascades that we want to validate that they work too
func (s *OrgIntegrationTestSuite) TestDeleteOrg() {
	assert := assert.New(s.T())
	ctx := context.Background()

	s.T().Run("invalid org ID", func(t *testing.T) {
		// Invalid org ID
		err := s.Organization.Delete(ctx, "invalid")
		assert.Error(err)
		assert.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("org non existent", func(t *testing.T) {
		// org not found
		err := s.Organization.Delete(ctx, uuid.NewString())
		assert.Error(err)
		assert.True(biz.IsNotFound(err))
	})

	s.T().Run("org, integrations and repositories deletion", func(t *testing.T) {
		// Mock calls to credentials deletion for both the integration and the OCI repository
		s.mockedCredsReaderWriter.On("DeleteCredentials", ctx, "stored-OCI-secret").Return(nil)

		err := s.Organization.Delete(ctx, s.org.ID)
		assert.NoError(err)

		// Integrations and repo deleted as well
		integrations, err := s.Integration.List(ctx, s.org.ID)
		assert.NoError(err)
		assert.Empty(integrations)

		ociRepo, err := s.OCIRepo.FindMainRepo(ctx, s.org.ID)
		assert.NoError(err)
		assert.Nil(ociRepo)

		workflows, err := s.Workflow.List(ctx, s.org.ID)
		assert.NoError(err)
		assert.Empty(workflows)

		contracts, err := s.WorkflowContract.List(ctx, s.org.ID)
		assert.NoError(err)
		assert.Empty(contracts)
	})
}

// Run the tests
func TestOrgUseCase(t *testing.T) {
	suite.Run(t, new(OrgIntegrationTestSuite))
}

// Utility struct to hold the test suite
type OrgIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	org                     *biz.Organization
	mockedCredsReaderWriter *creds.ReaderWriter
}

func (s *OrgIntegrationTestSuite) SetupTest() {
	t := s.T()
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()

	// Override credentials writer to set expectations
	s.mockedCredsReaderWriter = creds.NewReaderWriter(t)
	// Mock API call to store credentials

	// OCI repository credentials
	s.mockedCredsReaderWriter.On(
		"SaveCredentials", ctx, mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"},
	).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(t, testhelpers.WithCredsReaderWriter(s.mockedCredsReaderWriter))

	// Create org, integration and oci repository
	s.org, err = s.Organization.Create(ctx, "testing org")
	assert.NoError(err)

	// Integration
	// Mocked integration that will return both generic configuration and credentials
	integration := integrationMocks.NewFanOut(s.T())
	integration.On("Describe").Return(&sdk.IntegrationInfo{})
	integration.On("PreRegister", ctx, mock.Anything).Return(&sdk.PreRegistration{
		Configuration: &anypb.Any{}}, nil)
	_, err = s.Integration.RegisterAndSave(ctx, s.org.ID, integration, nil)
	assert.NoError(err)

	// OCI repository
	_, err = s.OCIRepo.CreateOrUpdate(ctx, s.org.ID, "repo", "username", "pass")
	assert.NoError(err)

	// Workflow + contract
	_, err = s.Workflow.Create(ctx, &biz.CreateOpts{Name: "test workflow", OrgID: s.org.ID})
	assert.NoError(err)

	// check integration, OCI repository and workflow and contracts are present in the db
	integrations, err := s.Integration.List(ctx, s.org.ID)
	assert.NoError(err)
	assert.Len(integrations, 1)

	ociRepo, err := s.OCIRepo.FindMainRepo(ctx, s.org.ID)
	assert.NoError(err)
	assert.NotNil(ociRepo)

	workflows, err := s.Workflow.List(ctx, s.org.ID)
	assert.NoError(err)
	assert.Len(workflows, 1)

	contracts, err := s.WorkflowContract.List(ctx, s.org.ID)
	assert.NoError(err)
	assert.Len(contracts, 1)
}
