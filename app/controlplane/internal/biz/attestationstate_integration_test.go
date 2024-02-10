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
	"github.com/chainloop-dev/chainloop/internal/credentials"
	creds "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/google/uuid"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *attestationStateTestSuite) TestInitialized() {
	ctx := context.Background()
	s.T().Run("run in different workflow causes error", func(t *testing.T) {
		_, err := s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg2.ID.String())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("the run doesn't exist", func(t *testing.T) {
		_, err := s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), uuid.NewString())
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("the run id is invalid", func(t *testing.T) {
		_, err := s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), "deadbeef")
		s.Error(err)
		s.True(biz.IsErrInvalidUUID(err))
	})

	s.T().Run("the run isn't initialized", func(t *testing.T) {
		ok, err := s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.False(ok)
	})

	s.T().Run("the run is initialized", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState)
		s.NoError(err)
		ok, err := s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.True(ok)
	})
}

func (s *attestationStateTestSuite) TestSave() {
	ctx := context.Background()
	s.T().Run("run in different workflow causes error", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg2.ID.String(), s.testState)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("the run doesn't exist", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), uuid.NewString(), s.testState)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("the run exists", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState)
		s.NoError(err)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.Equal(s.testState, got.EncryptedState)
	})

	s.T().Run("it can be overridden", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState)
		s.NoError(err)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.Equal(s.testState, got.EncryptedState)

		err = s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), []byte("new state"))
		s.NoError(err)

		got, err = s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.Equal([]byte("new state"), got.EncryptedState)
	})
}

func (s *attestationStateTestSuite) TestReset() {
	ctx := context.Background()

	s.T().Run("the run is initialized", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState)
		s.NoError(err)

		ok, err := s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.True(ok)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.Equal(s.testState, got.EncryptedState)

		err = s.AttestationState.Reset(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)

		ok, err = s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.False(ok)

		got, err = s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.Equal([]byte(nil), got.EncryptedState)
	})

	s.T().Run("if the run is not initialized the state doesn't change", func(t *testing.T) {
		err := s.AttestationState.Reset(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)

		ok, err := s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.False(ok)
	})
}

// Run the tests
func TestAttestationStateUseCase(t *testing.T) {
	suite.Run(t, new(attestationStateTestSuite))
}

// Utility struct to hold the test suite
type attestationStateTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	*workflowRunTestData
	testState []byte
}

func (s *attestationStateTestSuite) SetupTest() {
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On("SaveCredentials", context.Background(), mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	s.workflowRunTestData = &workflowRunTestData{}
	setupWorkflowRunTestData(s.T(), s.TestingUseCases, s.workflowRunTestData)
	s.testState = []byte("test state")
}
