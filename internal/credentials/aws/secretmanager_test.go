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
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sso/types"
	"github.com/chainloop-dev/bedrock/internal/credentials"
	mclient "github.com/chainloop-dev/bedrock/internal/credentials/aws/mocks"
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

func (s *testSuite) TestReadWriteOCICreds() {
	assert := assert.New(s.T())
	validCreds := &credentials.OCIKeypair{Repo: "test-repo", Username: "username", Password: "password"}
	//nolint:gosec
	// This is a test secret, it is not a real secret
	validCredsString := "{\"Repo\":\"test-repo\",\"Username\":\"username\",\"Password\":\"password\"}"

	testCases := []struct {
		name          string
		want          *credentials.OCIKeypair
		path          string
		expectedError bool
	}{
		{"empty secret", &credentials.OCIKeypair{}, "", true},
		{"missing repo", &credentials.OCIKeypair{Username: "un", Password: "p"}, "", true},
		{"missing username", &credentials.OCIKeypair{Username: "", Password: "p", Repo: "repo"}, "", true},
		{"missing password", &credentials.OCIKeypair{Username: "u", Password: "", Repo: "repo"}, "", true},
		{"valid creds", validCreds, "", false},
		{"valid creds custom path", validCreds, "fooo", false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			m := s.mockedManager
			mc, _ := m.client.(*mclient.SecretsManagerIface)
			ctx := context.Background()

			mc.On("CreateSecret", ctx, mock.Anything).Return(nil, nil)
			secretName, err := m.SaveOCICreds(ctx, orgID, tc.want)
			if tc.expectedError {
				assert.Error(err)
				return
			}

			assert.NoError(err)
			// Read the keypair
			got := &credentials.OCIKeypair{}
			mc.On("GetSecretValue", ctx, &secretsmanager.GetSecretValueInput{
				SecretId: aws.String(secretName),
			}).Return(&secretsmanager.GetSecretValueOutput{SecretString: aws.String(validCredsString)}, nil)

			err = m.ReadOCICreds(ctx, secretName, got)
			assert.NoError(err)

			// Compare the keypair
			assert.Equal(tc.want, got)

			// Not found error
			mc.On("GetSecretValue", ctx, &secretsmanager.GetSecretValueInput{
				SecretId: aws.String("invalid"),
			}).Return(nil, &types.ResourceNotFoundException{})

			err = m.ReadOCICreds(ctx, "invalid", got)
			assert.Error(err)
			assert.ErrorIs(err, credentials.ErrNotFound)
		})
	}
}

// // Create a new secret, delete it and check it does not exist antymore
func (s *testSuite) TestDeleteCreds() {
	assert := assert.New(s.T())
	m := s.mockedManager
	mc, _ := m.client.(*mclient.SecretsManagerIface)
	ctx := context.Background()
	secretName := "test-secret"

	mc.On("DeleteSecret", ctx, &secretsmanager.DeleteSecretInput{
		SecretId: aws.String(secretName),
	}).Return(nil, nil)

	err := m.DeleteCreds(ctx, secretName)
	assert.NoError(err)
}
func (s *testSuite) TestReadWriteAPICreds() {
	assert := assert.New(s.T())
	validCreds := &credentials.APICreds{Host: "http://hospath.local", Key: "api-key-not-a-secret"}
	//nolint:gosec
	// This is a test secret, it is not a real secret
	validCredsString := "{\"Host\":\"http://hospath.local\",\"Key\":\"api-key-not-a-secret\"}"

	testCases := []struct {
		name          string
		want          *credentials.APICreds
		path          string
		expectedError bool
	}{
		{"empty secret", &credentials.APICreds{}, "", true},
		{"missing host", &credentials.APICreds{Host: "", Key: "p"}, "", true},
		{"missing key", &credentials.APICreds{Host: "host", Key: ""}, "", true},
		{"valid creds", validCreds, "", false},
		{"valid creds custom path", validCreds, "fooo", false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			m := s.mockedManager
			mc, _ := m.client.(*mclient.SecretsManagerIface)
			ctx := context.Background()

			mc.On("CreateSecret", ctx, mock.Anything).Return(nil, nil)

			secretName, err := m.SaveAPICreds(ctx, orgID, tc.want)
			if tc.expectedError {
				assert.Error(err)
				return
			}

			assert.NoError(err)
			// Read the keypair
			got := &credentials.APICreds{}
			mc.On("GetSecretValue", ctx, &secretsmanager.GetSecretValueInput{
				SecretId: aws.String(secretName),
			}).Return(&secretsmanager.GetSecretValueOutput{SecretString: aws.String(validCredsString)}, nil)

			err = m.ReadAPICreds(ctx, secretName, got)
			assert.NoError(err)

			// Compare the keypair
			assert.Equal(tc.want, got)
		})
	}
}

type testSuite struct {
	suite.Suite
	mockedManager *Manager
}

// Run before each test
func (s *testSuite) SetupTest() {
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
