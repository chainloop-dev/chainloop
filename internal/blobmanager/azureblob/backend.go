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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	pb "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	backend "github.com/chainloop-dev/chainloop/internal/blobmanager"
)

type Backend struct {
	storageAccountName string
	credentials        *azidentity.ClientSecretCredential
}

var _ backend.UploaderDownloader = (*Backend)(nil)
var errNotImplemented = errors.New("not implemented")

const defaultContainerName = "chainloop"

func NewBackend(creds *Credentials) (*Backend, error) {
	credential, err := azidentity.NewClientSecretCredential(creds.TenantID, creds.ClientID, creds.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Service principal Credential: %w", err)
	}

	return &Backend{
		storageAccountName: creds.StorageAccountName,
		credentials:        credential,
	}, nil
}

// top level client used for creation/upload/download/listing operations
func (b *Backend) client() (*azblob.Client, error) {
	url := fmt.Sprintf("https://%s.blob.core.windows.net/", b.storageAccountName)
	// Top level client
	client, err := azblob.NewClient(url, b.credentials, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob Storage client: %w", err)
	}

	return client, nil
}

// blob client used for operating with a single blob
func (b *Backend) blobClient(digest string) (*blob.Client, error) {
	blobClient, err := blob.NewClient(b.resourcePath(digest), b.credentials, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob Storage client: %w", err)
	}

	return blobClient, nil
}

func (b *Backend) resourcePath(digest string) string {
	return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", b.storageAccountName, defaultContainerName, resourceName(digest))
}

func resourceName(digest string) string {
	return fmt.Sprintf("sha256:%s", digest)
}

// Exists check that the artifact is already present in the repository
// TODO: make some check on its size / digest
func (b *Backend) Exists(ctx context.Context, digest string) (bool, error) {
	blobClient, err := b.blobClient(digest)
	if err != nil {
		return false, fmt.Errorf("failed to create Azure Blob Storage client: %w", err)
	}

	if _, err = blobClient.GetProperties(ctx, nil); err != nil {
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get blob properties: %w", err)
	}

	return true, nil
}

const (
	annotationNameAuthor = "author"
	annotationNameTitle  = "title"
)

func (b *Backend) Upload(ctx context.Context, r io.Reader, resource *pb.CASResource) error {
	client, err := b.client()
	if err != nil {
		return err
	}

	// Create container name
	if err := b.createContainer(ctx, client); err != nil {
		return fmt.Errorf("failed to create Blob storage Container: %w", err)
	}

	_, err = client.UploadStream(ctx, defaultContainerName, resourceName(resource.Digest), r, &azblob.UploadStreamOptions{
		Metadata: map[string]*string{
			annotationNameAuthor: to.Ptr(backend.AuthorAnnotation),
			annotationNameTitle:  to.Ptr(resource.FileName),
		},
	})

	return err
}

func (b *Backend) Describe(_ context.Context, digest string) (*pb.CASResource, error) {
	return nil, errNotImplemented
}

func (b *Backend) Download(_ context.Context, w io.Writer, digest string) error {
	return errNotImplemented
}

func (b *Backend) createContainer(ctx context.Context, client *azblob.Client) error {
	// Create container name
	_, err := client.CreateContainer(ctx, defaultContainerName, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return fmt.Errorf("failed to create Blob storage Container: %w", err)
	}

	return nil
}

// CheckWritePermissions performs an actual write to the repository to check that the credentials
func (b *Backend) CheckWritePermissions(ctx context.Context) error {
	client, err := b.client()
	if err != nil {
		return fmt.Errorf("failed to create Azure Blob Storage client: %w", err)
	}

	// Create container name
	if err := b.createContainer(ctx, client); err != nil {
		return fmt.Errorf("failed to create Blob storage Container: %w", err)
	}

	// Touch a file
	_, err = client.UploadBuffer(ctx, defaultContainerName, "healthCheck", nil, nil)
	if err != nil {
		return fmt.Errorf("failed write to Blob Storage: %w", err)
	}

	return nil
}
