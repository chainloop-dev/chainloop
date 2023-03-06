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

package biz

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	v1 "github.com/chainloop-dev/bedrock/app/controlplane/api/controlplane/v1"
	"github.com/chainloop-dev/bedrock/app/controlplane/internal/integrations/dependencytrack"
	"github.com/chainloop-dev/bedrock/internal/credentials"
	"github.com/chainloop-dev/bedrock/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

type IntegrationAttachment struct {
	ID                        uuid.UUID
	CreatedAt                 *time.Time
	Config                    *v1.IntegrationAttachmentConfig
	WorkflowID, IntegrationID uuid.UUID
}

type Integration struct {
	ID        uuid.UUID
	Kind      string
	CreatedAt *time.Time
	Config    *v1.IntegrationConfig
	// Identifier to the external provider where any secret information is stored
	SecretName string
}

type IntegrationAndAttachment struct {
	*Integration
	*IntegrationAttachment
}

type IntegrationRepo interface {
	Create(ctx context.Context, orgID uuid.UUID, kind string, secretID string, config *v1.IntegrationConfig) (*Integration, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*Integration, error)
	FindByIDInOrg(ctx context.Context, orgID, ID uuid.UUID) (*Integration, error)
	SoftDelete(ctx context.Context, ID uuid.UUID) error
}

type IntegrationAttachmentRepo interface {
	Create(ctx context.Context, integrationID, workflowID uuid.UUID, config *v1.IntegrationAttachmentConfig) (*IntegrationAttachment, error)
	List(ctx context.Context, orgID, workflowID uuid.UUID) ([]*IntegrationAttachment, error)
	FindByIDInOrg(ctx context.Context, orgID, ID uuid.UUID) (*IntegrationAttachment, error)
	SoftDelete(ctx context.Context, ID uuid.UUID) error
}

type IntegrationUseCase struct {
	integrationRepo  IntegrationRepo
	integrationARepo IntegrationAttachmentRepo
	workflowRepo     WorkflowRepo
	credsRW          credentials.ReaderWriter
	logger           *log.Helper
}

const DependencyTrackKind = "Dependency-Track"

type NewIntegrationUsecaseOpts struct {
	IRepo   IntegrationRepo
	IaRepo  IntegrationAttachmentRepo
	WfRepo  WorkflowRepo
	CredsRW credentials.ReaderWriter
	Logger  log.Logger
}

func NewIntegrationUsecase(opts *NewIntegrationUsecaseOpts) *IntegrationUseCase {
	if opts.Logger == nil {
		opts.Logger = log.NewStdLogger(io.Discard)
	}

	return &IntegrationUseCase{opts.IRepo, opts.IaRepo, opts.WfRepo, opts.CredsRW, servicelogger.ScopedHelper(opts.Logger, "biz/integration")}
}

func (uc *IntegrationUseCase) AddDependencyTrack(ctx context.Context, orgID, host, apiKey string, enableProjectCreation bool) (*Integration, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Create the secret in the external secrets manager
	secretID, err := uc.credsRW.SaveAPICreds(ctx, orgID, &credentials.APICreds{Host: host, Key: apiKey})
	if err != nil {
		return nil, fmt.Errorf("storing the credentials: %w", err)
	}

	c := &v1.IntegrationConfig{
		Config: &v1.IntegrationConfig_DependencyTrack_{
			DependencyTrack: &v1.IntegrationConfig_DependencyTrack{
				AllowAutoCreate: enableProjectCreation, Domain: host,
			},
		},
	}

	// Persist data
	return uc.integrationRepo.Create(ctx, orgUUID, DependencyTrackKind, secretID, c)
}

func (uc *IntegrationUseCase) List(ctx context.Context, orgID string) ([]*Integration, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.integrationRepo.List(ctx, orgUUID)
}

func (uc *IntegrationUseCase) FindByIDInOrg(ctx context.Context, orgID, id string) (*Integration, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	uuid, err := uuid.Parse(id)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	return uc.integrationRepo.FindByIDInOrg(ctx, orgUUID, uuid)
}

