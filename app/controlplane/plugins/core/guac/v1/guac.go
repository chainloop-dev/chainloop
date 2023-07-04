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

package guac

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"cloud.google.com/go/storage"
	"code.cloudfoundry.org/bytefmt"
	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/api/option"
)

// Integration implements of a FanOut integration
// See https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/README.md for more information
type Integration struct {
	*sdk.FanOutIntegration
}

const providerGCS = "gcs"

// Options provided to the user when registering the integration
type registrationRequest struct {
	Provider    string `json:"provider,omitempty" jsonschema:"minLength=1,description=Blob storage provider: default gcs,enum=gcs"`
	Bucket      string `json:"bucket" jsonschema:"minLength=1,description=Bucket name where to store the artifacts"`
	Credentials string `json:"credentials" jsonschema:"minLength=2,description=Credentials to access the bucket"`
}

// No customization options are supported during attachment time
type attachmentRequest struct{}

// State stored to be retrieved later on during the execution of the actual dispatch
// NOTE: the credentials are not stored in this state but instead on a secure location, see registrationResponse below
type registrationState struct {
	Bucket   string `json:"bucket"`
	Provider string `json:"provider"`
}

func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "guac",
			Version:     "1.0",
			Description: "Export Attestation and SBOMs metadata to a blob storage backend so guacsec/guac can consume it",
			Logger:      l,
			InputSchema: &sdk.InputSchema{
				Registration: registrationRequest{},
				Attachment:   attachmentRequest{},
			},
		},
		// This plugin subscribes to SBOMs and attestations (note: attestations come by default)
		// In the future the kinds will expand to match
		// https://github.com/guacsec/guac#supported-input-formats
		sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON),
		sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON),
	)

	return &Integration{base}, err
}

