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

package service_test

import (
	"context"
	"testing"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/artifact-cas/internal/service"
	casjwt "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/blobmanager/mocks"
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

type resourceSuite struct {
	suite.Suite
	ociBackend       *mocks.UploaderDownloader
	backendProviders backend.Providers
}

func (s *resourceSuite) SetupTest() {
	ociBackendProvider := mocks.NewProvider(s.T())
	ociBackend := mocks.NewUploaderDownloader(s.T())
	ociBackendProvider.On("FromCredentials", mock.Anything, "secret-id").
		Return(ociBackend, nil)

	s.ociBackend = ociBackend
	s.backendProviders = backend.Providers{
		"backend1": ociBackendProvider,
	}
}

// Run the tests
func TestResourceSuite(t *testing.T) {
	suite.Run(t, new(resourceSuite))
}