func (uc *IntegrationUseCase) Delete(ctx context.Context, orgID, integrationID string) error {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	integrationUUID, err := uuid.Parse(integrationID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	uc.logger.Infow("msg", "deleting integration", "ID", integrationID)
	// Make sure that the integration is from this org and it has not associated workflows
	integration, err := uc.integrationRepo.FindByIDInOrg(ctx, orgUUID, integrationUUID)
	if err != nil {
		return err
	} else if integration == nil {
		return NewErrNotFound("integration")
	}

	if integration.SecretName != "" {
		uc.logger.Infow("msg", "deleting integration external secrets", "ID", integrationID, "secretName", integration.SecretName)
		if err := uc.credsRW.DeleteCreds(ctx, integration.SecretName); err != nil {
			return fmt.Errorf("deleting the credentials: %w", err)
		}
	}

	uc.logger.Infow("msg", "integration deleted", "ID", integrationID)
	// Check that the workflow to delete belongs to the provided organization
	return uc.integrationRepo.SoftDelete(ctx, integrationUUID)
}

type AttachOpts struct {
	IntegrationID, WorkflowID, OrgID string
	Config                           *v1.IntegrationAttachmentConfig
}

// - Integration and workflows exists in current organization
// - Integration is compatible with the provided installation config, i.e can autocreate
func (uc *IntegrationUseCase) AttachToWorkflow(ctx context.Context, opts *AttachOpts) (*IntegrationAttachment, error) {
	orgUUID, err := uuid.Parse(opts.OrgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	integrationUUID, err := uuid.Parse(opts.IntegrationID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Make sure that the integration is from this org
	integration, err := uc.integrationRepo.FindByIDInOrg(ctx, orgUUID, integrationUUID)
	if err != nil {
		return nil, err
	} else if integration == nil {
		return nil, NewErrNotFound("integration")
	}

	// Check workflow is in this org
	workflowUUID, err := uuid.Parse(opts.WorkflowID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	wf, err := uc.workflowRepo.GetOrgScoped(ctx, orgUUID, workflowUUID)
	if err != nil {
		return nil, err
	} else if wf == nil {
		return nil, NewErrNotFound("workflow")
	}

	// Check that the provided attachConfiguration is compatible with the referred integration
	if err := validateAttachment(ctx, integration, uc.credsRW, integration.Config, opts.Config); err != nil {
		return nil, newErrValidation(err)
	}

	return uc.integrationARepo.Create(ctx, integrationUUID, workflowUUID, opts.Config)
}

func (uc *IntegrationUseCase) ListAttachments(ctx context.Context, orgID, workflowID string) ([]*IntegrationAttachment, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	workflowUUID := uuid.Nil
	if workflowID != "" {
		var err error
		workflowUUID, err = uuid.Parse(workflowID)
		if err != nil {
			return nil, NewErrInvalidUUID(err)
		}
	}

	return uc.integrationARepo.List(ctx, orgUUID, workflowUUID)
}

// Detach integration from workflow
func (uc *IntegrationUseCase) Detach(ctx context.Context, orgID, attachmentID string) error {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	attachmentUUID, err := uuid.Parse(attachmentID)
	if err != nil {
		return NewErrInvalidUUID(err)
	}

	// Make sure that the attachment is part of the provided organization
	if attachment, err := uc.integrationARepo.FindByIDInOrg(ctx, orgUUID, attachmentUUID); err != nil {
		return err
	} else if attachment == nil {
		return NewErrNotFound("attachment")
	}

	return uc.integrationARepo.SoftDelete(ctx, attachmentUUID)
}

// Validations
// i.e we want to attach to a dependency track integration and we are proving the right attachment options
// Not only syntactically but also semantically, i.e we can only request auto-creation of projects if the integration allows it
func validateAttachment(ctx context.Context, integration *Integration, credsR credentials.Reader, ic *v1.IntegrationConfig, ac *v1.IntegrationAttachmentConfig) error {
	if ic == nil || ac == nil {
		return errors.New("invalid configuration")
	}

	switch c := ic.Config.(type) {
	case *v1.IntegrationConfig_DependencyTrack_:
		// Check static configuration first
		if err := c.DependencyTrack.ValidateAttachment(ac.GetDependencyTrack()); err != nil {
			return err
		}

		// Check with the actual remote data that an upload would be possible
		creds := &credentials.APICreds{}
		if err := credsR.ReadAPICreds(ctx, integration.SecretName, creds); err != nil {
			return err
		}

		// Instantiate an actual uploader to see if it would work with the current configuration
		d, err := dependencytrack.NewSBOMUploader(c.DependencyTrack.GetDomain(), creds.Key,
			nil, ac.GetDependencyTrack().GetProjectId(), ac.GetDependencyTrack().GetProjectName())
		if err != nil {
			return err
		}

		if err := d.Validate(ctx); err != nil {
			return err
		}

		return nil
	default:
		return errors.New("unsupported integration")
	}
}
