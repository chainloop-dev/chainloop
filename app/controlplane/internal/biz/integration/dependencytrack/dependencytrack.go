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
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations"
	"github.com/chainloop-dev/chainloop/app/controlplane/integrations/dependencytrack/cyclonedx/v1/uploader"
	pb "github.com/chainloop-dev/chainloop/app/controlplane/integrations/gen/dependencytrack/cyclonedx/v1"
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
}

// TODO: remove
const Kind = "Dependency-Track"

func New(integrationUC *biz.IntegrationUseCase, creds credentials.ReaderWriter, c biz.CASClient, l log.Logger) *Integration {
	return &Integration{integrationUC, creds, c, servicelogger.ScopedHelper(l, "biz/integration/deptrack")}
}

// Upload the SBOMs wrapped in the DSSE envelope to the configured Dependency Track instance
func (uc *Integration) UploadSBOMs(envelope *dsse.Envelope, orgID, workflowID, secretName string) error {
	ctx := context.Background()
	uc.log.Infow("msg", "looking for integration", "workflowID", workflowID, "integration", Kind)

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
		if integration.Kind == Kind {
			depTrackIntegrations = append(depTrackIntegrations, &biz.IntegrationAndAttachment{Integration: integration, IntegrationAttachment: at})
		}
	}

	if len(depTrackIntegrations) == 0 {
		uc.log.Infow("msg", "no attached integrations", "workflowID", workflowID, "integration", Kind)
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
			uc.log.Warnw("msg", "CYCLONE_DX material but download digest missing, skipping", "workflowID", workflowID, "integration", Kind, "name", material.Name)
			continue
		}

		digest := material.Hash.String()

		uc.log.Infow("msg", "SBOM present, downloading", "workflowID", workflowID, "integration", Kind, "name", material.Name)
		// Download SBOM
		buf := bytes.NewBuffer(nil)
		if err := uc.casClient.Download(ctx, secretName, buf, digest); err != nil {
			return fmt.Errorf("downloading from CAS: %w", err)
		}

		uc.log.Infow("msg", "SBOM downloaded", "digest", digest, "workflowID", workflowID, "integration", Kind, "name", material.Name)

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
						return doSendToDependencyTrack(ctx, uc.credentialsProvider, workflowID, buf, i, uc.log)
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

func doSendToDependencyTrack(ctx context.Context, credsReader credentials.Reader, workflowID string, sbom io.Reader, i *biz.IntegrationAndAttachment, log *log.Helper) error {
	integrationConfig := new(pb.RegistrationConfig)
	if err := i.Integration.Config.UnmarshalTo(integrationConfig); err != nil {
		return fmt.Errorf("unmarshalling config: %w", err)
	}

	attachmentConfig := new(pb.AttachmentConfig)
	if err := i.IntegrationAttachment.Config.UnmarshalTo(attachmentConfig); err != nil {
		return fmt.Errorf("unmarshalling config: %w", err)
	}

	creds := &integrations.Credentials{}
	if err := credsReader.ReadCredentials(ctx, i.SecretName, creds); err != nil {
		return err
	}

	log.Infow("msg", "Sending SBOM to Dependency-Track",
		"host", integrationConfig.Domain,
		"projectID", attachmentConfig.GetProjectId(), "projectName", attachmentConfig.GetProjectName(),
		"workflowID", workflowID, "integration", Kind,
	)

	d, err := uploader.NewSBOMUploader(integrationConfig.Domain, creds.Password, sbom, attachmentConfig.GetProjectId(), attachmentConfig.GetProjectName())
	if err != nil {
		return err
	}

	if err := d.Validate(ctx); err != nil {
		return err
	}

	if err := d.Do(ctx); err != nil {
		return err
	}

	log.Infow("msg", "SBOM Sent to Dependency-Track",
		"host", integrationConfig.Domain,
		"projectID", attachmentConfig.GetProjectId(), "projectName", attachmentConfig.GetProjectName(),
		"workflowID", workflowID, "integration", Kind,
	)

	return nil
}
