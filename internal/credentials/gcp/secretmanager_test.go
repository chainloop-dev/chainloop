package gcp

import (
	"context"
	"encoding/json"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	gcpmocks "github.com/chainloop-dev/chainloop/internal/credentials/gcp/mocks"
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
	clientMock := gcpmocks.NewSecretsManagerInterface(t)
	m.client = clientMock
	secretId := "some-secret-id"

	validAPICreds := &credentials.APICreds{Host: "host", Key: "key"}
	payload, err := json.Marshal(validAPICreds)
	assert.NoError(t, err)

	clientMock.On("AccessSecretVersion", ctx, mock.Anything).Return(&secretmanagerpb.AccessSecretVersionResponse{
		Name: secretId,
		Payload: &secretmanagerpb.SecretPayload{
			Data: payload,
		},
	}, nil).Once()

	creds := &credentials.APICreds{}
	err = m.ReadCredentials(ctx, secretId, creds)
	assert.NoError(t, err)
	assert.Equal(t, "host", creds.Host)
	assert.Equal(t, "key", creds.Key)
}

func TestDeleteCredentials(t *testing.T) {
	m := &Manager{}
	clientMock := gcpmocks.NewSecretsManagerInterface(t)
	m.client = clientMock
	m.projectID = defaultProjectID
	secretId := "some-secret-id"

	clientMock.On("GetSecret", mock.Anything, mock.Anything).Return(&secretmanagerpb.Secret{}, nil).Once()
	clientMock.On("DeleteSecret", mock.Anything, mock.Anything).Return(nil)

	err := m.DeleteCredentials(context.Background(), secretId)
	assert.NoError(t, err)
}
