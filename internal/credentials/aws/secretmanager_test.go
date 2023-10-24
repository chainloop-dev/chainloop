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

package aws

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/aws/smithy-go"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	mclient "github.com/chainloop-dev/chainloop/internal/credentials/aws/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func (s *testSuite) TestNewManager() {
	assert := assert.New(s.T())

	testCases := []struct {
		name          string
		region        string
		accessKey     string
		secretKey     string
		path          string
		expectedError bool
	}{
		{name: "missing region", region: "", accessKey: "ak", secretKey: "sk", expectedError: true},
		{name: "missing accessKey", region: "r", accessKey: "", secretKey: "sk", expectedError: true},
		{name: "missing secretKey", region: "r", accessKey: "ak", secretKey: "", expectedError: true},
		{name: "valid manager", region: "r", accessKey: "ak", secretKey: "sk", path: "foo"},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			opts := &NewManagerOpts{Region: tc.region, AccessKey: tc.accessKey, SecretKey: tc.secretKey, SecretPrefix: tc.path}
			_, err := NewManager(opts)
			if tc.expectedError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

const orgID = "test-org"
const defaultRegion = "default-region"
const defaultAccessKey = "access-key-not-a-real-key"
const defaultSecretKey = "secret-key-not-a-real-key"

func (s *testSuite) TestReadCredentialsErrorHandling() {
	fakeSecretID := "fakeSecretID"
	genericErr := errors.New("generic error")
	genericAPIErr := &smithy.GenericAPIError{Code: "AnotherAPIError", Message: "Some message"}

	testCases := []struct {
		name          string
		wantedError   error
		expectedError error
	}{
		{
			"GetSecretValue returns no error",
			nil,
			nil,
		}, {
			"GetSecretValue returns a smithy.APIError error of type 'resource not found'",
			&smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: "Some message"},
			credentials.ErrNotFound,
		}, {
			"GetSecretValue returns a smithy.APIError error of type 'other type'",
			genericAPIErr,
			genericAPIErr,
		}, {
			"GetSecretValue returns an error that is not smithy.APIError",
			genericErr,
			genericErr,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// re-set the manager mocked expectations
			initMockedManager(s)
			m := s.mockedManager
			mc, _ := m.client.(*mclient.SecretsManagerIface)
			ctx := context.Background()

			// mock response for method GetSecretValue(..)
			var getSecretValueResp secretsmanager.GetSecretValueOutput
			if tc.wantedError == nil {
				validAPICreds := &credentials.APICreds{Host: "h", Key: "k"}
				mockedResp, _ := json.Marshal(validAPICreds)
				getSecretValueResp = secretsmanager.GetSecretValueOutput{
					SecretString: aws.String(string(mockedResp)),
				}
			}

			// mock call to GetSecretValue to return the wanted error
			mc.On("GetSecretValue", ctx, &secretsmanager.GetSecretValueInput{
				SecretId: aws.String(fakeSecretID),
			}).Return(&getSecretValueResp, tc.wantedError)

			// call
			creds := &credentials.APICreds{}
			err := m.ReadCredentials(ctx, fakeSecretID, creds)

			// test
			if tc.expectedError == nil {
				require.NoError(s.T(), err)
			} else {
				require.ErrorIs(s.T(), err, tc.expectedError)
			}
		})
	}
}

func (s *testSuite) TestReadWriteCredentials() {
	assert := assert.New(s.T())
	validOCICreds := &credentials.OCIKeypair{Repo: "test-repo", Username: "username", Password: "password"}
	validAPICreds := &credentials.APICreds{Host: "h", Key: "k"}

	testCases := []struct {
		name          string
		want          any
		path          string
		expectedError bool
	}{
		{"valid OCI creds", validOCICreds, "", false},
		{"valid OCI creds custom path", validOCICreds, "fooo", false},
		{"valid API creds", validAPICreds, "", false},
		{"valid API creds custom path", validAPICreds, "fooo", false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Re-set the manager mocked expectations
			initMockedManager(s)
			m := s.mockedManager
			mc, _ := m.client.(*mclient.SecretsManagerIface)
			ctx := context.Background()

			mc.On("CreateSecret", ctx, mock.Anything).Return(nil, nil)
			secretName, err := m.SaveCredentials(ctx, orgID, tc.want)
			if tc.expectedError {
				assert.Error(err)
				return
			}

			mockedResp, err := json.Marshal(tc.want)
			require.NoError(s.T(), err)

			// Read the keypair
			mc.On("GetSecretValue", ctx, &secretsmanager.GetSecretValueInput{
				SecretId: aws.String(secretName),
			}).Return(&secretsmanager.GetSecretValueOutput{SecretString: aws.String(string(mockedResp))}, nil)

			// Choose the returning struct
			var got any
			switch reflect.TypeOf(tc.want).String() {
			case "*credentials.APICreds":
				got = &credentials.APICreds{}
			case "*credentials.OCIKeypair":
				got = &credentials.OCIKeypair{}
			}

			err = m.ReadCredentials(ctx, secretName, got)
			assert.NoError(err)

			// Compare the keypair
			assert.Equal(tc.want, got)

			// Not found error
			mc.On("GetSecretValue", ctx, &secretsmanager.GetSecretValueInput{
				SecretId: aws.String("invalid"),
			}).Return(nil, &types.ResourceNotFoundException{})

			err = m.ReadCredentials(ctx, "invalid", got)
			assert.Error(err)
			assert.ErrorIs(err, credentials.ErrNotFound)
		})
	}
}

// // Create a new secret, delete it and check it does not exist antymore
func (s *testSuite) TestDeleteCredentials() {
	assert := assert.New(s.T())
	m := s.mockedManager
	mc, _ := m.client.(*mclient.SecretsManagerIface)
	ctx := context.Background()
	secretName := "test-secret"

	mc.On("DeleteSecret", ctx, &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(secretName),
	}).Return(nil, nil)

	err := m.DeleteCredentials(ctx, secretName)
	assert.NoError(err)
}

type testSuite struct {
	suite.Suite
	mockedManager *Manager
}

// Run before each test
func (s *testSuite) SetupTest() {
	initMockedManager(s)
}

func initMockedManager(s *testSuite) {
	opts := &NewManagerOpts{Region: defaultRegion, AccessKey: defaultAccessKey, SecretKey: defaultSecretKey}
	m, err := NewManager(opts)
	require.NoError(s.T(), err)
	m.client = mclient.NewSecretsManagerIface(s.T())
	s.mockedManager = m
}

// Run the tests
func TestAWSSecretManager(t *testing.T) {
	suite.Run(t, new(testSuite))
}
