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
	"errors"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	bizMocks "github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/mocks"
	backends "github.com/chainloop-dev/chainloop/internal/blobmanager"
	blobM "github.com/chainloop-dev/chainloop/internal/blobmanager/mocks"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	credentialsM "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type casBackendTestSuite struct {
	suite.Suite
	validUUID       uuid.UUID
	invalidUUID     string
	useCase         *biz.CASBackendUseCase
	repo            *bizMocks.CASBackendRepo
	credsRW         *credentialsM.ReaderWriter
	backendProvider *blobM.Provider
}

func (s *casBackendTestSuite) TestFindDefaultBackendErr() {
	repo, err := s.useCase.FindDefaultBackend(context.Background(), s.invalidUUID)
	assert.True(s.T(), biz.IsErrInvalidUUID(err))
	assert.Nil(s.T(), repo)
}

func (s *casBackendTestSuite) TestFindDefaultBackendNotFound() {
	assert := assert.New(s.T())

	// Not found
	ctx := context.Background()
	s.repo.On("FindDefaultBackend", ctx, s.validUUID).Return(nil, nil)

	repo, err := s.useCase.FindDefaultBackend(ctx, s.validUUID.String())
	assert.ErrorAs(err, &biz.ErrNotFound{})
	assert.Nil(repo)
}

func (s *casBackendTestSuite) TestFindDefaultBackendFound() {
	assert := assert.New(s.T())

	ctx := context.Background()
	wantBackend := &biz.CASBackend{}
	s.repo.On("FindDefaultBackend", ctx, s.validUUID).Return(wantBackend, nil)

	backend, err := s.useCase.FindDefaultBackend(ctx, s.validUUID.String())
	assert.NoError(err)
	assert.Equal(backend, wantBackend)
}

func (s *casBackendTestSuite) TestSaveInvalidUUID() {
	repo, err := s.useCase.CreateOrUpdate(context.Background(), s.invalidUUID, "", "", "", biz.CASBackendOCI, true)
	assert.True(s.T(), biz.IsErrInvalidUUID(err))
	assert.Nil(s.T(), repo)
}

// If a repo exists it will get updated
func (s *casBackendTestSuite) TestSaveDefaultBackendAlreadyExist() {
	assert := assert.New(s.T())
	const repoName, username, password = "repo", "username", "pass"

	r := &biz.CASBackend{ID: s.validUUID}
	ctx := context.Background()
	s.repo.On("FindDefaultBackend", ctx, s.validUUID).Return(r, nil)
	s.credsRW.On("SaveCredentials", ctx, s.validUUID.String(), mock.Anything).Return("secret-key", nil)
	s.repo.On("Update", ctx, &biz.CASBackendUpdateOpts{
		ID: s.validUUID,
		CASBackendOpts: &biz.CASBackendOpts{
			Location: repoName, SecretName: "secret-key", Default: true, Provider: biz.CASBackendOCI,
		},
	}).Return(r, nil)

	gotRepo, err := s.useCase.CreateOrUpdate(ctx, s.validUUID.String(), repoName, username, password, biz.CASBackendOCI, true)
	assert.NoError(err)
	assert.Equal(gotRepo, r)
}

func (s *casBackendTestSuite) TestSaveDefaultBackendOk() {
	assert := assert.New(s.T())

	ctx := context.Background()
	const repo, username, password = "repo", "username", "pass"

	s.repo.On("FindDefaultBackend", ctx, s.validUUID).Return(nil, nil)
	s.credsRW.On("SaveCredentials", ctx, s.validUUID.String(), mock.Anything).Return("secret-key", nil)

	newRepo := &biz.CASBackend{}
	s.repo.On("Create", ctx, &biz.CASBackendCreateOpts{
		CASBackendOpts: &biz.CASBackendOpts{
			OrgID:    s.validUUID,
			Location: repo, SecretName: "secret-key", Default: true, Provider: biz.CASBackendOCI,
		},
	}).Return(newRepo, nil)

	gotRepo, err := s.useCase.CreateOrUpdate(ctx, s.validUUID.String(), repo, username, password, biz.CASBackendOCI, true)
	assert.NoError(err)
	assert.Equal(gotRepo, newRepo)
}

