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
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const defaultOrgID = "test-org"
const defaultProjectID = "1234-5678-9012"
const defaultServiceAccountKey string = "./testdata/test_gcp_key.json"

func TestNewManager(t *testing.T) {
	assert := assert.New(t)

	testCases := []struct {
		name              string
		projectID         string
		serviceAccountKey string
		expectedError     bool
	}{
		{name: "missing projectID", projectID: "", serviceAccountKey: defaultServiceAccountKey, expectedError: true},
		{name: "missing service account key", projectID: defaultProjectID, serviceAccountKey: "", expectedError: true},
		{name: "wrong service account key path", projectID: defaultProjectID, serviceAccountKey: "./non/existing/path/key.json", expectedError: true},
		{name: "wrong type of service account key", projectID: defaultProjectID, serviceAccountKey: "./testdata/key.txt", expectedError: true},
		{name: "valid service account key", projectID: defaultProjectID, serviceAccountKey: defaultServiceAccountKey},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts := &NewManagerOpts{ProjectID: tc.projectID, ServiceAccountKey: tc.serviceAccountKey}
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

	clientMock.On("DeleteSecret", mock.Anything, mock.Anything).Return(nil)

	err := m.DeleteCredentials(context.Background(), secretID)
	assert.NoError(t, err)
}
