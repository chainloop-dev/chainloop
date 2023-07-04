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

package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	WebhookURL string `json:"webhook" jsonschema:"format=uri,description=URL of the slack webhook"`
}

type attachmentRequest struct{}

func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "slack-webhook",
			Version:     "1.0",
			Description: "Send attestations to Slack",
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

// Register is executed when a operator wants to register a specific instance of this integration with their Chainloop organization
func (i *Integration) Register(_ context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	var request *registrationRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	if err := executeWebhook(request.WebhookURL, "This is a test message. Welcome to Chainloop!"); err != nil {
		return nil, fmt.Errorf("error validating a webhook: %w", err)
	}

	return &sdk.RegistrationResponse{
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

	attestationJSON, err := json.MarshalIndent(req.Input.Attestation.Statement, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	metadata := req.ChainloopMetadata
	// I was not able to make backticks work in the template
	a := fmt.Sprintf("\n```\n%s\n```\n", string(attestationJSON))
	tplData := &templateContent{
		WorkflowID:      metadata.WorkflowID,
		WorkflowName:    metadata.WorkflowName,
		WorkflowRunID:   metadata.WorkflowRunID,
		WorkflowProject: metadata.WorkflowProject,
		RunnerLink:      req.Input.Attestation.Predicate.GetRunLink(),
		Attestation:     a,
	}

	webhookURL := req.RegistrationInfo.Credentials.Password
	if err := executeWebhook(webhookURL, renderContent(tplData)); err != nil {
		return fmt.Errorf("error executing webhook: %w", err)
	}

	i.Logger.Info("execution finished")
	return nil
}

// Send attestation to Slack
func executeWebhook(webhookURL, msgContent string) error {
	payload := map[string]string{
		"text": msgContent,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error encoding payload: %w", err)
	}

	requestBody := bytes.NewReader(jsonPayload)

	// #nosec G107 - we are using a constant API URL that is not user input at this stage
	r, err := http.Post(webhookURL, "application/json", requestBody)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(r.Body)
		return fmt.Errorf("non-OK HTTP status while calling the webhook: %d, body: %s", r.StatusCode, string(b))
	}

	return nil
}

func validateExecuteRequest(req *sdk.ExecutionRequest) error {
	if req == nil || req.Input == nil {
		return errors.New("execution input not received")
	}

	if req.Input.Attestation == nil {
		return errors.New("execution input invalid, envelope is nil")
	}

	if req.RegistrationInfo == nil {
		return errors.New("missing registration configuration")
	}

	if req.RegistrationInfo.Credentials == nil {
		return errors.New("missing credentials")
	}

	return nil
}

type templateContent struct {
	WorkflowID, WorkflowName, WorkflowProject, WorkflowRunID, RunnerLink, Attestation string
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
{{.Attestation}}
`
