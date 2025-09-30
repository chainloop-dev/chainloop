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
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
)

// Integration implements of a FanOut integration
// See https://github.com/chainloop-dev/chainloop/blob/main/app/controlplane/plugins/README.md for more information
type Integration struct {
	*sdk.FanOutIntegration
}

// 1 - API schema definitions
// Define the input schemas for both registration and attachment
// You can annotate the struct with jsonschema tags to define the enable validations
// see https://github.com/invopop/jsonschema#example for more information
type registrationRequest struct {
	// TestURL is a required, valid URL
	TestURL string `json:"testURL" jsonschema:"format=uri,description=Example of URL-type input"`
}

type attachmentRequest struct {
	OptionalBool bool `json:"optionalBool,omitempty" jsonschema:"description=Example of optional boolean input"`
}

// 2 - Configuration state
// You can use an arbitrary struct as a configuration state, this means data that you want to persist across
// type registrationState struct{}
// type attachmentState struct{}

func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "template",
			Version:     "0.1",
			Description: "Template integration that can be used as a starting point for your own integrations",
			Logger:      l,
			InputSchema: &sdk.InputSchema{
				Registration: registrationRequest{},
				Attachment:   attachmentRequest{},
			},
		},
		// You can specify the inputs this attestation will be subscribed to, materials and or attestation envelope.
		// In this case we are subscribing to SBOM_CYCLONEDX_JSON
		sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON),
	)

	if err != nil {
		return nil, err
	}

	return &Integration{base}, nil
}

// Register is executed when a operator wants to register a specific instance of this integration with their Chainloop organization
func (i *Integration) Register(_ context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	// Unmarshal the request
	// NOTE: the request payload has been already validated against the input schema
	var request *registrationRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
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
	// response.Credentials =  &sdk.Credentials{Password: "deadbeef"}

	return response, nil
}

// Attachment is executed when to attach a registered instance of this integration to a specific workflow
func (i *Integration) Attach(_ context.Context, req *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	i.Logger.Info("attachment requested")

	// Parse the request that has already been validated against the input schema
	var request *attachmentRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid attachment request: %w", err)
	}

	// ....

	// You also have access to the configuration and credentials from the registration phase
	// They can be accessed via
	// var rc *registrationState
	// if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &rc); err != nil {
	//  return nil, fmt.Errorf("invalid registration configuration %w", err)
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

// Execute will be instantiated when either an attestation or a material has been received
// It's up to the plugin builder to differentiate between inputs
func (i *Integration) Execute(_ context.Context, req *sdk.ExecutionRequest) error {
	i.Logger.Info("execution requested")

	// You can receive more than one material
	for _, sbom := range req.Input.Materials {
		// Example of custom validation
		if err := validateExecuteOpts(sbom, req.RegistrationInfo, req.AttachmentInfo); err != nil {
			return fmt.Errorf("running validation: %w", err)
		}

		// Extract registration and attachment configuration if needed
		// var registrationConfig *registrationState
		// if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &registrationConfig); err != nil {
		//  return fmt.Errorf("invalid registration configuration %w", err)
		// }

		// // Extract attachment configuration
		// var attachmentConfig *attachmentState
		// if err := sdk.FromConfig(req.AttachmentInfo.Configuration, &attachmentConfig); err != nil {
		//  return fmt.Errorf("invalid attachment configuration %w", err)
		// }

		// START CUSTOM LOGIC
		// EO CUSTOM LOGIC
	}

	i.Logger.Info("execution finished")
	return nil
}

// Validator example for the execution phase
// In this case we expect to receive a SBOM in CycloneDX format
// plus registration, attachment state and credentials
func validateExecuteOpts(m *sdk.ExecuteMaterial, regConfig *sdk.RegistrationResponse, attConfig *sdk.AttachmentResponse) error {
	if m == nil || m.Content == nil {
		return errors.New("invalid input")
	}

	if m.Type != schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String() {
		return fmt.Errorf("invalid input type: %s", m.Type)
	}

	if regConfig == nil || regConfig.Configuration == nil {
		return errors.New("missing registration configuration")
	}

	if regConfig.Credentials == nil {
		return errors.New("missing credentials")
	}

	if attConfig == nil || attConfig.Configuration == nil {
		return errors.New("missing attachment configuration")
	}

	return nil
}
