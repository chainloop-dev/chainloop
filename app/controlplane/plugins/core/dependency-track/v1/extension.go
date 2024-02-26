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

package dependencytrack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"text/template"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/core/dependency-track/v1/client"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/invopop/jsonschema"
)

type DependencyTrack struct {
	*sdk.FanOutIntegration
}

// Request schemas for both registration and attachment
type registrationRequest struct {
	// The URL of the Dependency-Track instance
	InstanceURI string `json:"instanceURI" jsonschema:"format=uri,description=The URL of the Dependency-Track instance"`
	APIKey      string `json:"apiKey" jsonschema:"description=The API key to use for authentication"`
	// Support the option to automatically create projects if requested (optional)
	AllowAutoCreate bool `json:"allowAutoCreate,omitempty" jsonschema:"description=Support of creating projects on demand"`
}

type attachmentRequest struct {
	// Either one or the other
	ProjectID   string `json:"projectID,omitempty" jsonschema:"oneof_required=projectID,minLength=1,description=The ID of the existing project to send the SBOMs to"`
	ProjectName string `json:"projectName,omitempty" jsonschema:"oneof_required=projectName,minLength=1,description=The name of the project to create and send the SBOMs to"`

	ParentID string `json:"parentID,omitempty" jsonschema:"minLength=1,description=ID of parent project to create a new project under"`
}

// Enforces the requirement that parentID requires the presence of projectName
// invopop/jsonschema doesn't appear to support dependentRequired through reflection
func (x attachmentRequest) JSONSchemaExtend(schema *jsonschema.Schema) {
	schema.DependentRequired = map[string][]string{
		"parentID": {
			"projectName",
		},
	}
}

// Internal state for both registration and attachment
type registrationConfig struct {
	Domain          string `json:"domain"`
	AllowAutoCreate bool   `json:"allowAutoCreate"`
}

type attachmentConfig struct {
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
	ParentID    string `json:"parentId"`
}

const description = "Send CycloneDX SBOMs to your Dependency-Track instance"

func New(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanOut(
		&sdk.NewParams{
			ID:          "dependency-track",
			Version:     "1.4",
			Description: description,
			Logger:      l,
			InputSchema: &sdk.InputSchema{
				Registration: registrationRequest{},
				Attachment:   attachmentRequest{},
			},
		}, sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON))

	if err != nil {
		return nil, err
	}

	return &DependencyTrack{base}, nil
}

func (i *DependencyTrack) Register(ctx context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	var request *registrationRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid registration request: %w", err)
	}

	// Validate that the provided configuration is valid
	instance, enableProjectCreation := request.InstanceURI, request.AllowAutoCreate
	checker, err := client.NewIntegration(instance, request.APIKey, enableProjectCreation)
	if err != nil {
		return nil, fmt.Errorf("checking integration: %w", err)
	}

	// Validate that the provided configuration is valid against the remote service
	if err := checker.Validate(ctx); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	i.Logger.Infow("msg", "registration OK", "instance", instance, "allowAutoCreate", enableProjectCreation)

	rawConfig, err := sdk.ToConfig(&registrationConfig{Domain: instance, AllowAutoCreate: enableProjectCreation})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	// Return what configuration to store in the database and what to store in the external secrets manager
	return &sdk.RegistrationResponse{
		Credentials:   &sdk.Credentials{Password: request.APIKey},
		Configuration: rawConfig,
	}, nil
}

// Validate and return what configuration attachment to persist
func (i *DependencyTrack) Attach(ctx context.Context, req *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	i.Logger.Info("attachment requested")

	var request attachmentRequest
	if err := sdk.FromConfig(req.Payload, &request); err != nil {
		return nil, fmt.Errorf("invalid attachment request: %w", err)
	}

	// Extract registration configuration
	var rc *registrationConfig
	if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &rc); err != nil {
		return nil, fmt.Errorf("invalid registration configuration: %w", err)
	}

	if err := validateAttachment(ctx, rc, &request, req.RegistrationInfo.Credentials); err != nil {
		return nil, fmt.Errorf("invalid attachment configuration: %w", err)
	}

	i.Logger.Infow("msg", "attachment OK", "projectID", request.ProjectID, "projectName", request.ProjectName, "parentID", request.ParentID)

	// We want to store the project configuration
	rawConfig, err := sdk.ToConfig(&attachmentConfig{ProjectID: request.ProjectID, ProjectName: request.ProjectName, ParentID: request.ParentID})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	return &sdk.AttachmentResponse{Configuration: rawConfig}, nil
}

// Send the SBOMs to the configured Dependency Track instance
func (i *DependencyTrack) Execute(ctx context.Context, req *sdk.ExecutionRequest) error {
	var errs error
	// Iterate over all SBOMs
	for _, sbom := range req.Input.Materials {
		if err := doExecute(ctx, req, sbom, i.Logger); err != nil {
			errs = errors.Join(errs, err)
			continue
		}
	}

	if errs != nil {
		return fmt.Errorf("executing: %w", errs)
	}

	return nil
}

