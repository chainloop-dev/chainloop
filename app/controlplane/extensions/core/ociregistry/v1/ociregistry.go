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

package ociregistry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	v1 "github.com/chainloop-dev/chainloop/app/artifact-cas/api/cas/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/blobmanager/oci"
	"github.com/chainloop-dev/chainloop/internal/ociauth"
	"github.com/go-kratos/kratos/v2/log"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
)

type Integration struct {
	*sdk.FanOutIntegration
}

// 1 - API schema definitions
type registrationRequest struct {
	// NOTE: it can not be an URI since http schemas are not expected
	Repository string `json:"repository" jsonschema:"format=uri-reference,description=OCI repository uri and path"`
	Username   string `json:"username" jsonschema:"minLength=1,description=OCI repository username"`
	Password   string `json:"password" jsonschema:"minLength=1,description=OCI repository password"`
}

type attachmentRequest struct{}

// 2 - Configuration state
type registrationState struct {
	Repository string `json:"repository"`
}

// Attach attaches the integration service to the given grpc server.
// In the future this will be a plugin entrypoint
func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "oci-registry",
			Version:     "0.1",
			Description: "Send attestations to a compatible OCI registry",
			Logger:      l,
			InputSchema: &sdk.InputSchema{
				Registration: registrationRequest{},
				Attachment:   attachmentRequest{},
			},
		},
		sdk.WithEnvelope(),
	)

	if err != nil {
		return nil, err
	}

	return &Integration{base}, nil
}

// Register is executed when a operator wants to register a specific instance of this integration with their Chainloop organization
func (i *Integration) Register(_ context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	var request *registrationRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Create and validate credentials
	k, err := ociauth.NewCredentials(request.Repository, request.Username, request.Password)
	if err != nil {
		return nil, fmt.Errorf("the provided credentials are invalid")
	}

	// Check credentials
	b, err := oci.NewBackend(request.Repository, &oci.RegistryOptions{Keychain: k})
	if err != nil {
		return nil, fmt.Errorf("the provided credentials are invalid")
	}

	if err := b.CheckWritePermissions(context.TODO()); err != nil {
		return nil, fmt.Errorf("the provided credentials don't have write permissions")
	}

	// they seem valid, let's store them
	response := &sdk.RegistrationResponse{}

	// a) Configuration State
	rawConfig, err := sdk.ToConfig(&registrationState{
		Repository: request.Repository,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	response.Configuration = rawConfig

	// b) Credentials state
	response.Credentials = &sdk.Credentials{Password: request.Password, Username: request.Username}

	return response, nil
}

// Attachment is executed when to attach a registered instance of this integration to a specific workflow
func (i *Integration) Attach(_ context.Context, _ *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	i.Logger.Info("attachment requested")

	// NOOP
	return &sdk.AttachmentResponse{}, nil
}

func (i *Integration) Execute(ctx context.Context, req *sdk.ExecutionRequest) error {
	i.Logger.Info("execution requested")

	if err := validateExecuteRequest(req); err != nil {
		return fmt.Errorf("running validation: %w", err)
	}

	// Extract registration configuration and credentials
	var config *registrationState
	if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &config); err != nil {
		return fmt.Errorf("invalid registration configuration %w", err)
	}

	credentials := req.RegistrationInfo.Credentials

	// Create OCI backend client
	k, err := ociauth.NewCredentials(config.Repository, credentials.Username, credentials.Password)
	if err != nil {
		return fmt.Errorf("setting up the keychain: %w", err)
	}

	ociClient, err := oci.NewBackend(config.Repository, &oci.RegistryOptions{Keychain: k})
	if err != nil {
		return fmt.Errorf("creating OCI backend %w", err)
	}

	i.Logger.Infow("msg", "Uploading attestation", "repo", config.Repository, "workflowID", req.WorkflowID)
	// Perform the upload
	jsonContent, err := json.Marshal(req.Input.DSSEnvelope)
	if err != nil {
		return fmt.Errorf("marshaling the envelope: %w", err)
	}

	// Calculate digest
	h, _, err := cr_v1.SHA256(bytes.NewBuffer(jsonContent))
	if err != nil {
		return fmt.Errorf("calculating the digest: %w", err)
	}

	if err := ociClient.Upload(ctx, bytes.NewBuffer(jsonContent), &v1.CASResource{Digest: h.Hex, FileName: "attestation.json"}); err != nil {
		return fmt.Errorf("uploading the attestation: %w", err)
	}

	i.Logger.Infow("msg", "Attestation uploaded", "repo", config.Repository, "workflowID", req.WorkflowID)

	return nil
}

// Validate that we are receiving an envelope
// and the credentials and state from the registration stage
func validateExecuteRequest(req *sdk.ExecutionRequest) error {
	if req == nil || req.Input == nil {
		return errors.New("execution input not received")
	}

	if req.Input.DSSEnvelope == nil {
		return errors.New("execution input invalid, the envelope is empty")
	}

	if req.RegistrationInfo == nil || req.RegistrationInfo.Configuration == nil {
		return errors.New("missing registration configuration")
	}

	if req.RegistrationInfo.Credentials == nil {
		return errors.New("missing credentials")
	}

	return nil
}
