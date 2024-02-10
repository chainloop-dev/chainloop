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
	"fmt"
	"testing"

	schemav1 "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	creds "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

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
		if ok := proto.Equal(s.testState, got.State); !ok {
			s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", s.testState, got.State))
		}
	})

	s.T().Run("it can be overridden", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState)
		s.NoError(err)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		if ok := proto.Equal(s.testState, got.State); !ok {
			s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", s.testState, got.State))
		}

		newState := &schemav1.CraftingSchema{SchemaVersion: "v2"}
		err = s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), newState)
		s.NoError(err)

		got, err = s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		if ok := proto.Equal(newState, got.State); !ok {
			s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", newState, got.State))
		}
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

		_, err = s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)

		err = s.AttestationState.Reset(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)

		ok, err = s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.False(ok)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		if ok := proto.Equal(&schemav1.CraftingSchema{}, got.State); !ok {
			s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", nil, got.State))
		}
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
	testState *schemav1.CraftingSchema
}

func (s *attestationStateTestSuite) SetupTest() {
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On("SaveCredentials", context.Background(), mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	s.workflowRunTestData = &workflowRunTestData{}
	setupWorkflowRunTestData(s.T(), s.TestingUseCases, s.workflowRunTestData)
	s.testState = &schemav1.CraftingSchema{SchemaVersion: "v1"}
}