func doExecute(ctx context.Context, req *sdk.ExecutionRequest, sbom *sdk.ExecuteMaterial, l *log.Helper) error {
	l.Info("execution requested")

	// Make sure it's an SBOM and all the required configuration has been received
	if err := validateExecuteOpts(sbom, req.RegistrationInfo, req.AttachmentInfo); err != nil {
		return fmt.Errorf("running validation: %w", err)
	}

	// Extract registration configuration
	var registrationConfig *registrationConfig
	if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &registrationConfig); err != nil {
		return errors.New("invalid registration configuration")
	}

	// Extract attachment configuration
	var attachmentConfig *attachmentConfig
	if err := sdk.FromConfig(req.AttachmentInfo.Configuration, &attachmentConfig); err != nil {
		return errors.New("invalid attachment configuration")
	}

	// Calculate the project name based on the template

	projectName, err := resolveProjectName(attachmentConfig.ProjectName, req.Input.Attestation.Predicate.GetAnnotations(), sbom.Annotations)
	if err != nil {
		// If we can't find the annotation for example, we skip the SBOM
		l.Infow("msg", "failed to resolve project name, SKIPPING", "err", err, "materialName", sbom.Name)
		return nil
	}

	l.Infow("msg", "Uploading SBOM",
		"materialName", sbom.Name,
		"host", registrationConfig.Domain,
		"projectID", attachmentConfig.ProjectID, "projectName", projectName,
		"workflowID", req.Workflow.ID,
	)

	// Create an SBOM client and perform validation and upload
	d, err := client.NewSBOMUploader(registrationConfig.Domain,
		req.RegistrationInfo.Credentials.Password,
		bytes.NewReader(sbom.Content),
		attachmentConfig.ProjectID,
		projectName,
		attachmentConfig.ParentID)
	if err != nil {
		return fmt.Errorf("creating uploader: %w", err)
	}

	if err := d.Validate(ctx); err != nil {
		return fmt.Errorf("validating uploader: %w", err)
	}

	if err := d.Do(ctx); err != nil {
		return fmt.Errorf("uploading SBOM: %w", err)
	}

	l.Infow("msg", "SBOM Uploaded",
		"materialName", sbom.Name,
		"host", registrationConfig.Domain,
		"projectID", attachmentConfig.ProjectID, "projectName", projectName,
		"workflowID", req.Workflow.ID,
	)

	l.Info("execution finished")

	return nil
}

type interpolationContext struct {
	Material    *annotations
	Attestation *annotations
}
type annotations struct {
	Annotations map[string]string
}

// Make annotations keys case insensitive
// that way you can define templates such as {{ material.annotations.myAnnotation }} or {{ material.annotations.MyAnnotation }} and they will both work
func toCaseInsensitive(in map[string]string) map[string]string {
	for k, v := range in {
		in[strings.Title(k)] = v
	}

	return in
}

// Resolve the project name template.
// We currently support the following template variables:
// - {{ .Attestation.Annotations.<key> }} for global annotations
// - {{ .Material.Annotations.<key> }}  for material annotations
// For example, project-name => {{ material.annotations.my_annotation }}
func resolveProjectName(projectNameTpl string, attAnnotations, sbomAnnotations map[string]string) (string, error) {
	data := &interpolationContext{
		Material:    &annotations{toCaseInsensitive(sbomAnnotations)},
		Attestation: &annotations{toCaseInsensitive(attAnnotations)},
	}

	// The project name can contain template variables, useful to include annotations for example
	// We do fail if the key can't be found
	tpl, err := template.New("projectName").Option("missingkey=error").Parse(projectNameTpl)
	if err != nil {
		return "", fmt.Errorf("invalid project name: %w", err)
	}

	buf := bytes.NewBuffer(nil)
	if err := tpl.Execute(buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// i.e we want to attach to a dependency track integration and we are proving the right attachment options
// Not only syntactically but also semantically, i.e we can only request auto-creation of projects if the integration allows it
func validateAttachment(ctx context.Context, rc *registrationConfig, ac *attachmentRequest, credentials *sdk.Credentials) error {
	if err := validateAttachmentConfiguration(rc, ac); err != nil {
		return fmt.Errorf("validating attachment configuration: %w", err)
	}

	// Instantiate an actual client to see if it would work with the current configuration
	d, err := client.NewSBOMUploader(rc.Domain, credentials.Password, nil, ac.ProjectID, ac.ProjectName, ac.ParentID)
	if err != nil {
		return fmt.Errorf("creating uploader: %w", err)
	}

	if err := d.Validate(ctx); err != nil {
		return fmt.Errorf("validating uploader: %w", err)
	}

	return nil
}

func validateAttachmentConfiguration(rc *registrationConfig, ac *attachmentRequest) error {
	if rc == nil || ac == nil {
		return errors.New("invalid configuration")
	}

	if ac.ProjectName != "" {
		if !rc.AllowAutoCreate {
			return errors.New("auto creation of projects is not supported in this integration")
		}

		// The project name can contain template variables, useful to include annotations for example
		if _, err := template.New("projectName").Parse(ac.ProjectName); err != nil {
			return fmt.Errorf("invalid project name: %w", err)
		}
	}

	if ac.ProjectID == "" && ac.ProjectName == "" {
		return errors.New("project id or name must be provided")
	}

	if ac.ParentID != "" && ac.ProjectName == "" {
		return errors.New("project name must be provided to work with parent id")
	}

	return nil
}

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
