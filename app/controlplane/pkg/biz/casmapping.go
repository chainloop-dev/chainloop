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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/chainloop-dev/chainloop/internal/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

type CASMapping struct {
	ID, OrgID, WorkflowRunID uuid.UUID
	CASBackend               *CASBackend
	Digest                   string
	CreatedAt                *time.Time
	// A public mapping means that the material/attestation can be downloaded by anyone
	Public bool
}

type CASMappingRepo interface {
	Create(ctx context.Context, digest string, casBackendID, workflowRunID uuid.UUID) (*CASMapping, error)
	// List all the CAS mappings for the given digest
	FindByDigest(ctx context.Context, digest string) ([]*CASMapping, error)
}

type CASMappingUseCase struct {
	repo           CASMappingRepo
	membershipRepo MembershipRepo
	logger         *log.Helper
}

func NewCASMappingUseCase(repo CASMappingRepo, mRepo MembershipRepo, logger log.Logger) *CASMappingUseCase {
	return &CASMappingUseCase{repo, mRepo, servicelogger.ScopedHelper(logger, "cas-mapping-usecase")}
}

func (uc *CASMappingUseCase) Create(ctx context.Context, digest string, casBackendID, workflowRunID string) (*CASMapping, error) {
	casBackendUUID, err := uuid.Parse(casBackendID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	workflowRunUUID, err := uuid.Parse(workflowRunID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// parse the digest to make sure is a valid sha256 sum
	if _, err = cr_v1.NewHash(digest); err != nil {
		return nil, NewErrValidation(fmt.Errorf("invalid digest format: %w", err))
	}

	return uc.repo.Create(ctx, digest, casBackendUUID, workflowRunUUID)
}

func (uc *CASMappingUseCase) FindByDigest(ctx context.Context, digest string) ([]*CASMapping, error) {
	return uc.repo.FindByDigest(ctx, digest)
}

// FindCASMappingForDownloadByUser returns the CASMapping appropriate for the given digest and user
// This means, in order
// 1 - Any mapping that points to an organization which the user is member of
// 1.1 If there are multiple mappings, it will pick the default one or the first one
// 2 - Any mapping that is public
func (uc *CASMappingUseCase) FindCASMappingForDownloadByUser(ctx context.Context, digest string, userID string) (*CASMapping, error) {
	uc.logger.Infow("msg", "finding cas mapping for download", "digest", digest, "user", userID)

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Load organizations for the given user
	memberships, err := uc.membershipRepo.FindByUser(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships: %w", err)
	}

	userOrgs := make([]string, 0, len(memberships))
	for _, m := range memberships {
		userOrgs = append(userOrgs, m.OrganizationID.String())
	}

	return uc.FindCASMappingForDownloadByOrg(ctx, digest, userOrgs)
}

func (uc *CASMappingUseCase) FindCASMappingForDownloadByOrg(ctx context.Context, digest string, orgs []string) (*CASMapping, error) {
	if _, err := cr_v1.NewHash(digest); err != nil {
		return nil, NewErrValidation(fmt.Errorf("invalid digest format: %w", err))
	}

	if len(orgs) == 0 {
		return nil, NewErrValidationStr("no organizations provided")
	}

	// 1 - All CAS mappings for the given digest
	mappings, err := uc.repo.FindByDigest(ctx, digest)
	if err != nil {
		return nil, fmt.Errorf("failed to list cas mappings: %w", err)
	}

	uc.logger.Debugw("msg", fmt.Sprintf("found %d entries globally", len(mappings)), "digest", digest, "orgs", orgs)
	if len(mappings) == 0 {
		return nil, NewErrNotFound("digest not found in any mapping")
	}

	// 2 - CAS mappings associated with the given list of orgs
	orgMappings, err := filterByOrgs(mappings, orgs)
	if err != nil {
		return nil, fmt.Errorf("failed to load mappings associated to an user: %w", err)
	} else if len(orgMappings) > 0 {
		result := defaultOrFirst(orgMappings)

		uc.logger.Infow("msg", "mapping found!", "digest", digest, "orgs", orgs, "casBackend", result.CASBackend.ID, "default", result.CASBackend.Default, "public", result.Public)
		return result, nil
	}

	// 3 - mappings that are public
	publicMappings := filterByPublic(mappings)
	// The user has not access to neither proprietary nor public mappings
	if len(publicMappings) == 0 {
		uc.logger.Warnw("msg", "digest exist but user does not have access to it", "digest", digest, "orgs", orgs)
		return nil, NewErrUnauthorized(errors.New("unauthorized access to the artifact"))
	}

	// Pick the appropriate mapping from multiple ones
	result := defaultOrFirst(publicMappings)
	uc.logger.Infow("msg", "mapping found!", "digest", digest, "orgs", orgs, "casBackend", result.CASBackend.ID, "default", result.CASBackend.Default, "public", result.Public)
	return result, nil
}

// Extract only the mappings associated with a list of orgs
func filterByOrgs(mappings []*CASMapping, orgs []string) ([]*CASMapping, error) {
	result := make([]*CASMapping, 0)

	for _, mapping := range mappings {
		for _, o := range orgs {
			if mapping.OrgID.String() == o {
				result = append(result, mapping)
			}
		}
	}

	return result, nil
}

func filterByPublic(mappings []*CASMapping) []*CASMapping {
	result := make([]*CASMapping, 0)

	for _, mapping := range mappings {
		if mapping.Public {
			result = append(result, mapping)
		}
	}

	return result
}

func defaultOrFirst(mappings []*CASMapping) *CASMapping {
	if len(mappings) == 0 {
		return nil
	}

	result := mappings[0]
	for _, mapping := range mappings {
		if mapping.CASBackend.Default {
			result = mapping
			break
		}
	}

	return result
}

type CASMappingLookupRef struct {
	Name, Digest string
}

// LookupCASItemsInAttestation returns a list of references to the materials that have been uploaded to CAS
// as well as the attestation digest itself
func (uc *CASMappingUseCase) LookupDigestsInAttestation(att *dsse.Envelope) ([]*CASMappingLookupRef, error) {
	// Calculate the attestation hash
	jsonAtt, err := json.Marshal(att)
	if err != nil {
		return nil, fmt.Errorf("marshaling attestation: %w", err)
	}

	// Extract the materials that have been uploaded too
	predicate, err := chainloop.ExtractPredicate(att)
	if err != nil {
		return nil, fmt.Errorf("extracting predicate: %w", err)
	}

	// Calculate the attestation hash
	h, _, err := cr_v1.SHA256(bytes.NewBuffer(jsonAtt))
	if err != nil {
		return nil, fmt.Errorf("calculating attestation hash: %w", err)
	}

	references := []*CASMappingLookupRef{
		{
			Name:   "attestation",
			Digest: h.String(),
		},
	}

	for _, material := range predicate.GetMaterials() {
		if material.UploadedToCAS {
			references = append(references, &CASMappingLookupRef{
				Name:   material.Name,
				Digest: material.Hash.String(),
			})
		}
	}

	return references, nil
}
