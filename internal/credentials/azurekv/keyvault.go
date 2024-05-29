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
	"fmt"
	"io"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/google/uuid"

	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
)

const (
	healthCheckSecret      = "chainloop-healthcheck"
	healthCheckNonExisting = "chainloop-non-existing"
)

type Manager struct {
	client       SecretsRW
	secretPrefix string
	logger       *log.Helper
}

type SecretsRW interface {
	SetSecret(ctx context.Context, secretName string, params azsecrets.SetSecretParameters, options *azsecrets.SetSecretOptions) (azsecrets.SetSecretResponse, error)
	GetSecret(ctx context.Context, secretName string, version string, options *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
	DeleteSecret(ctx context.Context, secretName string, options *azsecrets.DeleteSecretOptions) (azsecrets.DeleteSecretResponse, error)
}

type NewManagerOpts struct {
	// Active Directory Tenant ID
	TenantID string
	// Registered application / service principal client ID
	ClientID string
	// Registered application / service principal client secret
	ClientSecret string
	// Vault URL
	VaultURI string
	// Optional secret prefix
	SecretPrefix string
	Logger       log.Logger
	Role         credentials.Role
}

var ErrValidation = errors.New("credentials validation error")

func (o *NewManagerOpts) Validate() error {
	if o.TenantID == "" {
		return fmt.Errorf("%w: missing tenant ID", ErrValidation)
	}

	if o.ClientID == "" {
		return fmt.Errorf("%w: missing client ID", ErrValidation)
	}

	if o.ClientSecret == "" {
		return fmt.Errorf("%w: missing client secret", ErrValidation)
	}

	if o.VaultURI == "" {
		return fmt.Errorf("%w: missing VAULT URI", ErrValidation)
	}

	return nil
}

func NewManager(opts *NewManagerOpts) (*Manager, error) {
	l := opts.Logger
	if l == nil {
		l = log.NewStdLogger(io.Discard)
	}

	logger := servicelogger.ScopedHelper(l, "credentials/azure-key-vault")
	logger.Infow("msg", "configuring Azure KeyVault", "URI", opts.VaultURI, "role", opts.Role, "prefix", opts.SecretPrefix)

	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	credential, err := azidentity.NewClientSecretCredential(opts.TenantID, opts.ClientID, opts.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Service principal Credential: %w", err)
	}

	// Establish a connection to the Key Vault client
	client, err := azsecrets.NewClient(opts.VaultURI, credential, nil)
	if err != nil {
		log.Fatalf("failed to create a client: %v", err)
	}

	logger.Infow("msg", "Azure KeyVault configured", "URI", opts.VaultURI, "role", opts.Role, "prefix", opts.SecretPrefix)

	return &Manager{
		secretPrefix: opts.SecretPrefix,
		client:       client,
		logger:       logger,
	}, nil
}

// SaveCredentials saves credentials
func (m *Manager) SaveCredentials(ctx context.Context, orgID string, creds any) (string, error) {
	secretName := strings.Join([]string{m.secretPrefix, orgID, uuid.New().String()}, "-")
	// Store the credentials as json key pairs
	c, err := json.Marshal(creds)
	if err != nil {
		return "", fmt.Errorf("marshaling credentials to be stored: %w", err)
	}

	if _, err := m.client.SetSecret(ctx, secretName, azsecrets.SetSecretParameters{Value: strPtr(string(c))}, nil); err != nil {
		return "", fmt.Errorf("failed to set secret: %w", err)
	}

	return secretName, nil
}

// ReadCredentials reads the latest version of the credentials
func (m *Manager) ReadCredentials(ctx context.Context, secretName string, creds any) error {
	// retrieve latest version of the secret
	resp, err := m.client.GetSecret(ctx, secretName, "", nil)
	var respErr *azcore.ResponseError
	if err != nil {
		if errors.As(err, &respErr) && respErr.StatusCode == 404 {
			return fmt.Errorf("%w: path=%s", credentials.ErrNotFound, secretName)
		}

		return fmt.Errorf("failed to get secret: %w", err)
	}

	return json.Unmarshal([]byte(*resp.Value), creds)
}

// DeleteCredentials deletes credentials and versions
func (m *Manager) DeleteCredentials(ctx context.Context, secretName string) error {
	_, err := m.client.DeleteSecret(ctx, secretName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return nil
}

// ValidateWriterClient checks if the client is valid by writing and deleting a secret
// in the provided mount path.
func ValidateWriterClient(m *Manager, pathPrefix string) error {
	secretName := strings.Join([]string{pathPrefix, healthCheckSecret, uuid.NewString()}, "-")

	ctx := context.Background()
	if _, err := m.client.SetSecret(ctx, secretName, azsecrets.SetSecretParameters{Value: strPtr("")}, nil); err != nil {
		return fmt.Errorf("failed to set secret: %w", err)
	}

	if _, err := m.client.DeleteSecret(ctx, secretName, nil); err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return nil
}

// string to pointer
func strPtr(s string) *string {
	return &s
}

func ValidateReaderClient(m *Manager, pathPrefix string) error {
	// try to retrieve a non-existing key
	// if we get 404 means that we have permissions to read in that path
	secretName := strings.Join([]string{pathPrefix, healthCheckNonExisting, uuid.NewString()}, "-")
	_, err := m.client.GetSecret(context.Background(), secretName, "", nil)
	var respErr *azcore.ResponseError
	if err != nil {
		if errors.As(err, &respErr) && respErr.StatusCode == 404 {
			// Everything is ok
			return nil
		}

		return fmt.Errorf("failed to get secret: %w", err)
	}

	return errors.New("expected error")
}
