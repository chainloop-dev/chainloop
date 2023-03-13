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
	blobM "github.com/chainloop-dev/chainloop/internal/blobmanager/mocks"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	credentialsM "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type ociRepositoryTestSuite struct {
	suite.Suite
	validUUID       uuid.UUID
	invalidUUID     string
	useCase         *biz.OCIRepositoryUseCase
	repo            *bizMocks.OCIRepositoryRepo
	credsRW         *credentialsM.ReaderWriter
	backendProvider *blobM.Provider
}

func (s *ociRepositoryTestSuite) TestFindMainRepoErr() {
	repo, err := s.useCase.FindMainRepo(context.Background(), s.invalidUUID)
	assert.True(s.T(), biz.IsErrInvalidUUID(err))
	assert.Nil(s.T(), repo)
}

func (s *ociRepositoryTestSuite) TestFindMainRepoNotFound() {
	assert := assert.New(s.T())

	// Not found
	ctx := context.Background()
	s.repo.On("FindMainRepo", ctx, s.validUUID).Return(nil, nil)

	repo, err := s.useCase.FindMainRepo(ctx, s.validUUID.String())
	assert.NoError(err)
	assert.Nil(repo)
}

func (s *ociRepositoryTestSuite) TestFindMainRepoFound() {
	assert := assert.New(s.T())

	ctx := context.Background()
	wantRepo := &biz.OCIRepository{}
	s.repo.On("FindMainRepo", ctx, s.validUUID).Return(wantRepo, nil)

	repo, err := s.useCase.FindMainRepo(ctx, s.validUUID.String())
	assert.NoError(err)
	assert.Equal(repo, wantRepo)
}

func (s *ociRepositoryTestSuite) TestSaveInvalidUUID() {
	repo, err := s.useCase.CreateOrUpdate(context.Background(), s.invalidUUID, "", "", "")
	assert.True(s.T(), biz.IsErrInvalidUUID(err))
	assert.Nil(s.T(), repo)
}

// If a repo exists it will get updated
func (s *ociRepositoryTestSuite) TestSaveMainRepoAlreadyExist() {
	assert := assert.New(s.T())
	const repoName, username, password = "repo", "username", "pass"

	r := &biz.OCIRepository{ID: s.validUUID.String()}
	ctx := context.Background()
	s.repo.On("FindMainRepo", ctx, s.validUUID).Return(r, nil)
	s.credsRW.On("SaveCredentials", ctx, s.validUUID.String(), mock.Anything).Return("secret-key", nil)
	s.repo.On("Update", ctx, &biz.OCIRepoUpdateOpts{
		ID: s.validUUID,
		OCIRepoOpts: &biz.OCIRepoOpts{
			Repository: repoName, Username: username, Password: password, SecretName: "secret-key",
		},
	}).Return(r, nil)

	gotRepo, err := s.useCase.CreateOrUpdate(ctx, s.validUUID.String(), repoName, username, password)
	assert.NoError(err)
	assert.Equal(gotRepo, r)
}

func (s *ociRepositoryTestSuite) TestSaveMainRepoOk() {
	assert := assert.New(s.T())

	ctx := context.Background()
	const repo, username, password = "repo", "username", "pass"

	s.repo.On("FindMainRepo", ctx, s.validUUID).Return(nil, nil)
	s.credsRW.On("SaveCredentials", ctx, s.validUUID.String(), mock.Anything).Return("secret-key", nil)

	newRepo := &biz.OCIRepository{}
	s.repo.On("Create", ctx, &biz.OCIRepoCreateOpts{
		OrgID: s.validUUID,
		OCIRepoOpts: &biz.OCIRepoOpts{
			Repository: repo, Username: username, Password: password, SecretName: "secret-key",
		},
	}).Return(newRepo, nil)

	gotRepo, err := s.useCase.CreateOrUpdate(ctx, s.validUUID.String(), repo, username, password)
	assert.NoError(err)
	assert.Equal(gotRepo, newRepo)
}

func (s *ociRepositoryTestSuite) TestPerformValidation() {
	assert := assert.New(s.T())
	t := s.T()
	validRepo := &biz.OCIRepository{ID: s.validUUID.String(), ValidationStatus: biz.OCIRepoValidationOK}

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

	t.Run("credentials missing, set validation status => invalid", func(t *testing.T) {
		s.repo.On("FindByID", mock.Anything, s.validUUID).Return(validRepo, nil)
		s.repo.On("UpdateValidationStatus", mock.Anything, s.validUUID, biz.OCIRepoValidationFailed).Return(nil)
		s.backendProvider.On("FromCredentials", mock.Anything, mock.Anything).Return(nil, credentials.ErrNotFound)
		err := s.useCase.PerformValidation(context.Background(), s.validUUID.String())
		assert.NoError(err)
		s.resetMock()
	})

	t.Run("invalid credentials, set validation status => invalid", func(t *testing.T) {
		b := blobM.NewUploaderDownloader(t)

		s.repo.On("FindByID", mock.Anything, s.validUUID).Return(validRepo, nil)
		s.repo.On("UpdateValidationStatus", mock.Anything, s.validUUID, biz.OCIRepoValidationFailed).Return(nil)
		s.backendProvider.On("FromCredentials", mock.Anything, mock.Anything).Return(b, nil)
		b.On("CheckWritePermissions", mock.Anything).Return(errors.New("invalid credentials"))

		err := s.useCase.PerformValidation(context.Background(), s.validUUID.String())
		assert.NoError(err)
		s.resetMock()
	})

	t.Run("valid credentials, set validation status => ok", func(t *testing.T) {
		b := blobM.NewUploaderDownloader(t)

		s.repo.On("FindByID", mock.Anything, s.validUUID).Return(validRepo, nil)
		s.repo.On("UpdateValidationStatus", mock.Anything, s.validUUID, biz.OCIRepoValidationOK).Return(nil)
		s.backendProvider.On("FromCredentials", mock.Anything, mock.Anything).Return(b, nil)
		b.On("CheckWritePermissions", mock.Anything).Return(nil)

		err := s.useCase.PerformValidation(context.Background(), s.validUUID.String())
		assert.NoError(err)
		s.resetMock()
	})
}

// Run all the tests
func TestOCIRepository(t *testing.T) {
	suite.Run(t, new(ociRepositoryTestSuite))
}

func (s *ociRepositoryTestSuite) resetMock() {
	s.repo.Mock = mock.Mock{}
	s.credsRW.Mock = mock.Mock{}
	s.backendProvider.Mock = mock.Mock{}
}

func (s *ociRepositoryTestSuite) SetupTest() {
	s.validUUID = uuid.New()
	s.invalidUUID = "deadbeef"
	s.repo = bizMocks.NewOCIRepositoryRepo(s.T())
	s.credsRW = credentialsM.NewReaderWriter(s.T())
	s.backendProvider = blobM.NewProvider(s.T())
	s.useCase = biz.NewOCIRepositoryUsecase(s.repo, s.credsRW, s.backendProvider, nil)
}
