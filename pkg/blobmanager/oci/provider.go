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

package oci

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/chainloop-dev/chainloop/internal/ociauth"
	backend "github.com/chainloop-dev/chainloop/pkg/blobmanager"
	"github.com/chainloop-dev/chainloop/pkg/credentials"
)

type BackendProvider struct {
	cReader credentials.Reader
}

var _ backend.Provider = (*BackendProvider)(nil)

func NewBackendProvider(cReader credentials.Reader) *BackendProvider {
	return &BackendProvider{cReader: cReader}
}

const ProviderID = "OCI"

func (p *BackendProvider) ID() string {
	return ProviderID
}

func (p *BackendProvider) FromCredentials(ctx context.Context, secretName string) (backend.UploaderDownloader, error) {
	creds := &credentials.OCIKeypair{}
	if err := p.cReader.ReadCredentials(ctx, secretName, creds); err != nil {
		return nil, err
	}

	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials retrieved from storage: %w", err)
	}

	k, err := ociauth.NewCredentials(creds.Repo, creds.Username, creds.Password)
	if err != nil {
		return nil, err
	}

	return NewBackend(creds.Repo, &RegistryOptions{Keychain: k})
}

func (p *BackendProvider) ValidateAndExtractCredentials(location string, credsJSON []byte) (any, error) {
	var creds credentials.OCIKeypair
	if err := json.Unmarshal(credsJSON, &creds); err != nil {
		return nil, fmt.Errorf("unmarshaling credentials: %w", err)
	}

	// We are currently storing the repo location in the secret as well
	creds.Repo = location
	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	// Create and validate credentials
	k, err := ociauth.NewCredentials(location, creds.Username, creds.Password)
	if err != nil {
		return nil, fmt.Errorf("creating credentials: %w", err)
	}

	// Check credentials
	b, err := NewBackend(location, &RegistryOptions{Keychain: k})
	if err != nil {
		return nil, fmt.Errorf("checking credentials: %w", err)
	}

	if err := b.CheckWritePermissions(context.TODO()); err != nil {
		return nil, fmt.Errorf("checking write permissions: %w", err)
	}

	return creds, nil
}
