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

package azureblob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/chainloop-dev/chainloop/internal/credentials"
)

type BackendProvider struct {
	cReader credentials.Reader
}

var _ backend.Provider = (*BackendProvider)(nil)

func NewBackendProvider(cReader credentials.Reader) *BackendProvider {
	return &BackendProvider{cReader: cReader}
}

const ProviderID = "AzureBlob"

func (p *BackendProvider) ID() string {
	return ProviderID
}

func (p *BackendProvider) FromCredentials(ctx context.Context, secretName string) (backend.UploaderDownloader, error) {
	creds := &Credentials{}
	if err := p.cReader.ReadCredentials(ctx, secretName, creds); err != nil {
		return nil, err
	}

	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials retrieved from storage: %w", err)
	}

	return NewBackend(creds)
}

// location contains the storage account name + container name
func (p *BackendProvider) ValidateAndExtractCredentials(location string, credsJSON []byte) (any, error) {
	var creds *Credentials
	if err := json.Unmarshal(credsJSON, &creds); err != nil {
		return nil, fmt.Errorf("unmarshaling credentials: %w", err)
	}

	parts := strings.Split(location, "/")
	if len(parts) != 2 {
		return nil, errors.New("invalid location: must be in the format <account>/<container>")
	}

	// Override the location in the credentials since that's something we don't allow updating
	creds.StorageAccountName = parts[0]
	creds.Container = parts[1]

	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	b, err := NewBackend(creds)
	if err != nil {
		return nil, fmt.Errorf("creating backend: %w", err)
	}

	if err := b.CheckWritePermissions(context.TODO()); err != nil {
		return nil, fmt.Errorf("checking write permissions: %w", err)
	}

	return creds, nil
}

type Credentials struct {
	// Storage Account Name
	StorageAccountName string
	// Storage Account Container (optional)
	Container string
	// Active Directory Tenant ID
	TenantID string
	// Registered application / service principal client ID
	ClientID string
	// Registered application / service principal client secret
	ClientSecret string
}

var ErrValidation = errors.New("credentials validation error")

// Validate that the APICreds has all its properties set
func (c *Credentials) Validate() error {
	if c.StorageAccountName == "" {
		return fmt.Errorf("%w: missing storage account name", ErrValidation)
	}

	if c.TenantID == "" {
		return fmt.Errorf("%w: missing tenant ID", ErrValidation)
	}

	if c.ClientID == "" {
		return fmt.Errorf("%w: missing client ID", ErrValidation)
	}

	if c.ClientSecret == "" {
		return fmt.Errorf("%w: missing client secret", ErrValidation)
	}

	return nil
}
