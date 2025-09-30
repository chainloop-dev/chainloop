// Copyright 2025 The Chainloop Authors.
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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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

// registrationRequest defines the configuration required during registration
type registrationRequest struct {
	URL string `json:"url" jsonschema:"minLength=1,description=Webhook URL to send payloads to"`
}

// attachmentRequest defines the configuration required during attachment
type attachmentRequest struct {
	SendAttestation *bool `json:"send_attestation,omitempty" jsonschema:"description=Send attestation,default=true"`
	SendSBOM        *bool `json:"send_sbom,omitempty" jsonschema:"description=Additionally send CycloneDX or SPDX Software Bill Of Materials (SBOM),default=false"`
}

// attachmentState defines the state stored after attachment
type attachmentState struct {
	SendAttestation bool `json:"send_attestation"`
	SendSBOM        bool `json:"send_sbom"`
}

// registrationState defines the state stored after registration
type registrationState struct {
	// No additional state needed for webhook besides the URL stored in credentials
}

// webhookPayload defines the JSON schema for the webhook payload
type webhookPayload struct {
	Metadata *sdk.ChainloopMetadata `json:"Metadata"`
	Data     []byte                 `json:"Data"` // e.g., SBOM or attestation raw content in bytes
	Kind     string                 `json:"Kind"` // e.g., "SBOM_CYCLONEDX_JSON", "ATTESTATION"
}

// New initializes the webhook integration
func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "webhook",
			Version:     "1.0",
			Description: "Send Attestation and SBOMs to a generic POST webhook URL",
			Logger:      l,
			InputSchema: &sdk.InputSchema{
				Registration: registrationRequest{},
				Attachment:   attachmentRequest{},
			},
		},
		// In addition to the attestation payload the following material types are also available
		sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON),
		sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create FanOut integration: %w", err)
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
		i.Logger.Errorw("failed to parse registration payload", "error", err)
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Validate the URL
	if err := validateURL(regReq.URL); err != nil {
		i.Logger.Errorw("invalid webhook URL", "error", err, "url", regReq.URL)
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	// Optionally, perform a test request to ensure the webhook URL is reachable
	if err := i.testWebhookURL(ctx, regReq.URL); err != nil {
		i.Logger.Errorw("unable to reach webhook URL", "error", err, "url", regReq.URL)
		return nil, fmt.Errorf("unable to reach webhook URL: %w", err)
	}

	// Store the URL in credentials
	credentials := &sdk.Credentials{
		URL: regReq.URL, // Storing the URL in the URL field
	}

	// No additional state needed
	rawConfig, err := sdk.ToConfig(&registrationState{})
	if err != nil {
		i.Logger.Errorw("failed to marshal registration state", "error", err)
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
		i.Logger.Errorw("failed to parse attachment payload", "error", err)
		return nil, fmt.Errorf("invalid attachment request: %w", err)
	}

	// Set default values if not provided
	sendAttestation := true
	if attachReq.SendAttestation != nil {
		sendAttestation = *attachReq.SendAttestation
	}

	sendSBOM := false
	if attachReq.SendSBOM != nil {
		sendSBOM = *attachReq.SendSBOM
	}

	// Store the settings in the attachment state
	rawConfig, err := sdk.ToConfig(&attachmentState{
		SendAttestation: sendAttestation,
		SendSBOM:        sendSBOM,
	})
	if err != nil {
		i.Logger.Errorw("failed to marshal attachment state", "error", err)
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
		i.Logger.Error("missing webhook URL in credentials")
		return errors.New("missing webhook URL in credentials")
	}
	webhookURL := req.RegistrationInfo.Credentials.URL

	// Extract the settings from the attachment state
	var attachState attachmentState
	if err := sdk.FromConfig(req.AttachmentInfo.Configuration, &attachState); err != nil {
		i.Logger.Errorw("invalid attachment state", "error", err)
		return fmt.Errorf("invalid attachment state: %w", err)
	}

	// Send attestation if enabled and present
	if attachState.SendAttestation && req.Input.Attestation != nil {
		statementJSON, err := json.Marshal(req.Input.Attestation)
		if err != nil {
			i.Logger.Errorw("failed to marshal attestation", "error", err)
			return fmt.Errorf("marshalling attestation: %w", err)
		}
		if err := i.sendWebhook(ctx, webhookURL, "ATTESTATION", statementJSON, req.ChainloopMetadata); err != nil {
			i.Logger.Errorw("failed to send attestation webhook", "error", err)
			return err
		}
	}

	// Send SBOM if enabled and present
	if attachState.SendSBOM {
		for _, material := range req.Input.Materials {
			// Ensure material type is either SBOM_CYCLONEDX_JSON or SBOM_SPDX_JSON
			if material.Type != schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String() && material.Type != schemaapi.CraftingSchema_Material_SBOM_SPDX_JSON.String() {
				i.Logger.Warnw("unsupported material type, skipping", "type", material.Type)
				continue
			}
			// Validate material content
			if len(material.Content) == 0 {
				i.Logger.Warnw("encountered SBOM with empty content, skipping", "type", material.Type)
				continue
			}

			// Send the SBOM webhook
			if err := i.sendWebhook(ctx, webhookURL, material.Type, material.Content, req.ChainloopMetadata); err != nil {
				i.Logger.Errorw("failed to send SBOM webhook", "error", err, "type", material.Type)
				return err
			}
		}
	}

	return nil
}

// sendWebhook sends a webhook with the specified kind and payload
func (i *Integration) sendWebhook(ctx context.Context, url, kind string, payload []byte, metadata *sdk.ChainloopMetadata) error {
	payloadBytes, err := json.Marshal(webhookPayload{
		Metadata: metadata,
		Data:     payload,
		Kind:     kind,
	})
	if err != nil {
		i.Logger.Errorw("failed to marshal webhook payload", "error", err, "kind", kind)
		return fmt.Errorf("marshalling webhook payload: %w", err)
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		i.Logger.Errorw("failed to create HTTP request", "error", err, "url", url)
		return fmt.Errorf("creating HTTP request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := i.client.Do(req)
	if err != nil {
		i.Logger.Errorw("failed to send HTTP request", "error", err, "url", url)
		return fmt.Errorf("sending HTTP request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			i.Logger.Warnw("failed to close response body", "error", err)
		}
	}()

	// Read response body for more detailed error messages
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		i.Logger.Warnw("failed to read response body", "error", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		i.Logger.Errorw("webhook responded with non-success status code", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("webhook responded with status code %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// testWebhookURL sends a test webhook using the sendWebhook method to ensure the webhook URL is reachable
func (i *Integration) testWebhookURL(ctx context.Context, webhookURL string) error {
	// Define dummy metadata for the test
	dummyMetadata := &sdk.ChainloopMetadata{
		Workflow: &sdk.ChainloopMetadataWorkflow{Name: "test-webhook-workflow"},
		WorkflowRun: &sdk.ChainloopMetadataWorkflowRun{
			ID: "test-webhook-run",
		},
	}

	// Define a dummy payload (empty JSON object)
	dummyData := []byte("{}")

	// Define a unique kind for the test
	testKind := "TEST_WEBHOOK"

	// Use sendWebhook to send the test payload
	if err := i.sendWebhook(ctx, webhookURL, testKind, dummyData, dummyMetadata); err != nil {
		return fmt.Errorf("test webhook failed: %w", err)
	}

	return nil
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
