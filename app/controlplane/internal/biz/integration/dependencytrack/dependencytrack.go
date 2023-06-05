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
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	dti "github.com/chainloop-dev/chainloop/app/controlplane/integrations/dependencytrack/cyclonedx/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/sdk/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// TODO: This module will be removed and added as part of the new integrations framework
type Integration struct {
	integrationUC       *biz.IntegrationUseCase
	credentialsProvider credentials.ReaderWriter
	casClient           biz.CASClient
	log                 *log.Helper
	l                   log.Logger
}

func New(integrationUC *biz.IntegrationUseCase, creds credentials.ReaderWriter, c biz.CASClient, l log.Logger) *Integration {
	return &Integration{integrationUC, creds, c, servicelogger.ScopedHelper(l, "biz/integration/deptrack"), l}
}

// Upload the SBOMs wrapped in the DSSE envelope to the configured Dependency Track instance
func (uc *Integration) UploadSBOMs(envelope *dsse.Envelope, orgID, workflowID, secretName string) error {
	// TODO: all this code will be replaced by a new generic dispatcher
	deptrackFanOut, err := dti.NewIntegration(uc.l)
	if err != nil {
		return fmt.Errorf("creating integration: %w", err)
	}

	kind := deptrackFanOut.Describe().ID

	ctx := context.Background()
	uc.log.Infow("msg", "looking for integration", "workflowID", workflowID, "integration", kind)

	// List enabled integrations with this workflow
	attachments, err := uc.integrationUC.ListAttachments(ctx, orgID, workflowID)
	if err != nil {
		return err
	}

	// Load the ones about dependency track
	var depTrackIntegrations []*biz.IntegrationAndAttachment
	for _, at := range attachments {
		integration, err := uc.integrationUC.FindByIDInOrg(ctx, orgID, at.IntegrationID.String())
		if err != nil {
			return err
		} else if integration == nil {
			continue
		}
		if integration.Kind == kind {
			depTrackIntegrations = append(depTrackIntegrations, &biz.IntegrationAndAttachment{Integration: integration, IntegrationAttachment: at})
		}
	}

	if len(depTrackIntegrations) == 0 {
		uc.log.Infow("msg", "no attached integrations", "workflowID", workflowID, "integration", kind)
		return nil
	}

	// There is at least one enabled integration, extract the SBOMs
	predicate, err := chainloop.ExtractPredicate(envelope)
	if err != nil {
		return err
	}

	for _, material := range predicate.GetMaterials() {
		if material.Type != contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String() {
			continue
		}

		if material.Hash == nil {
			uc.log.Warnw("msg", "CYCLONE_DX material but download digest missing, skipping", "workflowID", workflowID, "integration", kind, "name", material.Name)
			continue
		}

		digest := material.Hash.String()

		uc.log.Infow("msg", "SBOM present, downloading", "workflowID", workflowID, "integration", kind, "name", material.Name)
		// Download SBOM
		buf := bytes.NewBuffer(nil)
		if err := uc.casClient.Download(ctx, secretName, buf, digest); err != nil {
			return fmt.Errorf("downloading from CAS: %w", err)
		}

		uc.log.Infow("msg", "SBOM downloaded", "digest", digest, "workflowID", workflowID, "integration", kind, "name", material.Name)

		// Run integrations with that sbom
		var wg sync.WaitGroup
		var errs = make(chan error)
		var wgDone = make(chan bool)

		for _, i := range depTrackIntegrations {
			wg.Add(1)
			b := backoff.NewExponentialBackOff()
			b.MaxElapsedTime = 10 * time.Second

			go func(i *biz.IntegrationAndAttachment) {
				defer wg.Done()
				err := backoff.RetryNotify(
					func() error {
						creds := &sdk.Credentials{}
						if err := uc.credentialsProvider.ReadCredentials(ctx, i.SecretName, creds); err != nil {
							return err
						}

						materialContent, err := io.ReadAll(buf)
						if err != nil {
							return fmt.Errorf("reading material content: %w", err)
						}

						// Execute integration pre-attachment logic
						err = deptrackFanOut.Execute(ctx, &sdk.ExecuteReq{
							Config: &sdk.BundledConfig{
								Registration: i.Integration.Config, Attachment: i.IntegrationAttachment.Config, Credentials: creds,
								WorkflowID: workflowID,
							},
							Input: &sdk.ExecuteInput{
								Material: &sdk.ExecuteMaterial{NormalizedMaterial: material, Content: materialContent},
							},
						})
						if err != nil {
							return fmt.Errorf("executing integration: %w", err)
						}

						return nil
					},
					b,
					func(err error, delay time.Duration) {
						uc.log.Warnw("msg", "error uploading SBOM", "retry", delay, "error", err)
					},
				)
				if err != nil {
					errs <- err
					log.Error(err)
				}
			}(i)
		}

		go func() {
			wg.Wait()
			close(wgDone)
		}()

		select {
		case <-wgDone:
			break
		case err := <-errs:
			return err
		}
	}

	return nil
}
