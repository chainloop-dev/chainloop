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
	"fmt"
	"testing"

	"github.com/chainloop-dev/chainloop/app/controlplane/integrations"
	integrationMocks "github.com/chainloop-dev/chainloop/app/controlplane/integrations/mocks"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz/testhelpers"
	creds "github.com/chainloop-dev/chainloop/internal/credentials/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func (s *testSuite) TestCreate() {
	const kind = "my-integration"
	assert := assert.New(s.T())

	config, err := structpb.NewValue(map[string]interface{}{
		"firstName": "John",
	})

	assert.NoError(err)

	configAny, err := anypb.New(config)
	assert.NoError(err)

	// Mocked integration that will return both generic configuration and credentials
	integration := integrationMocks.NewRegistrable(s.T())

	ctx := context.Background()
	integration.On("PreRegister", ctx, configAny).Return(&integrations.PreRegistration{
		Configuration: config, Kind: kind, Credentials: &integrations.Credentials{
			Password: "key", URL: "host"},
	}, nil)

	got, err := s.Integration.Create(ctx, s.org.ID, integration, configAny)
	assert.NoError(err)
	fmt.Println(got)
	assert.Equal(kind, got.Kind)

	// Check stored configuration
	gotConfig := new(structpb.Value)
	err = got.Config.UnmarshalTo(gotConfig)
	assert.NoError(err)
	// Check configuration was stored
	assert.Equal("John", gotConfig.GetStructValue().Fields["firstName"].GetStringValue())
	// Check credential was stored
	assert.Equal("stored-integration-secret", got.SecretName)
}
func (s *testSuite) SetupTest() {
	t := s.T()
	var err error
	assert := assert.New(s.T())
	ctx := context.Background()

	// Override credentials writer to set expectations
	s.mockedCredsReaderWriter = creds.NewReaderWriter(t)
	// Mock API call to store credentials

	// Dependency-track integration credentials
	s.mockedCredsReaderWriter.On(
		"SaveCredentials", ctx, mock.Anything, &integrations.Credentials{URL: "host", Password: "key"},
	).Return("stored-integration-secret", nil)

	s.TestingUseCases = testhelpers.NewTestingUseCases(t, testhelpers.WithCredsReaderWriter(s.mockedCredsReaderWriter))

	// Create org, integration and oci repository
	s.org, err = s.Organization.Create(ctx, "testing org")
	assert.NoError(err)
}

// Run the tests
func TestIntegration(t *testing.T) {
	suite.Run(t, new(testSuite))
}

// Utility struct to hold the test suite
type testSuite struct {
	testhelpers.UseCasesEachTestSuite
	org                     *biz.Organization
	mockedCredsReaderWriter *creds.ReaderWriter
}
