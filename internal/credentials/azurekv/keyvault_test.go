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

package azurekv

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/credentials/azurekv/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

func (s *testSuite) TestNewManager() {
	testCases := []struct {
		name          string
		tenantID      string
		clientID      string
		clientSecret  string
		vaultURI      string
		Role          credentials.Role
		expectedError bool
	}{
		{name: "missing tenantID", tenantID: "", clientID: "clientID", clientSecret: "clientSecret", vaultURI: "vaultURI", Role: credentials.RoleReader, expectedError: true},
		{name: "missing clientID", tenantID: "tenantID", clientID: "", clientSecret: "clientSecret", vaultURI: "vaultURI", Role: credentials.RoleReader, expectedError: true},
		{name: "missing clientSecret", tenantID: "tenantID", clientID: "clientID", clientSecret: "", vaultURI: "vaultURI", Role: credentials.RoleReader, expectedError: true},
		{name: "missing vaultURI", tenantID: "tenantID", clientID: "clientID", clientSecret: "clientSecret", vaultURI: "", Role: credentials.RoleReader, expectedError: true},
		{name: "valid reader configuration", tenantID: "tenantID", clientID: "clientID", clientSecret: "clientSecret", vaultURI: "vaultURI", Role: credentials.RoleReader},
		{name: "valid writer configuration", tenantID: "tenantID", clientID: "clientID", clientSecret: "clientSecret", vaultURI: "vaultURI", Role: credentials.RoleWriter},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			opts := &NewManagerOpts{TenantID: tc.tenantID, ClientID: tc.clientID, ClientSecret: tc.clientSecret, VaultURI: tc.vaultURI, Role: tc.Role}
			_, err := NewManager(opts)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func (s *testSuite) TestValidateWriterClient() {
	s.Run("happy path", func() {
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("SetSecret", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(azsecrets.SetSecretResponse{}, nil)
		secretsRW.On("DeleteSecret", mock.Anything, mock.Anything, mock.Anything).Return(azsecrets.DeleteSecretResponse{}, nil)
		s.NoError(ValidateWriterClient(&Manager{client: secretsRW}, "prefix"))
	})

	s.Run("can't write", func() {
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("SetSecret", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(azsecrets.SetSecretResponse{}, errors.New("can't write"))
		s.Error(ValidateWriterClient(&Manager{client: secretsRW}, "prefix"))
	})

	s.Run("can't delete", func() {
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("SetSecret", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(azsecrets.SetSecretResponse{}, nil)
		secretsRW.On("DeleteSecret", mock.Anything, mock.Anything, mock.Anything).Return(azsecrets.DeleteSecretResponse{}, errors.New("can't delete"))
		s.Error(ValidateWriterClient(&Manager{client: secretsRW}, "prefix"))
	})
}

func (s *testSuite) TestValidateReaderClient() {
	s.Run("the secret is found means error", func() {
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("GetSecret", mock.Anything, mock.Anything, "", mock.Anything).Return(azsecrets.GetSecretResponse{}, nil)
		s.Error(ValidateReaderClient(&Manager{client: secretsRW}, "prefix"))
	})

	s.Run("secret not found but can read", func() {
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("GetSecret", mock.Anything, mock.Anything, "", mock.Anything).
			Return(azsecrets.GetSecretResponse{}, &azcore.ResponseError{StatusCode: 404})
		s.NoError(ValidateReaderClient(&Manager{client: secretsRW}, "prefix"))
	})

	s.Run("can't read", func() {
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("GetSecret", mock.Anything, mock.Anything, "", mock.Anything).
			Return(azsecrets.GetSecretResponse{}, &azcore.ResponseError{StatusCode: 401})
		s.Error(ValidateReaderClient(&Manager{client: secretsRW}, "prefix"))
	})
}

func (s *testSuite) TestDeleteCredentials() {
	s.Run("happy path", func() {
		ctx := context.Background()
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("DeleteSecret", ctx, "my-secret", mock.Anything).Return(azsecrets.DeleteSecretResponse{}, nil)
		s.NoError((&Manager{client: secretsRW}).DeleteCredentials(ctx, "my-secret"))
	})

	s.Run("can't delete", func() {
		ctx := context.Background()
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("DeleteSecret", ctx, "my-secret", mock.Anything).Return(azsecrets.DeleteSecretResponse{}, errors.New("can't delete"))
		s.Error((&Manager{client: secretsRW}).DeleteCredentials(ctx, "my-secret"))
	})
}

func (s *testSuite) TestSaveCredentials() {
	creds := &credentials.APICreds{
		Host: "myhost",
		Key:  "mykey",
	}

	toStoreCreds, err := json.Marshal(creds)
	s.NoError(err)

	s.Run("happy path", func() {
		ctx := context.Background()
		secretsRW := mocks.NewSecretsRW(s.T())
		var want string
		secretsRW.On("SetSecret", ctx, mock.Anything, azsecrets.SetSecretParameters{Value: strPtr(string(toStoreCreds))}, mock.Anything).
			Return(azsecrets.SetSecretResponse{}, nil).Run(func(args mock.Arguments) {
			want = args.Get(1).(string)
		})

		m := &Manager{client: secretsRW, secretPrefix: "my-prefix"}
		got, err := m.SaveCredentials(ctx, "my-org", creds)
		s.NoError(err)
		s.Equal(want, got)
	})

	s.Run("can't save", func() {
		ctx := context.Background()
		secretsRW := mocks.NewSecretsRW(s.T())
		upstreamErr := errors.New("upstream error")
		secretsRW.On("SetSecret", ctx, mock.Anything, azsecrets.SetSecretParameters{Value: strPtr(string(toStoreCreds))}, mock.Anything).
			Return(azsecrets.SetSecretResponse{}, upstreamErr)

		m := &Manager{client: secretsRW, secretPrefix: "my-prefix"}
		got, err := m.SaveCredentials(ctx, "my-org", creds)
		s.ErrorIs(err, upstreamErr)
		s.Empty(got)
	})
}

func (s *testSuite) TestReadCredentials() {
	want := &credentials.APICreds{
		Host: "myhost",
		Key:  "mykey",
	}

	wantRaw, err := json.Marshal(want)
	s.NoError(err)

	s.Run("happy path", func() {
		ctx := context.Background()
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("GetSecret", mock.Anything, mock.Anything, "", mock.Anything).
			Return(azsecrets.GetSecretResponse{
				SecretBundle: azsecrets.SecretBundle{
					Value: strPtr(string(wantRaw)),
				},
			}, nil)

		m := &Manager{client: secretsRW}
		got := &credentials.APICreds{}
		s.NoError(m.ReadCredentials(ctx, "my-secret", got))
		s.Equal(want, got)
	})

	s.Run("not found", func() {
		ctx := context.Background()
		secretsRW := mocks.NewSecretsRW(s.T())
		secretsRW.On("GetSecret", mock.Anything, mock.Anything, "", mock.Anything).
			Return(azsecrets.GetSecretResponse{}, &azcore.ResponseError{StatusCode: 404})

		m := &Manager{client: secretsRW}
		got := &credentials.APICreds{}
		err := m.ReadCredentials(ctx, "my-secret", got)
		s.Error(err)
		s.ErrorIs(err, credentials.ErrNotFound)
	})

	s.Run("other error", func() {
		ctx := context.Background()
		secretsRW := mocks.NewSecretsRW(s.T())
		upstreamErr := errors.New("upstream error")
		secretsRW.On("GetSecret", mock.Anything, mock.Anything, "", mock.Anything).
			Return(azsecrets.GetSecretResponse{}, upstreamErr)

		m := &Manager{client: secretsRW}
		got := &credentials.APICreds{}
		err := m.ReadCredentials(ctx, "my-secret", got)
		s.Error(err)
		s.ErrorIs(err, upstreamErr)
	})
}

type testSuite struct {
	suite.Suite
	secretsRW *mocks.SecretsRW
	m         *Manager
}

func (s *testSuite) SetupTest() {
	s.secretsRW = mocks.NewSecretsRW(s.T())
	s.m = &Manager{client: s.secretsRW}
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(testSuite))
}