// Register is executed when a operator wants to register a specific instance of this integration with their Chainloop organization
func (i *Integration) Register(ctx context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	// Marshal the request information
	var request *registrationRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Check that the credentials are valid and the bucket exists
	client, err := storage.NewClient(ctx, option.WithCredentialsJSON([]byte(request.Credentials)))
	if err != nil {
		return nil, fmt.Errorf("creating storage client: %w", err)
	}

	_, err = loadBucket(ctx, client, request.Bucket)
	if err != nil {
		return nil, err
	}

	// Store the bucket and provider in the state
	rawConfig, err := sdk.ToConfig(&registrationState{
		Bucket:   request.Bucket,
		Provider: string(providerGCS),
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	return &sdk.RegistrationResponse{
		// We also want to store the credentials securely
		Credentials:   &sdk.Credentials{Password: request.Credentials},
		Configuration: rawConfig,
	}, nil
}

// Attachment is executed when to attach a registered instance of this integration to a specific workflow
func (i *Integration) Attach(_ context.Context, _ *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	// Nothing to do during attachment
	return &sdk.AttachmentResponse{}, nil
}

// Execute will be instantiated when either an attestation or a material has been received
// It's up to the plugin builder to differentiate between inputs
func (i *Integration) Execute(ctx context.Context, req *sdk.ExecutionRequest) error {
	// Extract registration and attachment configuration if needed
	var registrationConfig *registrationState
	if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &registrationConfig); err != nil {
		return fmt.Errorf("invalid registration configuration %w", err)
	}

	if req.RegistrationInfo.Credentials == nil || req.RegistrationInfo.Credentials.Password == "" {
		return errors.New("missing expected credentials")
	}

	client, err := storage.NewClient(ctx, option.WithCredentialsJSON([]byte(req.RegistrationInfo.Credentials.Password)))
	if err != nil {
		return fmt.Errorf("creating storage client: %w", err)
	}

	bucket, err := loadBucket(ctx, client, registrationConfig.Bucket)
	if err != nil {
		return fmt.Errorf("loading bucket: %w", err)
	}

	// 1 - Upload the attestation
	envelopeJSON, err := json.Marshal(req.Input.Attestation.Envelope)
	if err != nil {
		return fmt.Errorf("marshalling attestation: %w", err)
	}

	filename := uniqueFilename(pathPrefix, "attestation.json", req.Input.Attestation.Hash.Hex)
	if err := uploadToBucket(ctx, bucket, filename, envelopeJSON, req.ChainloopMetadata, i.Logger); err != nil {
		return fmt.Errorf("uploading the SBOM to the bucket: %w", err)
	}

	// 2 - Upload all the materials, in our case they are SBOMs
	for _, sbom := range req.Input.Materials {
		filename := uniqueFilename(pathPrefix, sbom.Value, sbom.Hash.Hex)
		if err := uploadToBucket(ctx, bucket, filename, sbom.Content, req.ChainloopMetadata, i.Logger); err != nil {
			return fmt.Errorf("uploading the SBOM to the bucket: %w", err)
		}
	}

	return nil
}

// Append the digest and the extension i.e
// attestation-deadbeef.json
// sbom-cyclone-dx-123-deadbeef.xml
func uniqueFilename(path, filename string, digest string) string {
	// Find the file name at the end of the path without the extension
	name := filepath.Base(strings.TrimSuffix(filename, filepath.Ext(filename)))
	filename = fmt.Sprintf("%s-%s%s", name, digest, filepath.Ext(filename))

	if path != "" {
		filename = filepath.Join(path, filename)
	}

	return filename
}

// The path we use in the bucket to store the files i.e chainloop => chainloop/chainloop-deadbeef-sbom.json
const pathPrefix = "chainloop"

// uploadToBucket uploads the provided content to the bucket under a deterministic, unique name (digest + filename)
// It also sets the content type and additional metadata
func uploadToBucket(ctx context.Context, bucket *storage.BucketHandle, filename string, content []byte, md *sdk.ChainloopMetadata, logger *log.Helper) error {
	bucketInfo, err := bucket.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("getting bucket info: %w", err)
	}

	buf := bytes.NewBuffer(content)
	fileSize := uint64(buf.Len())
	logger.Infow("msg", "writing to the bucket", "file", filename, "bucket", bucketInfo.Name, "size", bytefmt.ByteSize(fileSize))

	w := bucket.Object(filename).NewWriter(ctx)
	defer w.Close()

	// Set metadata and content type
	// application/json can't be detected automatically by https://mimesniff.spec.whatwg.org/#supplied-mime-type-detection-algorithm
	// If not set, the underlying library will try to sniff it automatically
	if filepath.Ext(filename) == ".json" {
		w.ObjectAttrs.ContentType = "application/json"
	}

	w.ObjectAttrs.Metadata = map[string]string{
		"author":          "chainloop",
		"workflowID":      md.WorkflowID,
		"workflowName":    md.WorkflowName,
		"workflowProject": md.WorkflowProject,
		"workflowRunID":   md.WorkflowRunID,
		"filename":        filename,
	}

	if _, err := io.Copy(w, buf); err != nil {
		return fmt.Errorf("writing to the bucket: %w", err)
	}

	logger.Infow("msg", "uploaded to the bucket", "file", filename, "bucket", bucketInfo.Name, "size", bytefmt.ByteSize(fileSize))

	return nil
}

// loadBucket returns a bucket handle if:
// 1. The credentials are valid
// 2. The bucket exists
// 3. The credentials have write permissions
func loadBucket(ctx context.Context, client *storage.Client, bucket string) (*storage.BucketHandle, error) {
	if bucket == "" {
		return nil, errors.New("no bucket provided")
	}

	// Check that the bucket exists
	b := client.Bucket(bucket)
	_, err := b.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrBucketNotExist) {
			return nil, fmt.Errorf("the bucket %s does not exist", bucket)
		}

		return nil, fmt.Errorf("checking the bucket: %w", err)
	}

	// Write test file to make sure credentials have write permissions
	testFile := b.Object("chainloop-test-write")
	if err := testFile.NewWriter(ctx).Close(); err != nil {
		return nil, fmt.Errorf("we can't write to the bucket %s: %w", bucket, err)
	}
	_ = testFile.Delete(ctx)

	return b, nil
}
