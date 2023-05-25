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

package gcp

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	gcpmocks "github.com/chainloop-dev/chainloop/internal/credentials/gcp/mocks"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const defaultOrgID = "test-org"
const defaultProjectID = "1234-5678-9012"
const defaultAccessKey string = `{
  "type": "service_account",
  "project_id": "chainloop-test-dev",
  "private_key_id": "12345123451234512345",
  "private_key": "-----BEGIN PRIVATE KEY-----\nAAABBB\n-----END PRIVATE KEY-----\n",
  "client_email": "chainloop-dev@chainloop-dev.iam.gserviceaccount.com",
  "client_id": "5678567856781234",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/chainloop-dev%40chainloop-dev.iam.gserviceaccount.com",
  "universe_domain": "googleapis.com"
}`

func TestNewManager(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name          string
		projectID     string
		authKey       string
		expectedError bool
	}{
		{name: "missing projectID", projectID: "", authKey: defaultAccessKey, expectedError: true},
		{name: "missing authKey", projectID: defaultProjectID, authKey: "", expectedError: true},
		{name: "valid manager", projectID: defaultProjectID, authKey: defaultAccessKey},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := &NewManagerOpts{ProjectID: tc.projectID, AuthKey: tc.authKey}
			_, err := NewManager(opts)
			if tc.expectedError {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestCreateCredentials(t *testing.T) {
	ociCreds := &credentials.OCIKeypair{Repo: "repo", Username: "user", Password: "password"}
	creds, err := json.Marshal(ociCreds)
	assert.NoError(t, err)

	ctx := context.Background()
	m := &Manager{}
	m.logger = servicelogger.ScopedHelper(log.NewStdLogger(io.Discard), "credentials/gcp-secrets-manager")
	clientMock := gcpmocks.NewSecretsManagerInterface(t)
	m.client = clientMock

	clientMock.On("CreateSecret", ctx, mock.Anything).Return(&secretmanagerpb.Secret{}, nil).Once()
	clientMock.On("AddSecretVersion", ctx, mock.Anything).Return(&secretmanagerpb.SecretVersion{}, nil)

	_, err = m.SaveCredentials(ctx, defaultOrgID, creds)
	assert.NoError(t, err)
}

func TestReadCredentials(t *testing.T) {
	ctx := context.Background()
	m := &Manager{}
	m.logger = servicelogger.ScopedHelper(log.NewStdLogger(io.Discard), "credentials/gcp-secrets-manager")
	clientMock := gcpmocks.NewSecretsManagerInterface(t)
	m.client = clientMock
	secretID := "some-secret-id"

	validAPICreds := &credentials.APICreds{Host: "host", Key: "key"}
	payload, err := json.Marshal(validAPICreds)
	assert.NoError(t, err)

	clientMock.On("AccessSecretVersion", ctx, mock.Anything).Return(&secretmanagerpb.AccessSecretVersionResponse{
		Name: secretID,
		Payload: &secretmanagerpb.SecretPayload{
			Data: payload,
		},
	}, nil).Once()

	creds := &credentials.APICreds{}
	err = m.ReadCredentials(ctx, secretID, creds)
	assert.NoError(t, err)
	assert.Equal(t, "host", creds.Host)
	assert.Equal(t, "key", creds.Key)
}

func TestDeleteCredentials(t *testing.T) {
	m := &Manager{}
	m.logger = servicelogger.ScopedHelper(log.NewStdLogger(io.Discard), "credentials/gcp-secrets-manager")
	clientMock := gcpmocks.NewSecretsManagerInterface(t)
	m.client = clientMock
	m.projectID = defaultProjectID
	secretID := "some-secret-id"

	clientMock.On("GetSecret", mock.Anything, mock.Anything).Return(&secretmanagerpb.Secret{}, nil).Once()
	clientMock.On("DeleteSecret", mock.Anything, mock.Anything).Return(nil)

	err := m.DeleteCredentials(context.Background(), secretID)
	assert.NoError(t, err)
}
