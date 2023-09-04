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
	"github.com/chainloop-dev/chainloop/internal/servicelogger"
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

// FindCASMappingForDownload returns the CASMapping appropriate for the given digest and user
// This means any mapping that points to an organization which the user is member of
// If there are multiple mappings, it will try to pick the one that points to a default backend
// Otherwise the first one
func (uc *CASMappingUseCase) FindCASMappingForDownload(ctx context.Context, digest string, userID string) (*CASMapping, error) {
	uc.logger.Infow("msg", "finding cas mapping for download", "digest", digest, "user", userID)

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	if _, err = cr_v1.NewHash(digest); err != nil {
		return nil, NewErrValidation(fmt.Errorf("invalid digest format: %w", err))
	}

	// list all the CAS allMappings for the given digest
	allMappings, err := uc.repo.FindByDigest(ctx, digest)
	if err != nil {
		return nil, fmt.Errorf("failed to list cas mappings: %w", err)
	}

	uc.logger.Debugw("msg", fmt.Sprintf("found %d entries globally", len(allMappings)), "digest", digest, "user", userID)
	// The given digest has not been uploaded to any CAS backend
	if len(allMappings) == 0 {
		return nil, NewErrNotFound("digest not found in any mapping")
	}

	// filter the ones that the user has access to.
	// This means any mapping that points to an organization which the user is member of
	userMappings := make([]*CASMapping, 0)
	memberships, err := uc.membershipRepo.FindByUser(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships: %w", err)
	}

	for _, mapping := range allMappings {
		for _, m := range memberships {
			if mapping.OrgID == m.OrganizationID {
				userMappings = append(userMappings, mapping)
			}
		}
	}

	uc.logger.Debugw("msg", fmt.Sprintf("found %d entries for the user", len(userMappings)), "digest", digest, "user", userID)

	// The user has not access to
	if len(userMappings) == 0 {
		uc.logger.Warnw("msg", "digest exist but user does not have access to it", "digest", digest, "user", userID)
		return nil, NewErrUnauthorized(errors.New("unauthorized access to the artifact"))
	}

	// Pick the appropriate mapping from multiple ones
	// for now it will work as follows
	// 1 - If there is only one mapping, return it
	// 2 - if there are more than 1, we try to pick the one that points to a default backend
	// 3 - Otherwise the first one
	result := userMappings[0]
	for _, mapping := range userMappings {
		if mapping.CASBackend.Default {
			result = mapping
			break
		}
	}

	uc.logger.Infow("msg", "mapping found!", "digest", digest, "user", userID, "casBackend", result.CASBackend.ID, "default", result.CASBackend.Default)
	return result, nil
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
