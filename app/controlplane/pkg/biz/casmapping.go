//
// Copyright 2023-2026 The Chainloop Authors.
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
	"time"

	"github.com/chainloop-dev/chainloop/pkg/attestation/renderer/chainloop"
	"github.com/chainloop-dev/chainloop/pkg/otelx"
	"github.com/chainloop-dev/chainloop/pkg/servicelogger"
	"github.com/go-kratos/kratos/v2/log"
	cr_v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/uuid"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var casMappingTracer = otelx.Tracer("chainloop-controlplane", "biz/casmapping")

type CASMapping struct {
	ID, OrgID, WorkflowRunID uuid.UUID
	CASBackend               *CASBackend
	Digest                   string
	CreatedAt                *time.Time
	// Scope of the artifact within the organization. At most one of them is set, both being unset
	// means the mapping is only reachable with organization-wide access.
	//
	// The two scopes are not symmetric: projects live in this database and ProjectID is backed by a
	// foreign key, while products live in the Chainloop platform database, so ProductID is a plain
	// UUID reference. The download filter still treats both as equal grants, see RBACScope.
	ProjectID uuid.UUID
	ProductID uuid.UUID
}

type CASMappingRepo interface {
	// Create a mapping with an optional workflow run id
	Create(ctx context.Context, digest string, casBackendID uuid.UUID, opts *CASMappingCreateOpts) (*CASMapping, error)
	// FindByDigestInOrgs returns a single accessible mapping for the digest within the given orgs
	// (honouring the RBAC scopes), preferring the default backend. Returns (nil, nil) when none exists.
	FindByDigestInOrgs(ctx context.Context, digest string, orgs []uuid.UUID, scopes RBACScopes) (*CASMapping, error)
	// ListByDigestInOrg returns the mappings for the digest in the given org, with no RBAC
	// filtering applied. Mappings whose CAS backend has been (soft) deleted are left out, since
	// they can no longer serve the artifact, so an empty slice means either no mapping exists or
	// none of them can still be served.
	//
	// It has no consumer in this repository: the Chainloop platform reconciles product-scoped
	// mappings for evidence uploaded before this scope existed, and needs to know which scopes and
	// backends an artifact already has. Kept here because it must ship in the same release the
	// platform bumps to for the product scope itself.
	ListByDigestInOrg(ctx context.Context, digest string, orgID uuid.UUID) ([]*CASMapping, error)
}

type CASMappingUseCase struct {
	repo         CASMappingRepo
	membershipUC *MembershipUseCase
	logger       *log.Helper
}

func NewCASMappingUseCase(repo CASMappingRepo, membershipUC *MembershipUseCase, logger log.Logger) *CASMappingUseCase {
	return &CASMappingUseCase{repo, membershipUC, servicelogger.ScopedHelper(logger, "cas-mapping-usecase")}
}

type CASMappingCreateOpts struct {
	WorkflowRunID *uuid.UUID
	// Scope of the artifact, see the CASMapping fields of the same name.
	ProjectID *uuid.UUID
	ProductID *uuid.UUID
}

