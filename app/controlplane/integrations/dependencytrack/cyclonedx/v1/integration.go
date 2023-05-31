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
	"context"
	"errors"
	"fmt"

	core "github.com/chainloop-dev/chainloop/app/controlplane/integrations"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/dependencytrack/cyclonedx/v1/uploader"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/integrations/gen/dependencytrack/cyclonedx/v1"
	"google.golang.org/protobuf/types/known/anypb"
)

const Kind = "Dependency-Track"

var _ core.Registrable = (*Integration)(nil)
var _ core.Attachable = (*Integration)(nil)

type Integration struct {
	ID string
}

// Attach attaches the integration service to the given grpc server.
// In the future this will be a plugin entrypoint
func NewIntegration() (*Integration, error) {
	return &Integration{
		ID: Kind,
	}, nil
}

func (i *Integration) PreRegister(ctx context.Context, registrationRequest *anypb.Any) (*core.PreRegistration, error) {
	// Extract the request and un-marshal it to a concrete type
	req := new(pb.RegistrationRequest)
	if err := registrationRequest.UnmarshalTo(req); err != nil {
		return nil, fmt.Errorf("invalid request type: %w", err)
	}

	// Validate the request payload
	if err := req.ValidateAll(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Validate that the provided configuration is valid
	domain, enableProjectCreation := req.GetConfig().GetDomain(), req.GetConfig().GetAllowAutoCreate()
	checker, err := uploader.NewIntegration(domain, req.ApiKey, enableProjectCreation)
	if err != nil {
		return nil, fmt.Errorf("checking integration: %w", err)
	}

	// Validate that the provided configuration is valid against the remote service
	if err := checker.Validate(ctx); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Return what configuration to store in the database and what to store in the external secrets manager
	return &core.PreRegistration{
		Credentials:   &core.Credentials{Password: req.GetApiKey()},
		Configuration: req.Config,
		Kind:          Kind,
	}, nil
}

// Check configuration and return what configuration attachment to persist
func (i *Integration) PreAttach(ctx context.Context, b *core.BundledConfig) (*core.PreAttachment, error) {
	// Extract registration configuration
	rc := new(pb.RegistrationConfig)
	if err := b.Registration.UnmarshalTo(rc); err != nil {
		return nil, fmt.Errorf("invalid registration configuration: %w", err)
	}

	ar := new(pb.AttachmentRequest)
	if err := b.Attachment.UnmarshalTo(ar); err != nil {
		return nil, fmt.Errorf("invalid registration configuration: %w", err)
	}

	// Validate dynamic configuration
	if err := validateAttachment(ctx, rc, ar.Config, b.Credentials); err != nil {
		return nil, fmt.Errorf("invalid attachment configuration: %w", err)
	}

	return &core.PreAttachment{Configuration: ar.Config}, nil
}

// i.e we want to attach to a dependency track integration and we are proving the right attachment options
// Not only syntactically but also semantically, i.e we can only request auto-creation of projects if the integration allows it
func validateAttachment(ctx context.Context, rc *pb.RegistrationConfig, ac *pb.AttachmentConfig, credentials *core.Credentials) error {
	if err := validateAttachmentConfiguration(rc, ac); err != nil {
		return fmt.Errorf("validating configuration: %w", err)
	}

	// Instantiate an actual uploader to see if it would work with the current configuration
	d, err := uploader.NewSBOMUploader(rc.GetDomain(), credentials.Password, nil, ac.GetProjectId(), ac.GetProjectName())
	if err != nil {
		return fmt.Errorf("creating uploader: %w", err)
	}

	if err := d.Validate(ctx); err != nil {
		return fmt.Errorf("validating uploader: %w", err)
	}

	return nil
}

func validateAttachmentConfiguration(ic *pb.RegistrationConfig, ac *pb.AttachmentConfig) error {
	if ic == nil || ac == nil {
		return errors.New("invalid configuration")
	}

	if err := ic.ValidateAll(); err != nil {
		return fmt.Errorf("invalid integration configuration: %w", err)
	}

	if err := ac.ValidateAll(); err != nil {
		return fmt.Errorf("invalid integration configuration: %w", err)
	}

	if ac.GetProjectName() != "" && !ic.AllowAutoCreate {
		return errors.New("auto creation of projects is not supported in this integration")
	}

	return nil
}
