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

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/biz/testhelpers"
	v1 "github.com/chainloop-dev/chainloop/internal/attestation/crafter/api/attestation/v1"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	creds "github.com/chainloop-dev/chainloop/pkg/credentials/mocks"
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
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState, s.passphrase)
		s.NoError(err)
		ok, err := s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.True(ok)
	})
}

func (s *attestationStateTestSuite) TestSave() {
	ctx := context.Background()
	s.T().Run("run in different workflow causes error", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg2.ID.String(), s.testState, s.passphrase)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("the run doesn't exist", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), uuid.NewString(), s.testState, s.passphrase)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("the run exists", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState, s.passphrase)
		s.NoError(err)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.passphrase)
		s.NoError(err)
		if ok := proto.Equal(s.testState, got.State); !ok {
			s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", s.testState, got.State))
		}
	})

	s.T().Run("it can be overridden", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState, s.passphrase)
		s.NoError(err)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.passphrase)
		s.NoError(err)
		if ok := proto.Equal(s.testState, got.State); !ok {
			s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", s.testState, got.State))
		}

		newState := &v1.CraftingState{}
		err = s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), newState, s.passphrase)
		s.NoError(err)

		got, err = s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.passphrase)
		s.NoError(err)
		if ok := proto.Equal(newState, got.State); !ok {
			s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", newState, got.State))
		}
	})
}

func (s *attestationStateTestSuite) TestRead() {
	ctx := context.Background()
	s.T().Run("can be retrieved with same passphrase", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState, s.passphrase)
		s.NoError(err)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.passphrase)
		s.NoError(err)
		if ok := proto.Equal(s.testState, got.State); !ok {
			s.Fail(fmt.Sprintf("These two protobuf messages are not equal:\nexpected: %v\nactual:  %v", s.testState, got.State))
		}
	})

	s.T().Run("it fails if they are different passphrases", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState, s.passphrase)
		s.NoError(err)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), "wrong-passphrase")
		s.ErrorContains(err, "incorrect passphrase")
		s.Nil(got)
	})

	s.T().Run("it fails if the content has been tampered with", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState, s.passphrase)
		s.NoError(err)

		// tamper directly with the database
		err = s.Repos.AttestationState.Save(ctx, s.runOrg1.ID, []byte("tampered data modified directly in the DB"))
		s.NoError(err)

		got, err := s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.passphrase)
		s.ErrorContains(err, "incorrect passphrase")
		s.Nil(got)
	})
}

func (s *attestationStateTestSuite) TestWorkflowRunLifecycle() {
	ctx := context.Background()
	s.T().Run("the state gets cleared when workflow run is set as finished", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState, s.passphrase)
		s.NoError(err)

		err = s.WorkflowRun.MarkAsFinished(ctx, s.runOrg1.ID.String(), biz.WorkflowRunSuccess, "finished")
		s.NoError(err)

		_, err = s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.passphrase)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})

	s.T().Run("or it expires", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState, s.passphrase)
		s.NoError(err)

		err = s.Repos.WorkflowRunRepo.Expire(ctx, s.runOrg1.ID)
		s.NoError(err)

		_, err = s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.passphrase)
		s.Error(err)
		s.True(biz.IsNotFound(err))
	})
}

func (s *attestationStateTestSuite) TestReset() {
	ctx := context.Background()

	s.T().Run("the run is initialized", func(t *testing.T) {
		err := s.AttestationState.Save(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.testState, s.passphrase)
		s.NoError(err)

		ok, err := s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.True(ok)

		_, err = s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.passphrase)
		s.NoError(err)

		err = s.AttestationState.Reset(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)

		ok, err = s.AttestationState.Initialized(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String())
		s.NoError(err)
		s.False(ok)

		_, err = s.AttestationState.Read(ctx, s.workflowOrg1.ID.String(), s.runOrg1.ID.String(), s.passphrase)
		s.Error(err)
		s.True(biz.IsNotFound(err))
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
	testState  *v1.CraftingState
	passphrase string
}

func (s *attestationStateTestSuite) SetupTest() {
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On("SaveCredentials", context.Background(), mock.Anything, &credentials.OCIKeypair{Repo: "repo", Username: "username", Password: "pass"}).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	s.workflowRunTestData = &workflowRunTestData{}
	setupWorkflowRunTestData(s.T(), s.TestingUseCases, s.workflowRunTestData)
	s.testState = &v1.CraftingState{Attestation: &v1.Attestation{Annotations: map[string]string{"key": "value"}}}
	s.passphrase = "development passphrase super secret"
}
