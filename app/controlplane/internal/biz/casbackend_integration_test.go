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

	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/oci"
	creds "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const location = "my-location"
const description = "my-description"
const backendType = oci.ProviderID

func (s *CASBackendIntegrationTestSuite) TestUniqueNameDuringCreate() {
	orgID, err := uuid.Parse(s.orgOne.ID)
	require.NoError(s.T(), err)

	testCases := []struct {
		name       string
		opts       *biz.CASBackendOpts
		wantErrMsg string
	}{
		{
			name:       "org missing",
			opts:       &biz.CASBackendOpts{Name: "name"},
			wantErrMsg: "required",
		},
		{
			name:       "name missing",
			opts:       &biz.CASBackendOpts{OrgID: orgID},
			wantErrMsg: "required",
		},
		{
			name:       "invalid name",
			opts:       &biz.CASBackendOpts{OrgID: orgID, Name: "this/not/valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name:       "another invalid name",
			opts:       &biz.CASBackendOpts{OrgID: orgID, Name: "this-not Valid"},
			wantErrMsg: "RFC 1123",
		},
		{
			name: "can create it with just the name and the org",
			opts: &biz.CASBackendOpts{OrgID: orgID, Name: "name"},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			orgID := tc.opts.OrgID.String()
			if uuid.Nil == tc.opts.OrgID {
				orgID = ""
			}

			got, err := s.CASBackend.Create(context.Background(), orgID, tc.opts.Name, location, description, backendType, nil, true)
			if tc.wantErrMsg != "" {
				s.ErrorContains(err, tc.wantErrMsg)
				return
			}

			require.NoError(s.T(), err)
			s.NotEmpty(got.ID)
			s.Equal(tc.opts.Name, got.Name)
		})
	}
}

func (s *CASBackendIntegrationTestSuite) TestCreate() {
	assert := assert.New(s.T())
	orgID := s.orgOne.ID

	s.Run("non-existing org", func() {
		_, err := s.CASBackend.Create(
			context.TODO(), uuid.NewString(), randomName(), location, description, backendType, nil, true,
		)
		assert.Error(err)
	})

	s.Run("create default", func() {
		b, err := s.CASBackend.Create(context.TODO(), orgID, "my-name", location, description, backendType, nil, true)
		assert.NoError(err)

		if diff := cmp.Diff(&biz.CASBackend{
			Location:         location,
			Name:             "my-name",
			Description:      description,
			SecretName:       "stored-OCI-secret",
			Provider:         backendType,
			ValidationStatus: "OK",
			Default:          true,
			Inline:           false,
			Limits: &biz.CASBackendLimits{
				MaxBytes: 104857600,
			},
		}, b,
			cmpopts.IgnoreFields(biz.CASBackend{}, "CreatedAt", "ID", "ValidatedAt", "OrganizationID"),
		); diff != "" {
			assert.Failf("mismatch (-want +got):\n%s", diff)
		}
	})

	s.Run("create fallback", func() {
		b, err := s.CASBackend.CreateInlineFallbackBackend(context.TODO(), orgID)
		assert.NoError(err)

		if diff := cmp.Diff(&biz.CASBackend{
			Description:      "Embed artifacts content in the attestation (fallback)",
			Provider:         biz.CASBackendInline,
			Name:             "default-inline",
			Default:          true,
			Inline:           true,
			Fallback:         true,
			ValidationStatus: "OK",
			Limits: &biz.CASBackendLimits{
				MaxBytes: 512000,
			},
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
	b1, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), location, description, backendType, nil, true)
	assert.NoError(err)
	assert.True(b1.Default)
	b2, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), "another-location", description, backendType, nil, true)
	assert.NoError(err)
	assert.True(b2.Default)

	// Check that the first one is no longer default
	b1, err = s.CASBackend.FindByIDInOrg(context.TODO(), s.orgNoBackend.ID, b1.ID.String())
	assert.NoError(err)
	assert.False(b1.Default)
}

func randomName() string {
	return uuid.New().String()
}

