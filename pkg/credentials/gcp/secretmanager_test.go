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

package gcp

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
	gcpmocks "github.com/chainloop-dev/chainloop/pkg/credentials/gcp/mocks"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func TestSaveCredentialsUpsert(t *testing.T) {
	existingSecretName := "my-existing-secret"

	testCases := []struct {
		name        string
		secretName  string // non-empty = WithSecretName upsert
		gcpNotFound bool   // simulate secret container absent in GCP
	}{
		{
			name:       "new secret — CreateSecret then AddSecretVersion",
			secretName: "",
		},
		{
			name:       "upsert existing — CreateSecret AlreadyExists, AddSecretVersion only",
			secretName: existingSecretName,
		},
		{
			name:        "upsert not found — CreateSecret then AddSecretVersion",
			secretName:  existingSecretName,
			gcpNotFound: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			m := &Manager{projectID: defaultProjectID}
			m.logger = servicelogger.ScopedHelper(log.NewStdLogger(io.Discard), "credentials/gcp-secrets-manager")
			clientMock := gcpmocks.NewSecretsManagerInterface(t)
			m.client = clientMock

			ociCreds := &credentials.OCIKeypair{Repo: "repo", Username: "user", Password: "password"}

			var saveOpts []credentials.SaveOption
			if tc.secretName != "" {
				saveOpts = append(saveOpts, credentials.WithSecretName(tc.secretName))
			}

			switch {
			case tc.secretName == "":
				// New path: CreateSecret returns a secret, then AddSecretVersion.
				clientMock.On("CreateSecret", ctx, mock.Anything).
					Return(&secretmanagerpb.Secret{Name: "projects/1234-5678-9012/secrets/generated-id"}, nil).Once()
				clientMock.On("AddSecretVersion", ctx, mock.Anything).
					Return(&secretmanagerpb.SecretVersion{Name: "…/versions/1"}, nil)
			case tc.gcpNotFound:
				// Upsert: secret container absent → CreateSecret succeeds → AddSecretVersion.
				clientMock.On("CreateSecret", ctx, mock.Anything).Return(&secretmanagerpb.Secret{}, nil).Once()
				clientMock.On("AddSecretVersion", ctx, mock.Anything).
					Return(&secretmanagerpb.SecretVersion{Name: "…/versions/1"}, nil)
			default:
				// Upsert: secret container exists → CreateSecret returns AlreadyExists → AddSecretVersion only.
				alreadyExistsErr := status.Error(codes.AlreadyExists, "secret already exists")
				clientMock.On("CreateSecret", ctx, mock.Anything).Return(nil, alreadyExistsErr).Once()
				clientMock.On("AddSecretVersion", ctx, mock.Anything).
					Return(&secretmanagerpb.SecretVersion{Name: "…/versions/2"}, nil)
			}

			returned, err := m.SaveCredentials(ctx, defaultOrgID, ociCreds, saveOpts...)
			assert.NoError(t, err)

			if tc.secretName != "" {
				assert.Equal(t, tc.secretName, returned, "upsert must return the same secret name")
			} else {
				assert.NotEmpty(t, returned)
			}
		})
	}
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
