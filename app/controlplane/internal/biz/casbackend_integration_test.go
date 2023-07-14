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

const location = "my-location"
const description = "my-description"
const backendType = biz.CASBackendOCI

func (s *CASBackendIntegrationTestSuite) TestCreate() {
	assert := assert.New(s.T())
	orgID := s.orgOne.ID

	s.Run("non-existing org", func() {
		_, err := s.CASBackend.Create(
			context.TODO(), uuid.NewString(), location, description, backendType, nil, true,
		)
		assert.Error(err)
	})

	s.Run("create default", func() {
		b, err := s.CASBackend.Create(context.TODO(), orgID, location, description, backendType, nil, true)
		assert.NoError(err)

		if diff := cmp.Diff(&biz.CASBackend{
			Location:         location,
			Description:      description,
			SecretName:       "stored-OCI-secret",
			Provider:         backendType,
			ValidationStatus: "OK",
			Default:          true,
		}, b,
			cmpopts.IgnoreFields(biz.CASBackend{}, "CreatedAt", "ID", "ValidatedAt", "OrganizationID"),
		); diff != "" {
			assert.Failf("mismatch (-want +got):\n%s", diff)
		}
	})
}
func (s *CASBackendIntegrationTestSuite) TestCreateOverride() {
	assert := assert.New(s.T())

	// When a new default backend is created, the previous default should be overridden
	b1, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, location, description, backendType, nil, true)
	assert.NoError(err)
	assert.True(b1.Default)
	b2, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, "another-location", description, backendType, nil, true)
	assert.NoError(err)
	assert.True(b2.Default)

	// Check that the first one is no longer default
	b1, err = s.TestingUseCases.CASBackend.FindByIDInOrg(context.TODO(), s.orgNoBackend.ID, b1.ID.String())
	assert.NoError(err)
	assert.False(b1.Default)
}

func (s *CASBackendIntegrationTestSuite) TestUpdate() {
	assert := assert.New(s.T())

	s.Run("overrides previous backends", func() {
		// When a new default backend is set, the previous default should be overridden
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, location, description, backendType, nil, true)
		assert.NoError(err)
		assert.True(defaultB.Default)
		nonDefaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, "another-location", description, backendType, nil, false)
		assert.NoError(err)
		assert.False(nonDefaultB.Default)

		// Update the non-default to be default
		nonDefaultB, err = s.TestingUseCases.CASBackend.Update(context.TODO(), s.orgNoBackend.ID, nonDefaultB.ID.String(), "", nil, true)
		assert.NoError(err)
		assert.True(nonDefaultB.Default)

		// Check that the first one is no longer default
		defaultB, err = s.TestingUseCases.CASBackend.FindByIDInOrg(context.TODO(), s.orgNoBackend.ID, defaultB.ID.String())
		assert.NoError(err)
		assert.False(defaultB.Default)
	})

	s.Run("can update only the description", func() {
		// When a new default backend is set, the previous default should be overridden
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, location, description, backendType, nil, true)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)

		// Update the description
		defaultB, err = s.TestingUseCases.CASBackend.Update(context.TODO(), s.orgNoBackend.ID, defaultB.ID.String(), "updated desc", nil, true)
		assert.NoError(err)
		assert.Equal("updated desc", defaultB.Description)
		assert.True(defaultB.Default)
	})

	s.Run("can update only the status", func() {
		// When a new default backend is set, the previous default should be overridden
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, location, description, backendType, nil, true)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)

		// update the status
		defaultB, err = s.TestingUseCases.CASBackend.Update(context.TODO(), s.orgNoBackend.ID, defaultB.ID.String(), description, nil, false)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)
		assert.False(defaultB.Default)
	})

	s.Run("can rotate credentials", func() {
		// When a new default backend is set, the previous default should be overridden
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, location, description, backendType, nil, true)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)

		// update the secret
		creds := struct{}{}
		ctx := context.TODO()
		s.credsWriter.Mock = mock.Mock{}
		s.credsWriter.On("SaveCredentials", ctx, s.orgNoBackend.ID, creds).Return("new-secret", nil)
		defaultB, err = s.TestingUseCases.CASBackend.Update(ctx, s.orgNoBackend.ID, defaultB.ID.String(), description, creds, true)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)
		assert.Equal("new-secret", defaultB.SecretName)
		assert.True(defaultB.Default)
	})
}

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
			name:  "2 backends in org",
			orgID: s.orgTwo.ID,
			expectedResult: []*biz.CASBackend{
				s.casBackend3,
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
	s.credsWriter = creds.NewReaderWriter(s.T())
	s.credsWriter.On(
		"SaveCredentials", ctx, mock.Anything, mock.Anything,
	).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(s.credsWriter))

	s.orgOne, err = s.Organization.Create(ctx, "testing org 1 with one backend")
	assert.NoError(err)
	s.orgTwo, err = s.Organization.Create(ctx, "testing org 2 with 2 backends")
	assert.NoError(err)
	s.orgNoBackend, err = s.Organization.Create(ctx, "testing org 3, no backends")
	assert.NoError(err)

	s.casBackend1, err = s.CASBackend.Create(ctx, s.orgOne.ID, "my-location", "backend 1 description", biz.CASBackendOCI, nil, true)
	assert.NoError(err)
	s.casBackend2, err = s.CASBackend.Create(ctx, s.orgTwo.ID, "my-location 2", "backend 2 description", biz.CASBackendOCI, nil, true)
	assert.NoError(err)
	s.casBackend3, err = s.CASBackend.Create(ctx, s.orgTwo.ID, "my-location 3", "backend 3 description", biz.CASBackendOCI, nil, false)
	assert.NoError(err)
}

func TestCASBackendUseCase(t *testing.T) {
	suite.Run(t, new(CASBackendIntegrationTestSuite))
}

type CASBackendIntegrationTestSuite struct {
	testhelpers.UseCasesEachTestSuite
	orgTwo, orgOne, orgNoBackend          *biz.Organization
	casBackend1, casBackend2, casBackend3 *biz.CASBackend
	credsWriter                           *creds.ReaderWriter
}

func TestIntegrationCASBackend(t *testing.T) {
	suite.Run(t, new(CASBackendIntegrationTestSuite))
}
