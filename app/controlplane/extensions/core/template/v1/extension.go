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

package template

import (
	"context"
	"errors"
	"fmt"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/core/template/v1/api"
	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
)

type Integration struct {
	*sdk.FanOutIntegration
}

// You can use an arbitrary struct as a configuration state
// type registrationState struct{}
// type attachmentState struct{}

// Attach attaches the integration service to the given grpc server.
// In the future this will be a plugin entrypoint
func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanout(
		&sdk.NewParams{
			ID:      "template",
			Version: "1.0",
			Logger:  l,
		},
		// You can specify the inputs this attestation will be subscribed to, materials and or attestation envelope.
		// In this case we are subscribing to SBOM_CYCLONEDX_JSON
		sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON),
		// You can also subscribed to any kind of materials
		sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED),
		// Or to the actual attestation
		// sdk.WithEnvelope(),
	)

	if err != nil {
		return nil, err
	}

	return &Integration{base}, nil
}

// Register is executed when a operator wants to register a specific instance of this integration with their Chainloop organization
func (i *Integration) Register(ctx context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	// Parse the request
	request, ok := req.Payload.(*api.RegistrationRequest)
	if !ok {
		return nil, errors.New("invalid request")
	}

	// Validate it
	// NOTE: This validation is defined as proto buffers annotations
	if err := request.ValidateAll(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// START CUSTOM LOGIC
	// i.e validate the received information against the actual external server
	// EO CUSTOM LOGIC

	response := &sdk.RegistrationResponse{}

	// At this point you are ready to register the integration but you might want to store some information
	// so its available during the attachment and execution phases later on.
	// There are two kinds of information you can store, configuration and credentials.
	//
	// a) Configuration State
	// rawConfig, err := sdk.ToConfig(&registrationState{})
	// if err != nil {
	// 	return nil, fmt.Errorf("marshalling configuration: %w", err)
	// }
	// response.Configuration = rawConfig

	// b) Credentials state
	// In some cases you might have sensitive information that you want to store
	// like for example user credentials, API Keys and so on
	// in such cases, you can attach them to the response as well via the Credentials field
	// rawConfig, err := sdk.ToConfig(&registrationState{})
	// response.Credentials =  &sdk.Credentials{Password: "deadbeef"},

	return response, nil
}

// Attachment is executed when to attach a registered instance of this integration to a specific workflow
func (i *Integration) Attach(ctx context.Context, req *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	i.Logger.Info("attachment requested")

	// Parse the request
	request, ok := req.Payload.(*api.AttachmentRequest)
	if !ok {
		return nil, errors.New("invalid attachment configuration")
	}

	// Validate the request payload
	if err := request.ValidateAll(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// You also have access to the configuration and credentials from the registration phase
	// They can be accessed via
	// var rc *registrationState
	// if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &rc); err != nil {
	// 	return nil, errors.New("invalid registration configuration")
	// }
	// and the credentials via req.RegistrationInfo.Credentials

	// START CUSTOM LOGIC
	// i.e validate the received information against the actual external server
	// EO CUSTOM LOGIC

	response := &sdk.AttachmentResponse{}

	// Similarly to the registration phase, you might want to store some information
	// so it will be available during the execution phase later on.
	// rawConfig, err := sdk.ToConfig(&attachmentState{})
	// if err != nil {
	// 	return nil, fmt.Errorf("marshalling configuration: %w", err)
	// }
	// response.Configuration = rawConfig

	return response, nil
}

// Send the SBOM to the configured Dependency Track instance
func (i *Integration) Execute(ctx context.Context, req *sdk.ExecutionRequest) error {
	i.Logger.Info("execution requested")

	// Example of custom validation
	if err := validateExecuteRequest(req); err != nil {
		return fmt.Errorf("running validation: %w", err)
	}

	// Extract registration and attachment configuration if needed
	// var registrationConfig *registrationState
	// if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &registrationConfig); err != nil {
	// 	return errors.New("invalid registration configuration")
	// }

	// // Extract attachment configuration
	// var attachmentConfig *attachmentState
	// if err := sdk.FromConfig(req.AttachmentInfo.Configuration, &attachmentConfig); err != nil {
	// 	return errors.New("invalid attachment configuration")
	// }

	// START CUSTOM LOGIC
	// EO CUSTOM LOGIC

	return nil
}

// Validator example for the execution phase
// In this case we expect to receive a SBOM in CycloneDX format
// plus registration, attachment state and credentials
func validateExecuteRequest(req *sdk.ExecutionRequest) error {
	if req == nil || req.Input == nil || req.Input.Material == nil || req.Input.Material.Content == nil {
		return errors.New("invalid input")
	}

	if req.Input.Material.Type != schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String() {
		return fmt.Errorf("invalid input type: %s", req.Input.Material.Type)
	}

	if req.RegistrationInfo == nil || req.RegistrationInfo.Configuration == nil {
		return errors.New("missing registration configuration")
	}

	if req.RegistrationInfo.Credentials == nil {
		return errors.New("missing credentials")
	}

	if req.AttachmentInfo == nil || req.AttachmentInfo.Configuration == nil {
		return errors.New("missing attachment configuration")
	}

	return nil
}
