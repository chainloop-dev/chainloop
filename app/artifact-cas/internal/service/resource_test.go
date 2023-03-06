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
	"github.com/chainloop-dev/chainloop/internal/blobmanager/mocks"
	casjwt "github.com/chainloop-dev/chainloop/internal/robotaccount/cas"
	jwtmiddleware "github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *resourceSuite) TestDescribe() {
	want := &v1.CASResource{FileName: "test.txt", Digest: "deadbeef"}

	s.ociProvider.On("Describe", mock.Anything, "deadbeef").
		Return(want, nil)

	ctx := jwtmiddleware.NewContext(context.Background(), &casjwt.Claims{StoredSecretID: "secret-id"})

	svc := service.NewResourceService(s.backendProvider)
	got, err := svc.Describe(ctx, &v1.ResourceServiceDescribeRequest{
		Digest: "deadbeef",
	})

	s.NoError(err)
	s.Equal(&v1.ResourceServiceDescribeResponse{Result: want}, got)
}

type resourceSuite struct {
	suite.Suite
	ociProvider     *mocks.UploaderDownloader
	backendProvider *mocks.Provider
}

func (s *resourceSuite) SetupTest() {
	backendProvider := mocks.NewProvider(s.T())
	ociProvider := mocks.NewUploaderDownloader(s.T())
	backendProvider.On("FromCredentials", mock.Anything, "secret-id").
		Return(ociProvider, nil)

	s.ociProvider = ociProvider
	s.backendProvider = backendProvider
}

// Run the tests
func TestResourceSuite(t *testing.T) {
	suite.Run(t, new(resourceSuite))
}
