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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	crv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/cenkalti/backoff/v4"

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

// Dispatch item is a plugin instance + resolved inputs that gets hydrated
// during the dispatch process with information from both the DB and the attestation
type dispatchItem struct {
	// Configuration
	registrationConfig, attachmentConfig []byte
	credentials                          *sdk.Credentials
	// Actual plugin instance
	plugin sdk.FanOut

	// Fully resolved inputs
	materials   []*sdk.ExecuteMaterial
	attestation *sdk.ExecuteAttestation
}

type dispatchQueue []*dispatchItem

type RunOpts struct {
	Envelope           *dsse.Envelope
	OrgID              string
	WorkflowID         string
	WorkflowRunID      string
	DownloadSecretName string
}

func (d *FanOutDispatcher) Run(ctx context.Context, opts *RunOpts) error {
	// Hydration process for the dispatch queue
	// 1. Load all the integrations that are attached to this workflow
	queue, err := d.initDispatchQueue(ctx, opts.OrgID, opts.WorkflowID)
	if err != nil {
		return fmt.Errorf("loading integration info: %w", err)
	}

	d.log.Infow("msg", fmt.Sprintf("found %d attached integrations", len(queue)), "workflowID", opts.WorkflowID)

	if len(queue) == 0 {
		return nil
	}

	// 2. Hydrate the dispatch queue with the actual inputs
	if err := d.loadInputs(ctx, queue, opts.Envelope, opts.DownloadSecretName); err != nil {
		return fmt.Errorf("loading materials: %w", err)
	}

	// 3 - Calculate workflow / run information
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

	// Dispatch the integrations
	for _, item := range queue {
		req := generateRequest(item, workflowMetadata)
		go func(p sdk.FanOut, r *sdk.ExecutionRequest) {
			_ = dispatch(ctx, p, req, d.log)
		}(item.plugin, req)
	}

	return nil
}

// Initialize the dispatchQueue with information about all the attached integrations
func (d *FanOutDispatcher) initDispatchQueue(ctx context.Context, orgID, workflowID string) (dispatchQueue, error) {
	d.log.Infow("msg", "looking for attached integration", "workflowID", workflowID)

	queue := dispatchQueue{}

	// List enabled integrations with this workflow
	attachments, err := d.integrationUC.ListAttachments(ctx, orgID, workflowID)
	if err != nil {
		return nil, fmt.Errorf("listing attachments: %w", err)
	}

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
		creds := &sdk.Credentials{}
		if dbIntegration.SecretName != "" {
			if err := d.credentialsProvider.ReadCredentials(ctx, dbIntegration.SecretName, creds); err != nil {
				return nil, fmt.Errorf("reading credentials: %w", err)
			}
		}

		// All the required configuration needed to run the integration
		queue = append(queue, &dispatchItem{
			registrationConfig: dbIntegration.Config,
			attachmentConfig:   attachment.Config,
			credentials:        creds,
			plugin:             backend,
		})
	}

	return queue, nil
}

// Load the inputs for the dispatchItem, both materials and attestation
func (d *FanOutDispatcher) loadInputs(ctx context.Context, queue dispatchQueue, att *dsse.Envelope, secretName string) error {
	if att == nil {
		return fmt.Errorf("attestation is nil")
	}

	// Calculate the attestation information from the envelope
	statement, err := chainloop.ExtractStatement(att)
	if err != nil {
		return fmt.Errorf("extracting statement: %w", err)
	}

	predicate, err := chainloop.ExtractPredicate(att)
	if err != nil {
		return fmt.Errorf("extracting predicate: %w", err)
	}

	// Calculate the attestation hash
	jsonAtt, err := json.Marshal(att)
	if err != nil {
		return fmt.Errorf("marshaling attestation: %w", err)
	}

	// Using this library to calculate the hash because it allows us transport the digest
	// both the hash and the algorithm in a structured way
	// Also, by using it we are consistent with the way we pass the hash associated to the materials to plugins downstream
	h, _, err := crv1.SHA256(bytes.NewBuffer(jsonAtt))
	if err != nil {
		return fmt.Errorf("calculating attestation hash: %w", err)
	}

	var attestationInput = &sdk.ExecuteAttestation{
		Envelope:  att,
		Hash:      h,
		Statement: statement,
		Predicate: predicate,
	}

	// 1 - Attach the attestation to all the dispatchItems
	for _, item := range queue {
		item.attestation = attestationInput
	}

	// 2 - Attach the materials to only the plugins that is subscribed to that material type
	for _, material := range predicate.GetMaterials() {
		// By default is the inline material content
		content := []byte(material.Value)
		// Flag to make sure we download it only once
		var downloaded bool

		// Find the plugins that are subscribed to this material type
		for _, item := range queue {
			if item.plugin.IsSubscribedTo(material.Type) {
				// It's a downloadable and has not been downloaded yet
				if !downloaded && material.Hash != nil && material.UploadedToCAS {
					buf := bytes.NewBuffer(nil)
					if err := d.casClient.Download(ctx, secretName, buf, material.Hash.String()); err != nil {
						return fmt.Errorf("downloading from CAS: %w", err)
					}

					content = buf.Bytes()
					downloaded = true
				}

				item.materials = append(item.materials, &sdk.ExecuteMaterial{
					NormalizedMaterial: material,
					Content:            content,
				})

				d.log.Infow("msg", "adding material to integration", "material", material.Type, "integration", item.plugin.String())
			}
		}
	}

	return nil
}

func dispatch(ctx context.Context, plugin sdk.FanOut, opts *sdk.ExecutionRequest, logger *log.Helper) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 10 * time.Second

	var inputType string
	switch {
	case opts.Input.Attestation != nil:
		inputType = "DSSEnvelope"
	case len(opts.Input.Materials) > 0:
		var materialTypes []string
		for _, m := range opts.Input.Materials {
			materialTypes = append(materialTypes, m.Type)
		}

		inputType = fmt.Sprintf("Materials: %q", materialTypes)
	default:
		return errors.New("no input provided")
	}

	return backoff.RetryNotify(
		func() error {
			logger.Infow("msg", "executing integration", "integration", plugin.String(), "input", inputType)

			err := plugin.Execute(ctx, opts)
			if err == nil {
				logger.Infow("msg", "execution OK!", "integration", plugin.String(), "input", inputType)
			}

			return err
		},
		b,
		func(err error, delay time.Duration) {
			logger.Warnf("error executing integration %s, will retry in %s - %s", plugin.String(), delay, err)
		},
	)
}

func generateRequest(in *dispatchItem, metadata *sdk.ChainloopMetadata) *sdk.ExecutionRequest {
	return &sdk.ExecutionRequest{
		ChainloopMetadata: metadata,
		RegistrationInfo: &sdk.RegistrationResponse{
			Credentials:   in.credentials,
			Configuration: in.registrationConfig,
		},
		AttachmentInfo: &sdk.AttachmentResponse{
			Configuration: in.attachmentConfig,
		},
		Input: &sdk.ExecuteInput{
			Materials:   in.materials,
			Attestation: in.attestation,
		},
	}
}
