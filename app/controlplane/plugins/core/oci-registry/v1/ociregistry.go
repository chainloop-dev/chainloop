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
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
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
	// Repository is not fully URI compliant and hence can not be validated with jsonschema
	Repository string `json:"repository" jsonschema:"minLength=1,description=OCI repository uri and path"`
	Username   string `json:"username" jsonschema:"minLength=1,description=OCI repository username"`
	Password   string `json:"password" jsonschema:"minLength=1,description=OCI repository password"`
}

type attachmentRequest struct {
	Prefix string `json:"prefix,omitempty" jsonschema:"minLength=1,description=OCI images name prefix (default chainloop)"`
}

// 2 - Configuration state
type registrationState struct {
	Repository string `json:"repository"`
}

type attachmentState struct {
	Prefix string `json:"prefix"`
}

func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "oci-registry",
			Version:     "1.0",
			Description: "Send attestations to a compatible OCI registry",
			Logger:      l,
			InputSchema: &sdk.InputSchema{
				Registration: &registrationRequest{},
				Attachment:   &attachmentRequest{},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	return &Integration{base}, nil
}

// Register is executed when a operator wants to register a specific instance of this integration with their Chainloop organization
func (i *Integration) Register(_ context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	// Extract request payload
	var request *registrationRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Create and validate OCI credentials
	k, err := ociauth.NewCredentials(request.Repository, request.Username, request.Password)
	if err != nil {
		return nil, fmt.Errorf("the provided credentials are invalid")
	}

	// Check write permissions
	b, err := oci.NewBackend(request.Repository, &oci.RegistryOptions{Keychain: k})
	if err != nil {
		return nil, fmt.Errorf("the provided credentials are invalid")
	}

	if err := b.CheckWritePermissions(context.TODO()); err != nil {
		return nil, fmt.Errorf("the provided credentials don't have write permissions")
	}

	// They seem valid, let's store them in the configuration and credentials state
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
func (i *Integration) Attach(_ context.Context, req *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	// Extract request payload
	var request *attachmentRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Define the state to be stored
	config, err := sdk.ToConfig(&attachmentState{Prefix: request.Prefix})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	return &sdk.AttachmentResponse{Configuration: config}, nil
}

func (i *Integration) Execute(ctx context.Context, req *sdk.ExecutionRequest) error {
	i.Logger.Info("execution requested")

	if err := validateExecuteRequest(req); err != nil {
		return fmt.Errorf("running validation: %w", err)
	}

	// Extract registration configuration and credentials
	var registrationConfig *registrationState
	if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &registrationConfig); err != nil {
		return fmt.Errorf("invalid registration configuration %w", err)
	}

	// Extract attachment configuration
	var attachmentConfig *attachmentState
	if err := sdk.FromConfig(req.AttachmentInfo.Configuration, &attachmentConfig); err != nil {
		return fmt.Errorf("invalid attachment configuration %w", err)
	}

	// Create OCI backend client
	credentials := req.RegistrationInfo.Credentials
	k, err := ociauth.NewCredentials(registrationConfig.Repository, credentials.Username, credentials.Password)
	if err != nil {
		return fmt.Errorf("setting up the keychain: %w", err)
	}

	// Add prefix if provided
	var opts = make([]oci.NewBackendOpt, 0)
	if attachmentConfig.Prefix != "" {
		opts = append(opts, oci.WithPrefix(attachmentConfig.Prefix))
	}

	ociClient, err := oci.NewBackend(registrationConfig.Repository, &oci.RegistryOptions{Keychain: k}, opts...)
	if err != nil {
		return fmt.Errorf("creating OCI backend %w", err)
	}

	i.Logger.Infow("msg", "Uploading attestation", "repo", registrationConfig.Repository, "workflowID", req.WorkflowID)

	// Perform the upload of the json marshalled attestation
	jsonContent, err := json.Marshal(req.Input.Attestation.Envelope)
	if err != nil {
		return fmt.Errorf("marshaling the envelope: %w", err)
	}

	// Calculate digest since it will be used as CAS reference
	h, _, err := cr_v1.SHA256(bytes.NewBuffer(jsonContent))
	if err != nil {
		return fmt.Errorf("calculating the digest: %w", err)
	}

	if err := ociClient.Upload(ctx, bytes.NewBuffer(jsonContent), &v1.CASResource{Digest: h.Hex, FileName: "attestation.json"}); err != nil {
		return fmt.Errorf("uploading the attestation: %w", err)
	}

	i.Logger.Infow("msg", "Attestation uploaded", "repo", registrationConfig.Repository, "workflowID", req.WorkflowID)

	return nil
}

// Validate that we are receiving an envelope
// and the credentials and state from the registration stage
func validateExecuteRequest(req *sdk.ExecutionRequest) error {
	if req == nil || req.Input == nil {
		return errors.New("execution input not received")
	}

	if req.Input.Attestation == nil {
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
