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

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/dependencytrack/cyclonedx/v1/uploader"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/integrations/gen/dependencytrack/cyclonedx/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	"github.com/go-kratos/kratos/v2/log"
)

const ID = "dependencytrack.cyclonedx.v1"

type DependencyTrack struct {
	*sdk.FanOutIntegration
}

type registrationConfig struct {
	Domain          string `json:"domain"`
	AllowAutoCreate bool   `json:"allowAutoCreate"`
}

type attachmentConfig struct {
	ProjectID   string `json:"projectId"`
	ProjectName string `json:"projectName"`
}

// Attach attaches the integration service to the given grpc server.
// In the future this will be a plugin entrypoint
func NewIntegration(l log.Logger) (sdk.FanOut, error) {
	base, err := sdk.NewFanout(
		&sdk.NewParams{
			ID:      "dependencytrack.cyclonedx.v1",
			Version: "1.0",
			Logger:  l,
		}, sdk.WithInputMaterial(schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON))

	if err != nil {
		return nil, err
	}

	base.Logger.Infof("integration initialized: %s", base)

	return &DependencyTrack{base}, nil
}

func (i *DependencyTrack) Register(ctx context.Context, req *sdk.RegistrationRequest) (*sdk.RegistrationResponse, error) {
	i.Logger.Info("registration requested")

	request, ok := req.Payload.(*pb.RegistrationRequest)
	if !ok {
		return nil, errors.New("invalid request")
	}

	// Validate the request payload
	if err := request.ValidateAll(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Validate that the provided configuration is valid
	domain, enableProjectCreation := request.GetDomain(), request.GetAllowAutoCreate()
	checker, err := uploader.NewIntegration(domain, request.ApiKey, enableProjectCreation)
	if err != nil {
		return nil, fmt.Errorf("checking integration: %w", err)
	}

	// Validate that the provided configuration is valid against the remote service
	if err := checker.Validate(ctx); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	i.Logger.Infow("msg", "registration OK", "domain", domain, "allowAutoCreate", enableProjectCreation)

	rawConfig, err := sdk.Config(&registrationConfig{Domain: domain, AllowAutoCreate: enableProjectCreation})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	// Return what configuration to store in the database and what to store in the external secrets manager
	return &sdk.RegistrationResponse{
		Credentials:   &sdk.Credentials{Password: request.GetApiKey()},
		Configuration: rawConfig,
	}, nil
}

// Validate and return what configuration attachment to persist
func (i *DependencyTrack) Attach(ctx context.Context, req *sdk.AttachmentRequest) (*sdk.AttachmentResponse, error) {
	i.Logger.Info("attachment requested")

	request, ok := req.Payload.(*pb.AttachmentRequest)
	if !ok {
		return nil, errors.New("invalid attachment configuration")
	}

	// Validate the request payload
	if err := request.ValidateAll(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Extract registration configuration
	var rc *registrationConfig
	if err := sdk.FromConfig(req.RegistrationInfo.Configuration, &rc); err != nil {
		return nil, errors.New("invalid registration configuration")
	}

	if err := validateAttachment(ctx, rc, request, req.RegistrationInfo.Credentials); err != nil {
		return nil, fmt.Errorf("invalid attachment configuration: %w", err)
	}

	i.Logger.Infow("msg", "attachment OK", "project", request.GetProject())

	// We want to store the project configuration
	rawConfig, err := sdk.Config(&attachmentConfig{ProjectID: request.GetProjectId(), ProjectName: request.GetProjectName()})
	if err != nil {
		return nil, fmt.Errorf("marshalling configuration: %w", err)
	}

	return &sdk.AttachmentResponse{Configuration: rawConfig}, nil
}

// Send the SBOM to the configured Dependency Track instance
func (i *DependencyTrack) Execute(ctx context.Context, req *sdk.ExecutionRequest) error {
	i.Logger.Info("execution requested")

	if err := validateExecuteOpts(req); err != nil {
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

	i.Logger.Infow("msg", "Uploading SBOM",
		"host", registrationConfig.Domain,
		"projectID", attachmentConfig.ProjectID, "projectName", attachmentConfig.ProjectName,
		"workflowID", req.WorkflowID,
	)

	// Create an SBOM uploader and perform validation and upload
	d, err := uploader.NewSBOMUploader(registrationConfig.Domain,
		req.RegistrationInfo.Credentials.Password,
		bytes.NewReader(req.Input.Material.Content),
		attachmentConfig.ProjectID,
		attachmentConfig.ProjectName)
	if err != nil {
		return fmt.Errorf("creating uploader: %w", err)
	}

	if err := d.Validate(ctx); err != nil {
		return fmt.Errorf("validating uploader: %w", err)
	}

	if err := d.Do(ctx); err != nil {
		return fmt.Errorf("uploading SBOM: %w", err)
	}

	i.Logger.Infow("msg", "SBOM Uploaded",
		"host", registrationConfig.Domain,
		"projectID", attachmentConfig.ProjectID, "projectName", attachmentConfig.ProjectName,
		"workflowID", req.WorkflowID,
	)

	return nil
}

// i.e we want to attach to a dependency track integration and we are proving the right attachment options
// Not only syntactically but also semantically, i.e we can only request auto-creation of projects if the integration allows it
func validateAttachment(ctx context.Context, rc *registrationConfig, ac *pb.AttachmentRequest, credentials *sdk.Credentials) error {
	if err := validateAttachmentConfiguration(rc, ac); err != nil {
		return fmt.Errorf("validating attachment configuration: %w", err)
	}

	// Instantiate an actual uploader to see if it would work with the current configuration
	d, err := uploader.NewSBOMUploader(rc.Domain, credentials.Password, nil, ac.GetProjectId(), ac.GetProjectName())
	if err != nil {
		return fmt.Errorf("creating uploader: %w", err)
	}

	if err := d.Validate(ctx); err != nil {
		return fmt.Errorf("validating uploader: %w", err)
	}

	return nil
}

func validateAttachmentConfiguration(rc *registrationConfig, ac *pb.AttachmentRequest) error {
	if rc == nil || ac == nil {
		return errors.New("invalid configuration")
	}

	if ac.GetProjectName() != "" && !rc.AllowAutoCreate {
		return errors.New("auto creation of projects is not supported in this integration")
	}

	if ac.GetProjectId() == "" && ac.GetProjectName() == "" {
		return errors.New("project id or name must be provided")
	}

	return nil
}

func validateExecuteOpts(opts *sdk.ExecutionRequest) error {
	if opts == nil || opts.Input == nil || opts.Input.Material == nil || opts.Input.Material.Content == nil {
		return errors.New("invalid input")
	}

	if opts.Input.Material.Type != schemaapi.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String() {
		return fmt.Errorf("invalid input type: %s", opts.Input.Material.Type)
	}

	if opts.RegistrationInfo == nil || opts.RegistrationInfo.Configuration == nil {
		return errors.New("missing registration configuration")
	}

	if opts.RegistrationInfo.Credentials == nil {
		return errors.New("missing credentials")
	}

	if opts.AttachmentInfo == nil || opts.AttachmentInfo.Configuration == nil {
		return errors.New("missing attachment configuration")
	}

	return nil
}
