// File: app/controlplane/plugins/core/webhook/v1/webhook.go

//
// Copyright 2024 Shebash.io
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

package webhook

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"


	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
)

// Integration implements a generic webhook integration
type Integration struct {
	*sdk.FanOutIntegration
	client *http.Client
}

const (
	providerWebhook = "webhook"
)

// registrationRequest defines the configuration required during registration
type registrationRequest struct {
	URL string `json:"url" jsonschema:"minLength=1,description=Webhook URL to send payloads to"`
}

// attachmentRequest defines the configuration required during attachment
type attachmentRequest struct {
	Materials string `json:"materials,omitempty" jsonschema:"description=Comma-separated list of materials to send (e.g., sbom, attestation)"`
}

// attachmentState defines the state stored after attachment
type attachmentState struct {
	Materials []string `json:"materials"`
}

// registrationState defines the state stored after registration
type registrationState struct {
	// No additional state needed for webhook besides the URL stored in credentials
}

// webhookPayload defines the JSON schema for the webhook payload
type webhookPayload struct {
	Metadata *sdk.ChainloopMetadata `json:"Metadata"`
	Data     json.RawMessage        `json:"Data"`     // e.g., SBOM or attestation JSON
	Kind     string                 `json:"Kind"`      // e.g., "SBOM_CYCLONEDX", "ATTESTATION"
}

// New initializes the webhook integration
func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "webhook",
			Version:     "1.0",
			Description: "Send Attestation and SBOMs to a generic webhook URL as JSON payloads",
			Logger:      l,
			InputSchema: &sdk.InputSchema{
				Registration: registrationRequest{},
				Attachment:   attachmentRequest{},
			},
		},
		// Subscribe to SBOMs only, attestations are handled separately
		sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON),
		sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON),
	)
	if err != nil {
		return nil, err
	}

	return &Integration{
		FanOutIntegration: base,
		client:            &http.Client{},
	}, nil
}

// Register is executed when registering the webhook integration
func (i *Integration) Register(ctx context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	// Parse the registration payload
	var regReq registrationRequest
	if err := sdk.FromConfig(req.Payload, &regReq); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Validate the URL
	if err := validateURL(regReq.URL); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	// Optionally, perform a test request to ensure the webhook URL is reachable
	if err := testWebhookURL(i.client, regReq.URL); err != nil {
		return nil, fmt.Errorf("unable to reach webhook URL: %w", err)
	}

	// Store the URL in credentials
	credentials := &sdk.Credentials{
		URL: regReq.URL, // Storing the URL in the URL field
	}

	// No additional state needed
	rawConfig, err := sdk.ToConfig(&registrationState{})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	return &sdk.RegistrationResponse{
		Credentials:   credentials,
		Configuration: rawConfig,
	}, nil
}

// Attach is executed when attaching the webhook integration to a workflow
func (i *Integration) Attach(_ context.Context, req *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	// Parse the attachment payload
	var attachReq attachmentRequest
	if err := sdk.FromConfig(req.Payload, &attachReq); err != nil {
		return nil, fmt.Errorf("invalid attachment request: %w", err)
	}

	// Split the materials string into a slice
	materials := []string{}
	if attachReq.Materials != "" {
		materials = append(materials, attachReq.Materials)
	}

	// Store the materials in the attachment state
	rawConfig, err := sdk.ToConfig(&attachmentState{Materials: materials})
	if err != nil {
		return nil, fmt.Errorf("marshalling attachment state: %w", err)
	}

	return &sdk.AttachmentResponse{
		Configuration: rawConfig,
	}, nil
}

// Execute is called when an attestation or SBOM is received
func (i *Integration) Execute(ctx context.Context, req *sdk.ExecutionRequest) error {
	// Extract the webhook URL from credentials
	if req.RegistrationInfo.Credentials == nil || req.RegistrationInfo.Credentials.URL == "" {
		return errors.New("missing webhook URL in credentials")
	}
	webhookURL := req.RegistrationInfo.Credentials.URL

	// Extract the materials from the attachment state
	var attachState attachmentState
	if err := sdk.FromConfig(req.AttachmentInfo.Configuration, &attachState); err != nil {
		return fmt.Errorf("invalid attachment state: %w", err)
	}

	// Send attestation if present
	if req.Input.Attestation != nil {
		statementJSON, err := json.Marshal(req.Input.Attestation)
		if err != nil {
			return fmt.Errorf("marshalling attestation: %w", err)
		}
		if err := i.sendWebhook(ctx, webhookURL, "ATTESTATION", statementJSON, req.ChainloopMetadata); err != nil {
			return err
		}
	}

	// Send each SBOM if present and specified in the attachment state
	for _, material := range req.Input.Materials {
		encodedContent := base64.StdEncoding.EncodeToString(material.Content)
		// create json message with the content
		jsonMsg := fmt.Sprintf(`{"content": "%s"}`, encodedContent)
		if err := i.sendWebhook(ctx, webhookURL, material.Type, json.RawMessage(jsonMsg), req.ChainloopMetadata); err != nil {
			return err
		}
	}

	return nil
}

// sendWebhook sends a webhook with the specified kind and payload
func (i *Integration) sendWebhook(ctx context.Context, url, kind string, payload json.RawMessage, metadata *sdk.ChainloopMetadata) error {
	payloadBytes, err := json.Marshal(webhookPayload{
		Metadata: metadata,
		Data:     payload,
		Kind:     kind,
	})
	if err != nil {
		return fmt.Errorf("marshalling webhook payload: %w", err)
	}

	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("creating HTTP request: %w", err)
	}
	reqHTTP.Header.Set("Content-Type", "application/json")

	resp, err := i.client.Do(reqHTTP)
	if err != nil {
		return fmt.Errorf("sending HTTP request to webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook responded with status code %d", resp.StatusCode)
	}

	i.Logger.Infow("msg", "sent payload to webhook", "url", url, "status", resp.StatusCode)

	return nil
}

// isBinary checks if the content is binary data
func isBinary(content []byte) bool {
	// Simple heuristic: check for non-printable characters
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}

// validateURL performs basic validation of the webhook URL
func validateURL(webhookURL string) error {
	parsedURL, err := url.ParseRequestURI(webhookURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}
	return nil
}

// testWebhookURL sends a POST request to ensure the webhook URL is reachable
func testWebhookURL(client *http.Client, webhookURL string) error {
	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewBuffer([]byte(`{"test": "test"}`)))
	if err != nil {
		return fmt.Errorf("creating POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("performing POST request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("webhook URL responded with status code %d", resp.StatusCode)
	}

	return nil
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
