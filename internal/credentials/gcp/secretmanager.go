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
	"errors"
	"fmt"
	"io"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/option"

	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/docker/distribution/uuid"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/googleapis/gax-go/v2"
)

type SecretsManagerInterface interface {
	CreateSecret(ctx context.Context, req *secretmanagerpb.CreateSecretRequest, opts ...gax.CallOption) (*secretmanagerpb.Secret, error)
	AddSecretVersion(ctx context.Context, req *secretmanagerpb.AddSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.SecretVersion, error)
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	DeleteSecret(ctx context.Context, req *secretmanagerpb.DeleteSecretRequest, opts ...gax.CallOption) error
	GetSecret(ctx context.Context, req *secretmanagerpb.GetSecretRequest, opts ...gax.CallOption) (*secretmanagerpb.Secret, error)
}

type Manager struct {
	projectID    string
	secretPrefix string
	client       SecretsManagerInterface
	logger       *log.Helper
}

type NewManagerOpts struct {
	ProjectID, ServiceAccountKey, SecretPrefix string
	Logger                                     log.Logger
	Role                                       credentials.Role
}

func NewManager(opts *NewManagerOpts) (*Manager, error) {
	if opts.ProjectID == "" || opts.ServiceAccountKey == "" {
		return nil, errors.New("projectID and serviceAccountKey are required")
	}

	l := opts.Logger
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	logger := servicelogger.ScopedHelper(l, "credentials/gcp-secrets-manager")
	logger.Infow("msg", "configuring gcp secrets-manager", "projectID", opts.ProjectID, "role", opts.Role, "prefix", opts.SecretPrefix)

	cli, err := secretmanager.NewClient(context.TODO(), option.WithCredentialsFile(opts.ServiceAccountKey))
	if err != nil {
		return nil, fmt.Errorf("error while creating the client: %w", err)
	}

	logger.Infow("msg", "created GCP connection", "projectID", opts.ProjectID, "role", opts.Role, "prefix", opts.SecretPrefix)

	return &Manager{
		projectID:    opts.ProjectID,
		secretPrefix: opts.SecretPrefix,
		client:       cli,
		logger:       logger,
	}, nil
}

// SaveCredentials saves credentials
func (m *Manager) SaveCredentials(ctx context.Context, orgID string, creds any) (string, error) {
	// store creds in key-value pair
	c, err := json.Marshal(creds)
	if err != nil {
		return "", fmt.Errorf("marshaling credentials to be stored: %w", err)
	}

	secretID := strings.Join([]string{m.secretPrefix, orgID, uuid.Generate().String()}, "-")

	// first create the secret itself
	createSecretReq := &secretmanagerpb.CreateSecretRequest{
		Parent:   fmt.Sprintf("projects/%s", m.projectID),
		SecretId: secretID,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}
	secret, err := m.client.CreateSecret(ctx, createSecretReq)
	if err != nil {
		return "", fmt.Errorf("creating secret in GCP: %w", err)
	}
	m.logger.Infow("msg", "created new secret", "secretID", secretID)

	// once the secret is created store it as the newest version
	addSecretVersionReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: secret.Name,
		Payload: &secretmanagerpb.SecretPayload{
			Data: c,
		},
	}
	v, err := m.client.AddSecretVersion(ctx, addSecretVersionReq)
	if err != nil {
		return "", fmt.Errorf("creating secret version in GCP: %w", err)
	}
	m.logger.Infow("msg", "added new secret version", "secretID", secretID, "versionID", v.Name)

	return secretID, nil
}

// ReadCredentials reads the latest version of the credentials
func (m *Manager) ReadCredentials(ctx context.Context, secretID string, creds any) error {
	getSecretRequest := secretmanagerpb.AccessSecretVersionRequest{
		Name: fmt.Sprintf("projects/%v/secrets/%v/versions/latest", m.projectID, secretID),
	}
	result, err := m.client.AccessSecretVersion(ctx, &getSecretRequest)
	if err != nil {
		return fmt.Errorf("%w: path=%s", credentials.ErrNotFound, secretID)
	}
	m.logger.Infow("msg", "accessed secret", "secretID", secretID)

	return json.Unmarshal(result.Payload.Data, creds)
}

// DeleteCredentials deletes credentials and versions
func (m *Manager) DeleteCredentials(ctx context.Context, secretID string) error {
	deleteRequest := secretmanagerpb.DeleteSecretRequest{
		Name: fmt.Sprintf("projects/%v/secrets/%v", m.projectID, secretID),
	}
	m.logger.Infow("msg", "deleting secret", "secretID", secretID)

	return m.client.DeleteSecret(ctx, &deleteRequest)
}
