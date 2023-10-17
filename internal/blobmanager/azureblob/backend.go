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
	container          string
	credentials        *azidentity.ClientSecretCredential
}

var _ backend.UploaderDownloader = (*Backend)(nil)

func NewBackend(creds *Credentials) (*Backend, error) {
	credential, err := azidentity.NewClientSecretCredential(creds.TenantID, creds.ClientID, creds.ClientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Service principal Credential: %w", err)
	}

	return &Backend{
		storageAccountName: creds.StorageAccountName,
		credentials:        credential,
		container:          creds.Container,
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
	return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", b.storageAccountName, b.container, resourceName(digest))
}

func resourceName(digest string) string {
	return fmt.Sprintf("sha256:%s", digest)
}

// Exists check that the artifact is already present in the repository
func (b *Backend) Exists(ctx context.Context, digest string) (bool, error) {
	_, err := b.Describe(ctx, digest)
	if err != nil && backend.IsNotFound(err) {
		return false, nil
	}

	return err == nil, err
}

const (
	annotationNameAuthor   = "Author"
	annotationNameFilename = "Filename"
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

	_, err = client.UploadStream(ctx, b.container, resourceName(resource.Digest), r, &azblob.UploadStreamOptions{
		Metadata: map[string]*string{
			annotationNameAuthor:   to.Ptr(backend.AuthorAnnotation),
			annotationNameFilename: to.Ptr(resource.FileName),
		},
	})

	return err
}

func (b *Backend) Describe(ctx context.Context, digest string) (*pb.CASResource, error) {
	blobClient, err := b.blobClient(digest)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob Storage client: %w", err)
	}

	properties, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		if bloberror.HasCode(err, bloberror.BlobNotFound) {
			return nil, backend.NewErrNotFound("artifact")
		}

		return nil, fmt.Errorf("failed to get blob properties: %w", err)
	}

	// Check asset author is chainloop that way we can ignore files uploaded by other tools
	// note: this is not a security mechanism, an additional check will be put in place for tamper check
	author, ok := properties.Metadata[annotationNameAuthor]
	if !ok || *author != backend.AuthorAnnotation {
		return nil, errors.New("asset not uploaded by Chainloop")
	}

	fileName, ok := properties.Metadata[annotationNameFilename]
	if !ok {
		return nil, fmt.Errorf("couldn't find file metadata")
	}

	return &pb.CASResource{
		FileName: *fileName,
		Size:     *properties.ContentLength,
	}, nil
}

func (b *Backend) Download(ctx context.Context, w io.Writer, digest string) error {
	exists, err := b.Exists(ctx, digest)
	if err != nil {
		return fmt.Errorf("invalid artifact: %w", err)
	} else if !exists {
		return backend.NewErrNotFound("artifact")
	}

	client, err := b.client()
	if err != nil {
		return fmt.Errorf("failed to create Azure Blob Storage client: %w", err)
	}

	// Download the blob
	downloadStream, err := client.DownloadStream(ctx, b.container, resourceName(digest), nil)
	if err != nil {
		return fmt.Errorf("failed to download blob: %w", err)
	}

	retryReader := downloadStream.NewRetryReader(ctx, &azblob.RetryReaderOptions{})
	defer retryReader.Close()

	if _, err := io.Copy(w, retryReader); err != nil {
		return fmt.Errorf("failed to copy blob to writer: %w", err)
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
	_, err = client.UploadBuffer(ctx, b.container, "healthCheck", nil, nil)
	if err != nil {
		return fmt.Errorf("failed write to Blob Storage: %w", err)
	}

	return nil
}

func (b *Backend) createContainer(ctx context.Context, client *azblob.Client) error {
	// Create container name
	_, err := client.CreateContainer(ctx, b.container, nil)
	if err != nil && !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
		return fmt.Errorf("failed to create Blob storage Container: %w", err)
	}

	return nil
}
