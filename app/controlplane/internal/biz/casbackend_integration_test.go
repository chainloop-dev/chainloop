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
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *CASBackendIntegrationTestSuite) TestList() {
	testCases := []struct {
		name           string
		orgID          string
		expectedResult []*biz.CASBackend
	}{
		{
			name:           "non-existent org",
			orgID:          uuid.New().String(),
			expectedResult: []*biz.CASBackend{},
		},
		{
			name:           "no backends for org",
			orgID:          s.orgNoBackend.ID,
			expectedResult: []*biz.CASBackend{},
		},
		{
			name:  "one backend for org",
			orgID: s.orgOne.ID,
			expectedResult: []*biz.CASBackend{
				s.casBackend1,
			},
		},
		{
			name:  "backend 2 in org",
			orgID: s.orgTwo.ID,
			expectedResult: []*biz.CASBackend{
				s.casBackend2,
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			assert := assert.New(s.T())
			ctx := context.Background()
			backends, err := s.TestingUseCases.CASBackend.List(ctx, tc.orgID)
			assert.NoError(err)
			assert.Equal(tc.expectedResult, backends)
		})
	}
}

func (s *CASBackendIntegrationTestSuite) SetupTest() {
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()
	// OCI repository credentials
	credsWriter := creds.NewReaderWriter(s.T())
	credsWriter.On(
		"SaveCredentials", ctx, mock.Anything, mock.Anything,
	).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(credsWriter))

	s.orgOne, err = s.Organization.Create(ctx, "testing org 1")
	assert.NoError(err)
	s.orgTwo, err = s.Organization.Create(ctx, "testing org 2")
	assert.NoError(err)
	s.orgNoBackend, err = s.Organization.Create(ctx, "testing org 3")
	assert.NoError(err)

	s.casBackend1, err = s.CASBackend.CreateOrUpdate(ctx, s.orgOne.ID, "backend 1", "username", "pass", biz.CASBackendOCI, true)
	assert.NoError(err)
	s.casBackend2, err = s.CASBackend.CreateOrUpdate(ctx, s.orgTwo.ID, "backend 2", "username", "pass", biz.CASBackendOCI, true)
	assert.NoError(err)
}

func TestCASBackendUseCase(t *testing.T) {
	suite.Run(t, new(CASBackendIntegrationTestSuite))
}

type CASBackendIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	orgTwo, orgOne, orgNoBackend *biz.Organization
	casBackend1, casBackend2     *biz.CASBackend
}

func TestIntegrationCASBackend(t *testing.T) {
	suite.Run(t, new(CASBackendIntegrationTestSuite))
}