func (s *CASBackendIntegrationTestSuite) TestUpdate() {
	assert := assert.New(s.T())

	s.Run("overrides previous backends", func() {
		// When a new default backend is set, the previous default should be overridden
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), location, description, backendType, nil, true)
		assert.NoError(err)
		assert.True(defaultB.Default)
		nonDefaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), "another-location", description, backendType, nil, false)
		assert.NoError(err)
		assert.False(nonDefaultB.Default)

		// Update the non-default to be default
		nonDefaultB, err = s.CASBackend.Update(context.TODO(), s.orgNoBackend.ID, nonDefaultB.ID.String(), "", "", nil, true)
		assert.NoError(err)
		assert.True(nonDefaultB.Default)

		// Check that the first one is no longer default
		defaultB, err = s.CASBackend.FindByIDInOrg(context.TODO(), s.orgNoBackend.ID, defaultB.ID.String())
		assert.NoError(err)
		assert.False(defaultB.Default)
	})

	s.Run("can update only the name", func() {
		// When a new default backend is set, the previous default should be overridden
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), location, description, backendType, nil, true)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)

		// Update the description
		defaultB, err = s.CASBackend.Update(context.TODO(), s.orgNoBackend.ID, defaultB.ID.String(), "updated-name", "", nil, true)
		assert.NoError(err)
		assert.Equal("updated-name", defaultB.Name)
		assert.True(defaultB.Default)
	})

	s.Run("can update only the description", func() {
		// When a new default backend is set, the previous default should be overridden
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), location, description, backendType, nil, true)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)

		// Update the description
		defaultB, err = s.CASBackend.Update(context.TODO(), s.orgNoBackend.ID, defaultB.ID.String(), "", "updated desc", nil, true)
		assert.NoError(err)
		assert.Equal("updated desc", defaultB.Description)
		assert.True(defaultB.Default)
	})

	s.Run("can update only the status", func() {
		// When a new default backend is set, the previous default should be overridden
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), location, description, backendType, nil, true)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)

		// update the status
		defaultB, err = s.CASBackend.Update(context.TODO(), s.orgNoBackend.ID, defaultB.ID.String(), "", description, nil, false)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)
		assert.False(defaultB.Default)
	})

	s.Run("the fallback backend will be set if default true => false", func() {
		// When a new default backend is set, the previous default should be overridden
		fallbackB, err := s.CASBackend.CreateInlineFallbackBackend(context.TODO(), s.orgNoBackend.ID)
		assert.NoError(err)
		assert.True(fallbackB.Fallback)
		assert.True(fallbackB.Default)

		// Create a new default backend
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), location, description, backendType, nil, true)
		assert.NoError(err)
		assert.False(defaultB.Fallback) // it's not fallback
		assert.True(defaultB.Default)

		// The fallback now is no longer the default
		fallbackB, err = s.CASBackend.FindFallbackBackend(context.TODO(), s.orgNoBackend.ID)
		assert.NoError(err)
		assert.False(fallbackB.Default)

		// update the status
		defaultB, err = s.CASBackend.Update(context.TODO(), s.orgNoBackend.ID, defaultB.ID.String(), "", description, nil, false)
		assert.NoError(err)
		assert.False(defaultB.Default)

		// The fallback is now the default
		fallbackB, err = s.CASBackend.FindFallbackBackend(context.TODO(), s.orgNoBackend.ID)
		assert.NoError(err)
		assert.True(fallbackB.Default)
	})

	s.Run("can rotate credentials", func() {
		// When a new default backend is set, the previous default should be overridden
		defaultB, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), location, description, backendType, nil, true)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)

		// update the secret
		creds := struct{}{}
		ctx := context.TODO()
		s.credsWriter.Mock = mock.Mock{}
		s.credsWriter.On("SaveCredentials", ctx, s.orgNoBackend.ID, creds).Return("new-secret", nil)
		defaultB, err = s.CASBackend.Update(ctx, s.orgNoBackend.ID, defaultB.ID.String(), "", description, creds, true)
		assert.NoError(err)
		assert.Equal(description, defaultB.Description)
		assert.Equal("new-secret", defaultB.SecretName)
		assert.True(defaultB.Default)
	})
}