// Create a mapping with an optional workflow run id
func (uc *CASMappingUseCase) Create(ctx context.Context, digest string, casBackendID string, opts *CASMappingCreateOpts) (*CASMapping, error) {
	ctx, span := otelx.Start(ctx, casMappingTracer, "CASMappingUseCase.Create")
	defer span.End()

	casBackendUUID, err := uuid.Parse(casBackendID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	if err := validateDigest(digest); err != nil {
		return nil, err
	}

	// The download filter grants access on either scope, so a mapping carrying both would be
	// reachable by members of the project AND members of the unrelated product.
	if opts != nil && opts.ProjectID != nil && opts.ProductID != nil {
		return nil, NewErrValidationStr("a mapping cannot be scoped to a project and a product at once")
	}

	return uc.repo.Create(ctx, digest, casBackendUUID, opts)
}

// validateDigest makes sure the digest is a valid sha256 sum
func validateDigest(digest string) error {
	if _, err := cr_v1.NewHash(digest); err != nil {
		return NewErrValidation(fmt.Errorf("invalid digest format: %w", err))
	}

	return nil
}

// FindCASMappingForDownloadByUser returns the CASMapping appropriate for the given digest and user.
// It returns a mapping that points to an organization the user is a member of (honoring the user's
// RBAC scopes); if there are multiple, it picks the default one or the first one.
func (uc *CASMappingUseCase) FindCASMappingForDownloadByUser(ctx context.Context, digest string, userID string) (*CASMapping, error) {
	ctx, span := otelx.Start(ctx, casMappingTracer, "CASMappingUseCase.FindCASMappingForDownloadByUser")
	defer span.End()

	uc.logger.Infow("msg", "finding cas mapping for download", "digest", digest, "user", userID)

	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, NewErrInvalidUUID(err)
	}

	userOrgs, scopes, err := uc.membershipUC.GetOrgsAndRBACInfoForUser(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	mapping, err := uc.FindCASMappingForDownloadByOrg(ctx, digest, userOrgs, scopes)
	if err != nil {
		return nil, fmt.Errorf("failed to find cas mapping for download: %w", err)
	}

	return mapping, nil
}

// FindCASMappingForDownloadByOrg looks for the CAS mapping to download the referenced artifact in one of the passed organizations.
// The result will get filtered out for those organizations RBAC is enabled for, i.e. those present in scopes.
func (uc *CASMappingUseCase) FindCASMappingForDownloadByOrg(ctx context.Context, digest string, orgs []uuid.UUID, scopes RBACScopes) (result *CASMapping, err error) {
	ctx, span := otelx.Start(ctx, casMappingTracer, "CASMappingUseCase.FindCASMappingForDownloadByOrg")
	defer span.End()

	if err := validateDigest(digest); err != nil {
		return nil, err
	}

	// log the result
	defer func() {
		if result != nil {
			uc.logger.Infow("msg", "mapping found!", "digest", digest, "orgs", orgs, "casBackend", result.CASBackend.ID, "default", result.CASBackend.Default)
		} else if err == nil || IsNotFound(err) {
			uc.logger.Infow("msg", "no mapping found!", "digest", digest, "orgs", orgs)
		}
	}()

	if len(orgs) == 0 {
		return nil, NewErrValidationStr("no organizations provided")
	}

	// A mapping reachable through one of the user's orgs (honouring the RBAC scopes), selected and
	// bounded in the database. This is the common path and stays cheap regardless of how many
	// mappings a digest has accumulated.
	mapping, err := uc.repo.FindByDigestInOrgs(ctx, digest, orgs, scopes)
	if err != nil {
		return nil, fmt.Errorf("failed to find cas mapping in orgs: %w", err)
	} else if mapping == nil {
		uc.logger.Warnw("msg", "digest not accessible to the requesting orgs", "digest", digest, "orgs", orgs, "scopes", scopes)
		return nil, NewErrNotFound("digest not found in any mapping")
	}

	return mapping, nil
}

// ListByDigestInOrg returns the servable mappings for the digest within the given organization,
// with no RBAC filtering. It is meant for trusted, system-level callers that need to inspect the
// existing scopes of an artifact, or the backends holding it, rather than to serve a download.
// Mappings on a (soft) deleted backend are left out, see the repository method.
func (uc *CASMappingUseCase) ListByDigestInOrg(ctx context.Context, digest string, orgID uuid.UUID) ([]*CASMapping, error) {
	ctx, span := otelx.Start(ctx, casMappingTracer, "CASMappingUseCase.ListByDigestInOrg")
	defer span.End()

	if err := validateDigest(digest); err != nil {
		return nil, err
	}

	if orgID == uuid.Nil {
		return nil, NewErrValidationStr("organization ID cannot be empty")
	}

	mappings, err := uc.repo.ListByDigestInOrg(ctx, digest, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list cas mappings in org: %w", err)
	}

	return mappings, nil
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

	// Include the policy evaluations bundle if stored in CAS
	if ref := predicate.GetPolicyEvaluationsRef(); ref != nil {
		if d, ok := ref.Digest["sha256"]; ok {
			references = append(references, &CASMappingLookupRef{
				Name:   ref.Name,
				Digest: fmt.Sprintf("sha256:%s", d),
			})
		}
	}

	return references, nil
}
