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

	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/ociauth"
)

type BackendProvider struct {
	cReader credentials.Reader
}

var _ backend.Provider = (*BackendProvider)(nil)

func NewBackendProvider(cReader credentials.Reader) *BackendProvider {
	return &BackendProvider{cReader: cReader}
}

func (p *BackendProvider) FromCredentials(ctx context.Context, secretName string) (backend.UploaderDownloader, error) {
	creds := &credentials.OCIKeypair{}
	if err := p.cReader.ReadOCICreds(ctx, secretName, creds); err != nil {
		return nil, err
	}

	k, err := ociauth.NewCredentials(creds.Repo, creds.Username, creds.Password)
	if err != nil {
		return nil, err
	}

	return NewBackend(creds.Repo, &RegistryOptions{Keychain: k})
}
