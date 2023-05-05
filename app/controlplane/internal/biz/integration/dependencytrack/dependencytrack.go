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
	v1 "github.com/chainloop-dev/chainloop/app/controlplane/api/controlplane/v1"
	contractAPI "github.com/chainloop-dev/chainloop/app/controlplane/api/workflowcontract/v1"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/biz"
	"github.com/chainloop-dev/chainloop/app/controlplane/internal/integrations/dependencytrack"
	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/internal/credentials"
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
	errors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type Integration struct {
	integrationUC       *biz.IntegrationUseCase
	ociUC               *biz.OCIRepositoryUseCase
	credentialsProvider credentials.ReaderWriter
	casClient           biz.CASClient
	log                 *log.Helper
}

const Kind = "Dependency-Track"

func New(integrationUC *biz.IntegrationUseCase, ociUC *biz.OCIRepositoryUseCase, creds credentials.ReaderWriter, c biz.CASClient, l log.Logger) *Integration {
	return &Integration{integrationUC, ociUC, creds, c, servicelogger.ScopedHelper(l, "biz/integration/deptrack")}
}

func (uc *Integration) Add(ctx context.Context, orgID, host, apiKey string, enableProjectCreation bool) (*biz.Integration, error) {
	// Validate Credentials before saving them
	creds := &credentials.APICreds{Host: host, Key: apiKey}
	if err := creds.Validate(); err != nil {
		return nil, biz.NewErrValidation(err)
	}

	// Create the secret in the external secrets manager
	secretID, err := uc.credentialsProvider.SaveCredentials(ctx, orgID, creds)
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
	return uc.integrationUC.Create(ctx, orgID, Kind, secretID, c)
}

// Upload the SBOMs wrapped in the DSSE envelope to the configured Dependency Track instance
func (uc *Integration) UploadSBOMs(envelope *dsse.Envelope, orgID, workflowID string) error {
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
	predicates, err := chainloop.ExtractPredicate(envelope)
	if err != nil {
		return err
	}

	predicate := predicates.V01
	if predicate == nil {
		return errors.Forbidden("not implemented", "only v0.1 predicate is supported for now")
	}

	repo, err := uc.ociUC.FindMainRepo(ctx, orgID)
	if err != nil {
		return err
	} else if repo == nil {
		return errors.NotFound("not found", "main repository not found")
	}

	for _, m := range predicate.Materials {
		if m.Type != contractAPI.CraftingSchema_Material_SBOM_CYCLONEDX_JSON.String() {
			continue
		}

		buf := bytes.NewBuffer(nil)
		digest, ok := m.Material.SLSA.Digest["sha256"]
		if !ok {
			continue
		}

		digest = "sha256:" + digest

		uc.log.Infow("msg", "SBOM present, downloading", "workflowID", workflowID, "integration", Kind, "name", m.Name)
		// Download SBOM
		if err := uc.casClient.Download(ctx, repo.SecretName, buf, digest); err != nil {
			return fmt.Errorf("downloading from CAS: %w", err)
		}

		uc.log.Infow("msg", "SBOM downloaded", "digest", digest, "workflowID", workflowID, "integration", Kind, "name", m.Name)

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
	integrationConfig := i.Integration.Config.GetDependencyTrack()
	attachmentConfig := i.IntegrationAttachment.Config.GetDependencyTrack()

	creds := &credentials.APICreds{}
	if err := credsReader.ReadCredentials(ctx, i.SecretName, creds); err != nil {
		return err
	}

	log.Infow("msg", "Sending SBOM to Dependency-Track",
		"host", integrationConfig.Domain,
		"projectID", attachmentConfig.GetProjectId(), "projectName", attachmentConfig.GetProjectName(),
		"workflowID", workflowID, "integration", Kind,
	)

	d, err := dependencytrack.NewSBOMUploader(integrationConfig.Domain, creds.Key, sbom, attachmentConfig.GetProjectId(), attachmentConfig.GetProjectName())
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
