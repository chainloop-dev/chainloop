//
// Copyright 2023-2026 The Chainloop Authors.
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

package service_test

import (
	"context"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	casjwt "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/mocks"
	"github.com/go-kratos/kratos/v2/errors"
	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *resourceSuite) TestDescribe() {
	want := &v1.CASResource{FileName: "test.txt", Digest: "deadbeef"}

	s.ociBackend.On("Describe", mock.Anything, "deadbeef").Return(want, nil)
	ctx := jwtmiddleware.NewContext(context.Background(), &casjwt.Claims{StoredSecretID: "secret-id", BackendType: "backend1", Role: casjwt.Downloader})

	svc := service.NewResourceService(s.backendProviders)
	got, err := svc.Describe(ctx, &v1.ResourceServiceDescribeRequest{
		Digest: "deadbeef",
	})

	s.NoError(err)
	s.Equal(&v1.ResourceServiceDescribeResponse{Result: want}, got)
}

func (s *resourceSuite) TestDescribeAuthorization() {
	const (
		digest      = "b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"
		otherDigest = "7d865e959b2466918c9863afca942d0fb89d7c9ac0c99bafc3749504ded97730"
	)

	testCases := []struct {
		name    string
		role    casjwt.Role
		digest  string
		wantErr bool
	}{
		{name: "downloader without digest claim", role: casjwt.Downloader},
		{name: "downloader scoped to the digest", role: casjwt.Downloader, digest: "sha256:" + digest},
		{name: "downloader scoped to another digest", role: casjwt.Downloader, digest: "sha256:" + otherDigest, wantErr: true},
		{name: "uploader", role: casjwt.Uploader, wantErr: true},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			want := &v1.CASResource{FileName: "test.txt", Digest: digest}
			// the mock fails the test on a describe of a forbidden digest
			if !tc.wantErr {
				s.ociBackend.On("Describe", mock.Anything, digest).Return(want, nil)
			}

			claims := &casjwt.Claims{StoredSecretID: "secret-id", BackendType: "backend1", Role: tc.role, Digest: tc.digest}
			ctx := jwtmiddleware.NewContext(context.Background(), claims)
			svc := service.NewResourceService(s.backendProviders)
			got, err := svc.Describe(ctx, &v1.ResourceServiceDescribeRequest{Digest: digest})
			if tc.wantErr {
				s.True(errors.IsForbidden(err) || errors.IsUnauthorized(err), "unexpected error: %v", err)
				s.Nil(got)
				return
			}

			s.NoError(err)
			s.Equal(want, got.GetResult())
		})
	}
}

type resourceSuite struct {
	suite.Suite
	ociBackend       *mocks.UploaderDownloader
	backendProviders backend.Providers
}

func (s *resourceSuite) SetupTest() {
	ociBackendProvider := mocks.NewProvider(s.T())
	ociBackend := mocks.NewUploaderDownloader(s.T())
	ociBackendProvider.On("FromCredentials", mock.Anything, "secret-id").
		Maybe().Return(ociBackend, nil)

	s.ociBackend = ociBackend
	s.backendProviders = backend.Providers{
		"backend1": ociBackendProvider,
	}
}

// SetupSubTest gives every s.Run subtest its own backend mocks
func (s *resourceSuite) SetupSubTest() {
	s.SetupTest()
}

// Run the tests
func TestResourceSuite(t *testing.T) {
	suite.Run(t, new(resourceSuite))
}
