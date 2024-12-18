package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
)

type Integration struct {
	*sdk.FanOutIntegration
}

// 1 - API schema definitions
type registrationRequest struct {
	WebhookURL string `json:"webhook" jsonschema:"format=uri,description=URL of the webhook"`
}

type attachmentRequest struct{}

// 2 - Configuration state
type registrationState struct {
	// Information from the webhook
	WebhookName string `json:"name"`
}

func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "webhook",
			Version:     "1.0",
			Description: "Send attestations to a generic webhook",
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

// Register is executed when an operator wants to register a specific instance of this integration with their Chainloop organization
func (i *Integration) Register(_ context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	var request *registrationRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Test the webhook URL and extract some information from it to use it as reference for the user
	i.Logger.Info("testing webhook URL POST with empty body")
	resp, err := http.Post(request.WebhookURL, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid webhook URL, status: %s", resp.Status)
	}

	var webHookInfo webhookResponse
	if err := json.NewDecoder(resp.Body).Decode(&webHookInfo); err != nil {
		return nil, fmt.Errorf("invalid webhook URL: %w", err)
	}

	// Configuration State
	config, err := sdk.ToConfig(&registrationState{
		WebhookName: webHookInfo.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	return &sdk.RegistrationResponse{
		Configuration: config,
		// We treat the webhook URL as a sensitive field so we store it in the credentials storage
		Credentials: &sdk.Credentials{URL: request.WebhookURL},
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

	summary, err := sdk.SummaryTable(req)
	if err != nil {
		return fmt.Errorf("generating summary table: %w", err)
	}

	webhookURL := req.RegistrationInfo.Credentials.URL
	if err := executeWebhook(webhookURL, []byte(summary), "New Attestation Received"); err != nil {
		return fmt.Errorf("error executing webhook: %w", err)
	}

	// Handle SBOM and JUNIT materials
	for _, material := range req.Input.Materials {
		switch material.Type {
		case sdk.MaterialTypeSBOM:
			if err := executeWebhook(webhookURL, material.Content, "New SBOM Material Received"); err != nil {
				return fmt.Errorf("error executing webhook for SBOM: %w", err)
			}
		case sdk.MaterialTypeJUNIT:
			if err := executeWebhook(webhookURL, material.Content, "New JUNIT Material Received"); err != nil {
				return fmt.Errorf("error executing webhook for JUNIT: %w", err)
			}
		}
	}

	i.Logger.Info("execution finished")
	return nil
}

// Send attestation to Webhook

// https://webhook.example.com/docs/reference#uploading-files
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
func executeWebhook(webhookURL string, statement []byte, msgContent string) error {
	var b bytes.Buffer
	multipartWriter := multipart.NewWriter(&b)

	// webhook POST payload JSON
	payload := payloadJSON{
		Content: msgContent,
		Attachments: []payloadAttachment{
			{
				ID:       0,
				Filename: "statement.txt",
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
	attachmentWriter, err := multipartWriter.CreateFormFile("files[0]", "statement.txt")
	if err != nil {
		return fmt.Errorf("creating attachment form field: %w", err)
	}

	if _, err := attachmentWriter.Write(statement); err != nil {
		return fmt.Errorf("writing attachment form field: %w", err)
	}

	// Needed to dump the content of the multipartWriter to the buffer
	multipartWriter.Close()

	req, err := http.NewRequest("POST", webhookURL, &b)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	client := &http.Client{}
	r, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
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