func (s *CASBackendIntegrationTestSuite) TestSoftDelete() {
	assert := assert.New(s.T())
	ctx := context.TODO()

	backends, err := s.CASBackend.List(ctx, s.orgTwo.ID)
	assert.NoError(err)
	// There are two backends
	require.Len(s.T(), backends, 2)

	// We are going to delete the default one
	toDelete := backends[1].ID
	assert.True(backends[1].Default)

	// Delete it
	err = s.CASBackend.SoftDelete(ctx, s.orgTwo.ID, toDelete.String())
	assert.NoError(err)

	// there is one left
	backends, err = s.CASBackend.List(ctx, s.orgTwo.ID)
	assert.NoError(err)
	// There is one backend
	require.Len(s.T(), backends, 1)
	assert.Equal(backends[0].ID, s.casBackend3.ID)

	// the deleted one can not be found by ID either
	_, err = s.CASBackend.FindByIDInOrg(ctx, s.orgTwo.ID, toDelete.String())
	assert.ErrorAs(err, &biz.ErrNotFound{})

	// the deleted one can not be found by as default
	_, err = s.CASBackend.FindDefaultBackend(ctx, s.orgTwo.ID)
	assert.ErrorAs(err, &biz.ErrNotFound{})
}

func (s *CASBackendIntegrationTestSuite) TestSoftDeleteFallbackOverride() {
	assert := assert.New(s.T())
	// We have two backends, one is fallback and another is default
	fallbackB, err := s.CASBackend.CreateInlineFallbackBackend(context.TODO(), s.orgNoBackend.ID)
	assert.NoError(err)
	assert.True(fallbackB.Default)

	// When a new default backend is set, the previous default should be overridden
	b, err := s.CASBackend.Create(context.TODO(), s.orgNoBackend.ID, randomName(), location, description, backendType, nil, true)
	assert.NoError(err)

	// The fallback is not the default anymore
	fallbackB, err = s.CASBackend.FindByIDInOrg(context.TODO(), s.orgNoBackend.ID, fallbackB.ID.String())
	assert.NoError(err)
	assert.False(fallbackB.Default)

	// Once we delete the default, the fallback should be the default again
	// Delete it
	err = s.CASBackend.SoftDelete(context.TODO(), s.orgNoBackend.ID, b.ID.String())
	assert.NoError(err)

	// The fallback is NOW THE DEFAULT
	fallbackB, err = s.CASBackend.FindByIDInOrg(context.TODO(), s.orgNoBackend.ID, fallbackB.ID.String())
	assert.NoError(err)
	assert.True(fallbackB.Default)
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
			backends, err := s.CASBackend.List(ctx, tc.orgID)
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
		"SaveCredentials", mock.Anything, mock.Anything, mock.Anything,
	).Return("stored-OCI-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(s.T(), testhelpers.WithCredsReaderWriter(s.credsWriter))

	s.orgOne, err = s.Organization.Create(ctx, "testing-org-1-with-one-backend")
	assert.NoError(err)
	s.orgTwo, err = s.Organization.Create(ctx, "testing-org-2-with-2-backends")
	assert.NoError(err)
	s.orgNoBackend, err = s.Organization.Create(ctx, "testing-org-3-no-backends")
	assert.NoError(err)

	s.casBackend1, err = s.CASBackend.Create(ctx, s.orgOne.ID, randomName(), "my-location", "backend 1 description", backendType, nil, true)
	assert.NoError(err)
	s.casBackend2, err = s.CASBackend.Create(ctx, s.orgTwo.ID, randomName(), "my-location 2", "backend 2 description", backendType, nil, true)
	assert.NoError(err)
	s.casBackend3, err = s.CASBackend.Create(ctx, s.orgTwo.ID, randomName(), "my-location 3", "backend 3 description", backendType, nil, false)
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
