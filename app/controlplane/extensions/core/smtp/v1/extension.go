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

package smtp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	nsmtp "net/smtp"

	"github.com/chainloop-dev/chainloop/app/controlplane/extensions/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/in-toto/in-toto-golang/in_toto"
)

type Integration struct {
	*sdk.FanOutIntegration
}

type registrationRequest struct {
	To       string `json:"to" jsonschema:"required,format=email"`
	From     string `json:"from" jsonschema:"required,format=email"`
	User     string `json:"user" jsonschema:"required,minLength=1"`
	Password string `json:"password" jsonschema:"required"`
	Host     string `json:"host" jsonschema:"required,format="`
	Port     string `json:"port" jsonschema:"required"`
}

type registrationState struct {
	To   string `json:"to"`
	From string `json:"from"`
	User string `json:"user"`
	Host string `json:"host"`
	Port string `json:"port"`
}

type attachmentRequest struct {
	Cc string `json:"cc" jsonschema:"format=email"`
}

type attachmentState struct {
	Cc string `json:"cc"`
}

func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:      "smtp",
			Version: "0.1",
			Logger:  l,
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

	// Unmarshal the request
	var request *registrationRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	response := &sdk.RegistrationResponse{}

	to, from, user, password, host, port := request.To, request.From, request.User, request.Password, request.Host, request.Port
	rawConfig, err := sdk.ToConfig(&registrationState{To: to, From: from, User: user, Host: host, Port: port})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}
	response.Configuration = rawConfig
	response.Credentials = &sdk.Credentials{Password: password}

	// validate and notify
	subject := "[chainloop] New SMTP integration added!"
	tpl := `
	We successfully registered a new SMTP integration in your Chainloop organization.

	Extension: %s version: %s
	- Host: %s
	- Port: %s
	- User: %s
	- From: %s
	- To: %s
	`
	body := fmt.Sprintf(tpl, i.Describe().ID, i.Describe().Version, host, port, user, from, to)
	sendEmail(host, port, user, password, from, to, "", subject, body)

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

	var rc *registrationState
	if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &rc); err != nil {
		return nil, errors.New("invalid registration configuration")
	}

	response := &sdk.AttachmentResponse{}
	rawConfig, err := sdk.ToConfig(&attachmentState{Cc: request.Cc})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}
	response.Configuration = rawConfig

	return response, nil
}

// Send the SBOM to the configured Dependency Track instance
func (i *Integration) Execute(_ context.Context, req *sdk.ExecutionRequest) error {
	i.Logger.Info("execution requested")

	if err := validateExecuteRequest(req); err != nil {
		return fmt.Errorf("running validation for workflow id %s: %w", req.WorkflowID, err)
	}

	var rc *registrationState
	if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &rc); err != nil {
		return errors.New("invalid registration configuration")
	}

	var ac *attachmentState
	if err := sdk.FromConfig(req.AttachmentInfo.Configuration, &ac); err != nil {
		return errors.New("invalid attachment configuration")
	}

	// get the attestation
	decodedPayload, err := req.Input.DSSEnvelope.DecodeB64Payload()
	if err != nil {
		return err
	}
	statement := &in_toto.Statement{}
	if err := json.Unmarshal(decodedPayload, statement); err != nil {
		return fmt.Errorf("un-marshaling predicate: %w", err)
	}
	jsonBytes, err := json.MarshalIndent(statement, "", "  ")
	i.Logger.Info("statement", string(jsonBytes))
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return err
	}

	// send the email
	to, from, user, password, host, port := rc.To, rc.From, rc.User, req.RegistrationInfo.Credentials.Password, rc.Host, rc.Port
	subject := "[chainloop] New workflow run finished successfully!"
	tpl := `A new workflow run finished successfully!

# Workflow: %s

# in-toto statement:
	%s

This email has been delivered via integration %s version %s.
	`
	body := fmt.Sprintf(tpl, req.WorkflowID, jsonBytes, i.Describe().ID, i.Describe().Version)
	sendEmail(host, port, user, password, from, to, ac.Cc, subject, body)

	return nil
}

func validateExecuteRequest(req *sdk.ExecutionRequest) error {
	if req == nil || req.Input == nil || req.Input.DSSEnvelope == nil {
		return errors.New("invalid input")
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

func sendEmail(host string, port string, user, password, from, to, cc, subject, body string) error {
	message := "From: " + from + "\n" +
		"To: " + to + "\n" +
		"Cc: " + cc + "\n" +
		"Subject: " + subject + "\n\n" +
		body

	auth := nsmtp.PlainAuth("", user, password, host)
	err := nsmtp.SendMail(host+":"+port, auth, from, []string{to}, []byte(message))
	if err != nil {
		fmt.Println("Error sending email1:", err)
		return err
	}

	return nil
}
