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

package dispatcher

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"

	schemaapi "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/plugins/sdk/v1"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type FanOutDispatcher struct {
	integrationUC       *biz.IntegrationUseCase
	wfUC                *biz.WorkflowUseCase
	credentialsProvider credentials.ReaderWriter
	casClient           biz.CASClient
	log                 *log.Helper
	l                   log.Logger
	loaded              sdk.AvailablePlugins
}

func New(integrationUC *biz.IntegrationUseCase, wfUC *biz.WorkflowUseCase, creds credentials.ReaderWriter, c biz.CASClient, registered sdk.AvailablePlugins, l log.Logger) *FanOutDispatcher {
	return &FanOutDispatcher{integrationUC, wfUC, creds, c, servicelogger.ScopedHelper(l, "fanout-dispatcher"), l, registered}
}

type integrationInfo struct {
	registrationConfig, attachmentConfig []byte
	credentials                          *sdk.Credentials
	workflowID                           string
	backend                              sdk.FanOut
}

// List of integrations that expect a material as input grouped by material type
// CDX_SBOM => [DEPTRACK INSTANCE 1, OCI INSTANCE 1, DEPTRACK INSTANCE 2]
type materialsDispatch map[schemaapi.CraftingSchema_Material_MaterialType][]*integrationInfo

// List of integrations that expect an attestation as input
type attestationDispatch []*integrationInfo

type dispatchQueue struct {
	// map of integrations that are subscribed to a material type event
	materials materialsDispatch
	// List of integrations that are subscribed to an attestation event
	attestations attestationDispatch
}

// Calculate the list of integrations that need to be called for this workflow
// This is done by looking at the list of attachments for this workflow and
// extracting the list of integrations that are subscribed to the materials
// and attestation that are part of the workflow
// The result is a fully populated dispatchQueue that contains the backend instance, and the configuration that will be required
// to be run during dispatch.Run
func (d *FanOutDispatcher) calculateDispatchQueue(ctx context.Context, orgID, workflowID string) (*dispatchQueue, error) {
	d.log.Infow("msg", "looking for attached integration", "workflowID", workflowID)

	// List enabled integrations with this workflow
	attachments, err := d.integrationUC.ListAttachments(ctx, orgID, workflowID)
	if err != nil {
		return nil, fmt.Errorf("listing attachments: %w", err)
	}

	materialDispatch := make(materialsDispatch)
	attestationDispatch := make(attestationDispatch, 0)
	for _, attachment := range attachments {
		// Get the integration DB object
		dbIntegration, err := d.integrationUC.FindByIDInOrg(ctx, orgID, attachment.IntegrationID.String())
		if err != nil {
			return nil, fmt.Errorf("finding integration in DB: %w", err)
		} else if dbIntegration == nil {
			d.log.Warnw("msg", "integration not found", "workflowID", workflowID, "ID", attachment.IntegrationID.String())
			continue
		}

		// Find the integration backend from the list of registered integrations
		backend, err := d.loaded.FindByID(dbIntegration.Kind)
		if err != nil {
			d.log.Warnw("msg", "integration backend not registered, skipped", "Kind", attachment.IntegrationID.String(), "err", err.Error())
			continue
		}

		d.log.Infow("msg", "found attached integration", "workflowID", workflowID, "integration", backend.String())

		// Craft required configuration
		// Retrieve credentials
		// TODO: remove from here since it's possible that this integration in fact is not being used in the end
		// so we'll be retrieving credentials for nothing
		creds := &sdk.Credentials{}
		if dbIntegration.SecretName != "" {
			if err := d.credentialsProvider.ReadCredentials(ctx, dbIntegration.SecretName, creds); err != nil {
				return nil, fmt.Errorf("reading credentials: %w", err)
			}
		}

		// All the required configuration needed to run the integration
		executionConfig := &integrationInfo{
			registrationConfig: dbIntegration.Config,
			attachmentConfig:   attachment.Config,
			credentials:        creds,
			workflowID:         workflowID,
			backend:            backend,
		}

		// Extract the list of materials this kind of backend is subscribed to
		inputs := backend.Describe().SubscribedInputs
		if inputs == nil {
			d.log.Warnw("msg", "integration does not subscribe to any material", "workflowID", workflowID, "backendID", backend.Describe().ID)
			continue
		}

		// If the integration is subscribed to the envelope, add it to the list of integrations that need to be called
		if inputs.DSSEnvelope {
			attestationDispatch = append(attestationDispatch, executionConfig)
		}

		// if the integration is subscribed to any material, add it to the list of integrations that need to be called
		if inputs.Materials != nil {
			for _, material := range inputs.Materials {
				// Add the integration to the list of integrations that need to be called for this material type
				if _, ok := materialDispatch[material.Type]; !ok {
					materialDispatch[material.Type] = []*integrationInfo{executionConfig}
				} else {
					materialDispatch[material.Type] = append(materialDispatch[material.Type], executionConfig)
				}
			}
		}
	}

	return &dispatchQueue{materials: materialDispatch, attestations: attestationDispatch}, nil
}

type RunOpts struct {
	Envelope           *dsse.Envelope
	OrgID              string
	WorkflowID         string
	WorkflowRunID      string
	DownloadSecretName string
}