func (s *casBackendTestSuite) TestPerformValidation() {
	assert := assert.New(s.T())
	t := s.T()
	validRepo := &biz.CASBackend{ID: s.validUUID, ValidationStatus: biz.CASBackendValidationOK, Provider: biz.CASBackendOCI}

	t.Run("invalid uuid", func(t *testing.T) {
		err := s.useCase.PerformValidation(context.Background(), s.invalidUUID)
		assert.True(biz.IsErrInvalidUUID(err))
	})

	t.Run("not found", func(t *testing.T) {
		s.repo.On("FindByID", mock.Anything, s.validUUID).Return(nil, nil)
		err := s.useCase.PerformValidation(context.Background(), s.validUUID.String())
		assert.True(biz.IsNotFound(err))
		s.resetMock()
	})

	t.Run("proper provider credentials missing, set validation status => invalid", func(t *testing.T) {
		s.repo.On("FindByID", mock.Anything, s.validUUID).Return(validRepo, nil)
		s.repo.On("UpdateValidationStatus", mock.Anything, s.validUUID, biz.CASBackendValidationFailed).Return(nil)

		s.credsRW.On("ReadCredentials", mock.Anything, mock.Anything, mock.Anything).Return(credentials.ErrNotFound)
		err := s.useCase.PerformValidation(context.Background(), s.validUUID.String())
		assert.NoError(err)
		s.resetMock()
	})

	t.Run("invalid credentials, set validation status => invalid", func(t *testing.T) {
		s.repo.On("FindByID", mock.Anything, s.validUUID).Return(validRepo, nil)
		s.repo.On("UpdateValidationStatus", mock.Anything, s.validUUID, biz.CASBackendValidationFailed).Return(nil)
		s.credsRW.On("ReadCredentials", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		s.backendProvider.On("ValidateAndExtractCredentials", validRepo.Location, mock.Anything).Return(nil, errors.New("invalid credentials"))

		err := s.useCase.PerformValidation(context.Background(), s.validUUID.String())
		assert.NoError(err)
		s.resetMock()
	})

	t.Run("valid credentials, set validation status => ok", func(t *testing.T) {
		s.repo.On("FindByID", mock.Anything, s.validUUID).Return(validRepo, nil)
		s.repo.On("UpdateValidationStatus", mock.Anything, s.validUUID, biz.CASBackendValidationOK).Return(nil)
		s.credsRW.On("ReadCredentials", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		s.backendProvider.On("ValidateAndExtractCredentials", validRepo.Location, mock.Anything).Return(nil, nil)

		err := s.useCase.PerformValidation(context.Background(), s.validUUID.String())
		assert.NoError(err)
		s.resetMock()
	})
}

// Run all the tests
func TestCASBackend(t *testing.T) {
	suite.Run(t, new(casBackendTestSuite))
}

func (s *casBackendTestSuite) resetMock() {
	s.repo.Mock = mock.Mock{}
	s.credsRW.Mock = mock.Mock{}
	s.backendProvider.Mock = mock.Mock{}
}

func (s *casBackendTestSuite) SetupTest() {
	s.validUUID = uuid.New()
	s.invalidUUID = "deadbeef"
	s.repo = bizMocks.NewCASBackendRepo(s.T())
	s.credsRW = credentialsM.NewReaderWriter(s.T())
	s.backendProvider = blobM.NewProvider(s.T())
	s.useCase = biz.NewCASBackendUseCase(s.repo, s.credsRW,
		backends.Providers{
			"OCI": s.backendProvider,
		}, nil,
	)
}
