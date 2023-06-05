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

	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/anypb"
)

type IntegrationAttachment struct {
	ID                        uuid.UUID
	CreatedAt                 *time.Time
	Config                    *anypb.Any
	WorkflowID, IntegrationID uuid.UUID
}

type Integration struct {
	ID        uuid.UUID
	Kind      string
	CreatedAt *time.Time
	Config    *anypb.Any
	// Identifier to the external provider where any secret information is stored
	SecretName string
}

type IntegrationAndAttachment struct {
	*Integration
	*IntegrationAttachment
}

type IntegrationRepo interface {
	Create(ctx context.Context, orgID uuid.UUID, kind string, secretID string, config *anypb.Any) (*Integration, error)
	List(ctx context.Context, orgID uuid.UUID) ([]*Integration, error)
	FindByIDInOrg(ctx context.Context, orgID, ID uuid.UUID) (*Integration, error)
	SoftDelete(ctx context.Context, ID uuid.UUID) error
}

type IntegrationAttachmentRepo interface {
	Create(ctx context.Context, integrationID, workflowID uuid.UUID, config *anypb.Any) (*IntegrationAttachment, error)
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

type NewIntegrationUseCaseOpts struct {
	IRepo   IntegrationRepo
	IaRepo  IntegrationAttachmentRepo
	WfRepo  WorkflowRepo
	CredsRW credentials.ReaderWriter
	Logger  log.Logger
}

func NewIntegrationUseCase(opts *NewIntegrationUseCaseOpts) *IntegrationUseCase {
	if opts.Logger == nil {
		opts.Logger = log.NewStdLogger(io.Discard)
	}

	return &IntegrationUseCase{opts.IRepo, opts.IaRepo, opts.WfRepo, opts.CredsRW, servicelogger.ScopedHelper(opts.Logger, "biz/integration")}
}

// Persist the secret and integration with its configuration in the database
func (uc *IntegrationUseCase) RegisterAndSave(ctx context.Context, orgID string, i sdk.FanOut, regConfig *anypb.Any) (*Integration, error) {
	orgUUID, err := uuid.Parse(orgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	preRegistration, err := i.PreRegister(ctx, regConfig)
	if err != nil {
		return nil, NewErrValidation(err)
	}

	var secretID string
	if preRegistration.Credentials != nil {
		// Create the secret in the external secrets manager
		secretID, err = uc.credsRW.SaveCredentials(ctx, orgID, preRegistration.Credentials)
		if err != nil {
			return nil, fmt.Errorf("saving credentials: %w", err)
		}
	}

	// Wrap the configuration in an anypb.Any to store it in the DB
	config, err := anypb.New(preRegistration.Configuration)
	if err != nil {
		return nil, fmt.Errorf("creating configuration: %w", err)
	}

	// Persist the integration configuration
	return uc.integrationRepo.Create(ctx, orgUUID, i.Describe().ID, secretID, config)
}

type AttachOpts struct {
	IntegrationID, WorkflowID, OrgID string
	// The integration that is being attached
	FanOutIntegration sdk.FanOut
	// The attachment configuration
	AttachmentConfig *anypb.Any
}

// - Integration and workflows exists in current organization
// - Run specific validation for the integration
// - Persist integration attachment
func (uc *IntegrationUseCase) AttachToWorkflow(ctx context.Context, opts *AttachOpts) (*IntegrationAttachment, error) {
	if opts.FanOutIntegration == nil {
		return nil, NewErrValidation(errors.New("integration not provided"))
	}

	orgUUID, err := uuid.Parse(opts.OrgID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	workflowUUID, err := uuid.Parse(opts.WorkflowID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Get workflow in the scope of this organization
	wf, err := uc.workflowRepo.GetOrgScoped(ctx, orgUUID, workflowUUID)
	if err != nil {
		return nil, err
	} else if wf == nil {
		return nil, NewErrNotFound("workflow")
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

	// Retrieve credentials from the external secrets manager
	creds := &sdk.Credentials{}
	if integration.SecretName != "" {
		if err := uc.credsRW.ReadCredentials(ctx, integration.SecretName, creds); err != nil {
			return nil, fmt.Errorf("reading credentials: %w", err)
		}
	}

	// Execute integration pre-attachment logic
	preAttachResp, err := opts.FanOutIntegration.PreAttach(ctx, &sdk.BundledConfig{Registration: integration.Config, Attachment: opts.AttachmentConfig, Credentials: creds})
	if err != nil {
		return nil, NewErrValidation(err)
	}

	// Wrap the attachment configuration in an anypb.Any to store it in the DB
	config, err := anypb.New(preAttachResp.Configuration)
	if err != nil {
		return nil, fmt.Errorf("creating configuration: %w", err)
	}

	// Persist the attachment
	attachment, err := uc.integrationARepo.Create(ctx, integrationUUID, workflowUUID, config)
	if err != nil {
		return nil, fmt.Errorf("persisting attachment: %w", err)
	}

	return attachment, nil
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
		if err := uc.credsRW.DeleteCredentials(ctx, integration.SecretName); err != nil {
			return fmt.Errorf("deleting the credentials: %w", err)
		}
	}

	uc.logger.Infow("msg", "integration deleted", "ID", integrationID)
	// Check that the workflow to delete belongs to the provided organization
	return uc.integrationRepo.SoftDelete(ctx, integrationUUID)
}

// List attachments returns the list of attachments for a given organization and optionally workflow
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

		// We check that the workflow belongs to the provided organization
		// This check is mostly informative to the user
		wf, err := uc.workflowRepo.GetOrgScoped(ctx, orgUUID, workflowUUID)
		if err != nil {
			return nil, err
		} else if wf == nil {
			return nil, NewErrNotFound("workflow")
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