// Run attestation and materials to the attached integrations
func (d *FanOutDispatcher) Run(ctx context.Context, opts *RunOpts) error {
	// Calculate metadata
	wf, err := d.wfUC.FindByID(ctx, opts.WorkflowID)
	if err != nil {
		return fmt.Errorf("finding workflow: %w", err)
	} else if wf == nil {
		return fmt.Errorf("workflow not found")
	}

	workflowMetadata := &sdk.ChainloopMetadata{
		WorkflowID:      opts.WorkflowID,
		WorkflowRunID:   opts.WorkflowRunID,
		WorkflowName:    wf.Name,
		WorkflowProject: wf.Project,
	}

	queue, err := d.calculateDispatchQueue(ctx, opts.OrgID, opts.WorkflowID)
	if err != nil {
		return fmt.Errorf("calculating dispatch queue: %w", err)
	}

	// get the in_toto statement from the envelope if present
	statement, err := chainloop.ExtractStatement(opts.Envelope)
	if err != nil {
		return fmt.Errorf("extracting statement: %w", err)
	}

	// Iterate over the materials in the attestation and dispatch them to the integrations that are subscribed to them
	predicate, err := chainloop.ExtractPredicate(opts.Envelope)
	if err != nil {
		return fmt.Errorf("extracting predicate: %w", err)
	}

	var attestationInput = &sdk.ExecuteAttestation{
		Envelope:  opts.Envelope,
		Statement: statement,
		Predicate: predicate,
	}

	// Send the envelope to the integrations that are subscribed to it
	for _, integration := range queue.attestations {
		req := generateRequest(integration, workflowMetadata)
		req.Input = &sdk.ExecuteInput{
			Attestation: attestationInput,
		}

		go func(backend sdk.FanOut) {
			_ = dispatch(ctx, backend, req, d.log)
		}(integration.backend)
	}

	for _, material := range predicate.GetMaterials() {
		// Find the backends that are subscribed to this material type, this includes
		// 1) Any integration backend that is listening to all material types
		// 2) Any integration backend that is listening to this specific material type
		var backends []*integrationInfo
		if b := queue.materials[schemaapi.CraftingSchema_Material_MATERIAL_TYPE_UNSPECIFIED]; b != nil {
			backends = append(backends, b...)
		}

		if b := queue.materials[schemaapi.CraftingSchema_Material_MaterialType(schemaapi.CraftingSchema_Material_MaterialType_value[material.Type])]; b != nil {
			backends = append(backends, b...)
		}

		// There are no backends that are subscribed to this material type
		if len(backends) == 0 {
			continue
		}

		d.log.Infow("msg", fmt.Sprintf("%d integrations found for this material type", len(backends)), "workflowID", opts.WorkflowID, "materialType", material.Type, "name", material.Name)

		// Retrieve material content
		content := []byte(material.Value)
		// It's a downloadable so we retrieve and override the content variable
		if material.Hash != nil && material.UploadedToCAS {
			digest := material.Hash.String()
			d.log.Infow("msg", "downloading material", "workflowID", opts.WorkflowID, "materialType", material.Type, "name", material.Name)
			buf := bytes.NewBuffer(nil)
			if err := d.casClient.Download(ctx, opts.DownloadSecretName, buf, digest); err != nil {
				return fmt.Errorf("downloading from CAS: %w", err)
			}

			content = buf.Bytes()
		}

		// Material information to be sent to the integration
		var materialInput = &sdk.ExecuteMaterial{
			NormalizedMaterial: material,
			Content:            content,
		}

		// Execute the integration backends
		for _, b := range backends {
			req := generateRequest(b, workflowMetadata)
			req.Input = &sdk.ExecuteInput{
				// They receive both the attestation information and the specific material information
				Material:    materialInput,
				Attestation: attestationInput,
			}

			go func() {
				_ = dispatch(ctx, b.backend, req, d.log)
			}()

			d.log.Infow("msg", "integration executed!", "workflowID", opts.WorkflowID, "materialType", material.Type, "integration", b.backend.Describe().ID)
		}
	}

	return nil
}

func dispatch(ctx context.Context, backend sdk.FanOut, opts *sdk.ExecutionRequest, logger *log.Helper) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 1 * time.Minute

	var inputType string
	switch {
	case opts.Input.Attestation != nil:
		inputType = "DSSEnvelope"
	case opts.Input.Material != nil:
		inputType = fmt.Sprintf("Material:%s", opts.Input.Material.Type)
	default:
		return errors.New("no input provided")
	}

	return backoff.RetryNotify(
		func() error {
			logger.Infow("msg", "executing integration", "integration", backend.String(), "input", inputType)
			err := backend.Execute(ctx, opts)
			if err == nil {
				logger.Infow("msg", "finished OK!", "integration", backend.String(), "input", inputType)
			}

			return err
		},
		b,
		func(err error, delay time.Duration) {
			logger.Warnf("error executing integration %s, will retry in %s - %s", backend.String(), delay, err)
		},
	)
}

func generateRequest(in *integrationInfo, metadata *sdk.ChainloopMetadata) *sdk.ExecutionRequest {
	return &sdk.ExecutionRequest{
		ChainloopMetadata: metadata,
		RegistrationInfo: &sdk.RegistrationResponse{
			Credentials:   in.credentials,
			Configuration: in.registrationConfig,
		},
		AttachmentInfo: &sdk.AttachmentResponse{
			Configuration: in.attachmentConfig,
		},
	}
}
