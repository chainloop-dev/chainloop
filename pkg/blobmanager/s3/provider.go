//
// Copyright 2024 The Chainloop Authors.
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

package s3

import (
	"context"
	"encoding/json"
	"fmt"

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

const ProviderID = "AWS-S3"

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

func (p *BackendProvider) ValidateAndExtractCredentials(location string, credsJSON []byte) (any, error) {
	creds, err := extractCreds(location, credsJSON)
	if err != nil {
		return nil, fmt.Errorf("extracting credentials: %w", err)
	}

	// Validate that the credentials are valid against the storage account
	b, err := NewBackend(creds)
	if err != nil {
		return nil, fmt.Errorf("creating backend: %w", err)
	}

	if err := b.CheckWritePermissions(context.TODO()); err != nil {
		return nil, fmt.Errorf("checking write permissions: %w", err)
	}

	return creds, nil
}

func extractCreds(location string, credsJSON []byte) (*Credentials, error) {
	var creds *Credentials
	if err := json.Unmarshal(credsJSON, &creds); err != nil {
		return nil, fmt.Errorf("unmarshaling credentials: %w", err)
	}

	// We do not allow overriding the location
	creds.Location = location

	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	return creds, nil
}

type Credentials struct {
	// AWS Access Key ID
	AccessKeyID string
	// AWS Secret Access Key
	SecretAccessKey string
	// Deprecated: use Location instead. Kept for backward compatibility with existing stored credentials
	BucketName string
	// Location is a combination of the bucket name and the endpoint
	// custom endpoint + bucket-name https://123.r2.cloudflarestorage.com/bucket-name
	// or just the bucket name i.e bucket-name
	Location string
	// Region ID, i.e us-east-1
	Region string
}

// Validate that the APICreds has all its properties set
func (c *Credentials) Validate() error {
	if c.AccessKeyID == "" {
		return fmt.Errorf("%w: missing accessKeyID", backend.ErrValidation)
	}

	if c.SecretAccessKey == "" {
		return fmt.Errorf("%w: missing secretAccessKey", backend.ErrValidation)
	}

	// BucketName is deprecated, we should use Location instead
	if c.Location == "" && c.BucketName == "" {
		return fmt.Errorf("%w: missing bucket and location", backend.ErrValidation)
	}

	return nil
}
