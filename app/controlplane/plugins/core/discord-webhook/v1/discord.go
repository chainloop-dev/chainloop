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

package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"text/template"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
)

type Integration struct {
	*sdk.FanOutIntegration
}

// 1 - API schema definitions
type registrationRequest struct {
	WebhookURL string `json:"webhook" jsonschema:"format=uri,description=URL of the discord webhook"`
	Username   string `json:"username,omitempty" jsonschema:"minLength=1,description=Override the default username of the webhook"`
}

type attachmentRequest struct{}

// 2 - Configuration state
type registrationState struct {
	// Information from the webhook
	WebhookName  string `json:"name"`
	WebhookOwner string `json:"owner"`

	// Username to be used while posting the message
	Username string `json:"username,omitempty"`
}

func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "discord-webhook",
			Version:     "1.1",
			Description: "Send attestations to Discord",
			Logger:      l,
			InputSchema: &sdk.InputSchema{
				Registration: registrationRequest{},
				Attachment:   attachmentRequest{},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	return &Integration{base}, nil
}

type webhookResponse struct {
	Name string `json:"name"`
	User struct {
		Username string `json:"username"`
	} `json:"user"`
}

// Register is executed when a operator wants to register a specific instance of this integration with their Chainloop organization
func (i *Integration) Register(_ context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	var request *registrationRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Test the webhook URL and extract some information from it to use it as reference for the user
	resp, err := http.Get(request.WebhookURL)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("invalid webhook URL")
	}

	var webHookInfo webhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&webHookInfo); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	// Configuration State
	config, err := sdk.ToConfig(&registrationState{
		WebhookName:  webHookInfo.Name,
		WebhookOwner: webHookInfo.User.Username,
		Username:     request.Username,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	return &sdk.RegistrationResponse{
		Configuration: config,
		// We treat the webhook URL as a sensitive field so we store it in the credentials storage
		Credentials: &sdk.Credentials{Password: request.WebhookURL},
	}, nil
}

// Attachment is executed when to attach a registered instance of this integration to a specific workflow
func (i *Integration) Attach(_ context.Context, _ *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	i.Logger.Info("attachment requested")
	return &sdk.AttachmentResponse{}, nil
}

// Execute will be instantiated when either an attestation or a material has been received
// It's up to the plugin builder to differentiate between inputs
func (i *Integration) Execute(_ context.Context, req *sdk.ExecutionRequest) error {
	i.Logger.Info("execution requested")

	if err := validateExecuteRequest(req); err != nil {
		return fmt.Errorf("running validation: %w", err)
	}

	var config *registrationState
	if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &config); err != nil {
		return fmt.Errorf("invalid registration config: %w", err)
	}

	attestationJSON, err := json.MarshalIndent(req.Input.Attestation.Statement, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	metadata := req.ChainloopMetadata
	tplData := &templateContent{
		WorkflowID:      metadata.WorkflowID,
		WorkflowName:    metadata.WorkflowName,
		WorkflowRunID:   metadata.WorkflowRunID,
		WorkflowProject: metadata.WorkflowProject,
		RunnerLink:      req.Input.Attestation.Predicate.GetRunLink(),
	}

	webhookURL := req.RegistrationInfo.Credentials.Password
	if err := executeWebhook(webhookURL, config.Username, attestationJSON, renderContent(tplData)); err != nil {
		return fmt.Errorf("error executing webhook: %w", err)
	}

	i.Logger.Info("execution finished")
	return nil
}

// Send attestation to Discord

// https://discord.com/developers/docs/reference#uploading-files
// --boundary
// Content-Disposition: form-data; name="payload_json"
// Content-Type: application/json
//
//	{
//	  "content": "New attestation!",
//	  "attachments": [{
//	      "id": 0,
//	      "filename": "attestation.json"
//	  }]
//	}
//
// --boundary
// Content-Disposition: form-data; name="files[0]"; filename="statement.json"
// --boundary
func executeWebhook(webhookURL, usernameOverride string, jsonStatement []byte, msgContent string) error {
	var b bytes.Buffer
	multipartWriter := multipart.NewWriter(&b)

	// webhook POST payload JSON
	payload := payloadJSON{
		Content:  msgContent,
		Username: usernameOverride,
		Attachments: []payloadAttachment{
			{
				ID:       0,
				Filename: "attestation.json",
			},
		},
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}

	payloadWriter, err := multipartWriter.CreateFormField("payload_json")
	if err != nil {
		return fmt.Errorf("creating payload form field: %w", err)
	}

	if _, err := payloadWriter.Write(payloadJSON); err != nil {
		return fmt.Errorf("writing payload form field: %w", err)
	}

	// attach attestation JSON
	attachmentWriter, err := multipartWriter.CreateFormFile("files[0]", "statement.json")
	if err != nil {
		return fmt.Errorf("creating attachment form field: %w", err)
	}

	if _, err := attachmentWriter.Write(jsonStatement); err != nil {
		return fmt.Errorf("writing attachment form field: %w", err)
	}

	// Needed to dump the content of the multipartWriter to the buffer
	multipartWriter.Close()

	// #nosec G107 - we are using a constant API URL that is not user input at this stage
	r, err := http.Post(webhookURL, multipartWriter.FormDataContentType(), &b)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		return fmt.Errorf("non-OK HTTP status while calling the webhook: %d, body: %s", r.StatusCode, string(b))
	}

	return nil
}

type payloadJSON struct {
	Content     string              `json:"content"`
	Username    string              `json:"username,omitempty"`
	Attachments []payloadAttachment `json:"attachments"`
}

type payloadAttachment struct {
	ID       int    `json:"id"`
	Filename string `json:"filename"`
}

func validateExecuteRequest(req *sdk.ExecutionRequest) error {
	if req == nil || req.Input == nil {
		return errors.New("execution input not received")
	}

	if req.Input.Attestation == nil {
		return errors.New("execution input invalid, envelope is nil")
	}

	if req.RegistrationInfo == nil || req.RegistrationInfo.Configuration == nil {
		return errors.New("missing registration configuration")
	}

	if req.RegistrationInfo.Credentials == nil {
		return errors.New("missing credentials")
	}

	return nil
}

type templateContent struct {
	WorkflowID, WorkflowName, WorkflowProject, WorkflowRunID, RunnerLink string
}

func renderContent(metadata *templateContent) string {
	t := template.Must(template.New("content").Parse(msgTemplate))

	var b bytes.Buffer
	if err := t.Execute(&b, metadata); err != nil {
		return ""
	}

	return strings.Trim(b.String(), "\n")
}

const msgTemplate = `
New attestation received!
- Workflow: {{.WorkflowProject}}/{{.WorkflowName}}
- Workflow Run: {{.WorkflowRunID}}
{{- if .RunnerLink }}
- Link to runner: {{.RunnerLink}}
{{end}}
`
