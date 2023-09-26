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
	"errors"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
)

type Backend struct {
	client *azblob.Client
}

var _ backend.UploaderDownloader = (*Backend)(nil)
var errNotImplemented = errors.New("not implemented")

const defaultContainerName = "chainloop"

func NewBackend(creds *Credentials) (*Backend, error) {
	credential, err := azidentity.NewClientSecretCredential(creds.TenantID, creds.ClientID, creds.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Service principal Credential: %w", err)
	}

	url := fmt.Sprintf("https://%s.blob.core.windows.net/", creds.StorageAccountName)
	client, err := azblob.NewClient(url, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob Storage client: %w", err)
	}

	return &Backend{client: client}, nil
}

// Exists check that the artifact is already present in the repository and it points to the
// same image digest, meaning it has not been re-pushed/replaced
// This method is very naive so signatures will be used in future releases
func (b *Backend) Exists(_ context.Context, digest string) (bool, error) {
	return false, errNotImplemented
}

func (b *Backend) Upload(_ context.Context, r io.Reader, resource *pb.CASResource) error {
	return errNotImplemented
}

func (b *Backend) Describe(_ context.Context, digest string) (*pb.CASResource, error) {
	return nil, errNotImplemented
}

func (b *Backend) Download(_ context.Context, w io.Writer, digest string) error {
	return errNotImplemented
}

// CheckWritePermissions performs an actual write to the repository to check that the credentials
func (b *Backend) CheckWritePermissions(ctx context.Context) error {
	// Create container name
	_, err := b.client.CreateContainer(ctx, defaultContainerName, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return fmt.Errorf("failed to create Blob storage Container: %w", err)
	}

	// Touch a file
	_, err = b.client.UploadBuffer(ctx, defaultContainerName, "healthCheck", nil, nil)
	if err != nil {
		return fmt.Errorf("failed write to Blob Storage: %w", err)
	}

	return nil
}
