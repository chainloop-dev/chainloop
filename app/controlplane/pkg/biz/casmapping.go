//
// Copyright 2023-2025 The Chainloop Authors.
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
	"fmt"
	"slices"
	"time"

	"github.com/chainloop-dev/chainloop/app/controlplane/pkg/authz"
	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
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
	Public    bool
	ProjectID uuid.UUID
}

type CASMappingFindOptions struct {
	Orgs       []uuid.UUID
	ProjectIDs []uuid.UUID
}

type CASMappingRepo interface {
	// Create a mapping with an optional workflow run id
	Create(ctx context.Context, digest string, casBackendID uuid.UUID, opts *CASMappingCreateOpts) (*CASMapping, error)
	// List all the CAS mappings for the given digest
	FindByDigest(ctx context.Context, digest string) ([]*CASMapping, error)
}

type CASMappingUseCase struct {
	repo           CASMappingRepo
	membershipRepo MembershipRepo
	projectsRepo   ProjectsRepo
	logger         *log.Helper
}

func NewCASMappingUseCase(repo CASMappingRepo, mRepo MembershipRepo, pRepo ProjectsRepo, logger log.Logger) *CASMappingUseCase {
	return &CASMappingUseCase{repo, mRepo, pRepo, servicelogger.ScopedHelper(logger, "cas-mapping-usecase")}
}

type CASMappingCreateOpts struct {
	WorkflowRunID *uuid.UUID
	ProjectID     *uuid.UUID
}

// Create a mapping with an optional workflow run id
func (uc *CASMappingUseCase) Create(ctx context.Context, digest string, casBackendID string, opts *CASMappingCreateOpts) (*CASMapping, error) {
	casBackendUUID, err := uuid.Parse(casBackendID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// parse the digest to make sure is a valid sha256 sum
	if _, err = cr_v1.NewHash(digest); err != nil {
		return nil, NewErrValidation(fmt.Errorf("invalid digest format: %w", err))
	}

	return uc.repo.Create(ctx, digest, casBackendUUID, opts)
}

func (uc *CASMappingUseCase) FindByDigest(ctx context.Context, digest string) ([]*CASMapping, error) {
	return uc.repo.FindByDigest(ctx, digest)
}

// FindCASMappingForDownloadByUser returns the CASMapping appropriate for the given digest and user.
// This means, in order:
// 1 - Any mapping that points to an organization which the user is member of.
// 1.1 If there are multiple mappings, it will pick the default one or the first one.
// 2 - Any mapping that is public.
func (uc *CASMappingUseCase) FindCASMappingForDownloadByUser(ctx context.Context, digest string, userID string) (*CASMapping, error) {
	uc.logger.Infow("msg", "finding cas mapping for download", "digest", digest, "user", userID)

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	// Load ALL memberships for the given user
	memberships, err := uc.membershipRepo.ListAllByUser(ctx, userUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships: %w", err)
	}

	userOrgs := make([]uuid.UUID, 0)
	// for every org with RBAC active, the list of allowed projects
	projectIDs := make(map[uuid.UUID][]uuid.UUID)
	for _, m := range memberships {
		if m.ResourceType == authz.ResourceTypeOrganization {
			userOrgs = append(userOrgs, m.ResourceID)
			// If the role in the org is member, we must enable RBAC for projects.
			if m.Role == authz.RoleOrgMember {
				// get list of projects in org, and match it with the memberships to build a filter
				orgProjects, err := getProjectsWithMembership(ctx, uc.projectsRepo, m.ResourceID, memberships)
				if err != nil {
					return nil, err
				}
				// note that appending an empty slice to a nil slice doesn't change it (it's still nil)
				projectIDs[m.ResourceID] = orgProjects
			}
		}
	}

	mapping, err := uc.FindCASMappingForDownloadByOrg(ctx, digest, userOrgs, projectIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to find cas mapping for download: %w", err)
	}

	return mapping, nil
}

// FindCASMappingForDownloadByOrg looks for the CAS mapping to download the referenced artifact in one of the passed organizations.
// The result will get filtered out if RBAC is enabled (projectIDs is not Nil)
func (uc *CASMappingUseCase) FindCASMappingForDownloadByOrg(ctx context.Context, digest string, orgs []uuid.UUID, projectIDs map[uuid.UUID][]uuid.UUID) (result *CASMapping, err error) {
	if _, err := cr_v1.NewHash(digest); err != nil {
		return nil, NewErrValidation(fmt.Errorf("invalid digest format: %w", err))
	}

	// log the result
	defer func() {
		if result != nil {
			uc.logger.Infow("msg", "mapping found!", "digest", digest, "orgs", orgs, "casBackend", result.CASBackend.ID, "default", result.CASBackend.Default, "public", result.Public)
		} else if err == nil || IsNotFound(err) {
			uc.logger.Infow("msg", "no mapping found!", "digest", digest, "orgs", orgs)
		}
	}()

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

	// 2 - CAS mappings associated with the given list of orgs and project IDs
	orgMappings, err := filterByOrgs(mappings, orgs, projectIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load mappings associated to an user: %w", err)
	} else if len(orgMappings) > 0 {
		return defaultOrFirst(orgMappings), nil
	}

	// 3 - mappings that are public
	publicMappings := filterByPublic(mappings)
	// The user has not access to neither proprietary nor public mappings
	if len(publicMappings) == 0 {
		uc.logger.Warnw("msg", "digest exist but user does not have access to it", "digest", digest, "orgs", orgs)
		return nil, NewErrNotFound("digest not found in any mapping")
	}

	// Pick the appropriate mapping from multiple ones
	return defaultOrFirst(publicMappings), nil
}

// Extract only the mappings associated with a list of orgs and optionally a list of projects
func filterByOrgs(mappings []*CASMapping, orgs []uuid.UUID, projectIDs map[uuid.UUID][]uuid.UUID) ([]*CASMapping, error) {
	result := make([]*CASMapping, 0)

	for _, mapping := range mappings {
		for _, o := range orgs {
			if mapping.OrgID == o {
				if visibleProjects, ok := projectIDs[mapping.ProjectID]; ok {
					if slices.Contains(visibleProjects, mapping.ProjectID) {
						result = append(result, mapping)
					}
				} else {
					result = append(result, mapping)
				}
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

// LookupDigestsInAttestation returns a list of references to the materials that have been uploaded to CAS
// as well as the attestation digest itself
func (uc *CASMappingUseCase) LookupDigestsInAttestation(att *dsse.Envelope, digest cr_v1.Hash) ([]*CASMappingLookupRef, error) {
	// Extract the materials that have been uploaded too
	predicate, err := chainloop.ExtractPredicate(att)
	if err != nil {
		return nil, fmt.Errorf("extracting predicate: %w", err)
	}

	references := []*CASMappingLookupRef{
		{
			Name:   "attestation",
			Digest: digest.String(),
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
